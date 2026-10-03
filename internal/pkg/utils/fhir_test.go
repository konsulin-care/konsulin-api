package utils

import (
	"testing"
	"time"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/require"
)

func TestCalculateAgeCalendarBoundaries(t *testing.T) {
	for _, tc := range []struct {
		birth, today string
		want         int
	}{
		{"", "2021-03-01", 0},
		{"invalid", "2021-03-01", 0},
		{"2000-03-01", "2021-02-28", 20},
		{"2000-03-01", "2021-03-01", 21},
		{"2000-03-01", "2021-03-02", 21},
		{"2001-03-01", "2020-02-29", 18},
		{"2001-03-01", "2020-03-01", 19},
		{"2000-02-29", "2021-02-28", 20},
		{"2000-02-29", "2021-03-01", 21},
		{"2000-10-15", "2021-10-14", 20},
		{"2000-10-15", "2021-10-15", 21},
	} {
		today, err := time.Parse("2006-01-02", tc.today)
		require.NoError(t, err)
		require.Equal(t, tc.want, calculateAgeAt(tc.birth, today), tc.birth+" at "+tc.today)
	}
	require.Zero(t, CalculateAge(time.Now().Format("2006-01-02")))
}

func TestFHIRProfileDemographics(t *testing.T) {
	name := []fhir_dto.HumanName{{Prefix: []string{"Dr"}, Given: []string{"Ada"}, Family: "Example"}}
	telecom := []fhir_dto.ContactPoint{{System: "fax", Value: "ignore"}, {System: "email", Value: "ada@example.invalid"}, {System: "phone", Use: "work", Value: "ignore"}, {System: "phone", Use: "mobile", Value: "6281234567890"}}
	extensions := []fhir_dto.Extension{{Url: "other", ValueString: "ignore"}, {Url: constvars.FhirEducationExtensionURL, ValueString: "Degree"}}
	addresses := []fhir_dto.Address{{Use: "temp", Line: []string{"ignore"}}, {Use: "home", Line: []string{"Home", "Street"}}, {Use: "work", Line: []string{"Clinic", "Street"}}}
	patient := BuildPatientProfileResponse(&fhir_dto.Patient{Name: name, Telecom: telecom, Extension: extensions, Address: addresses, Gender: "female", BirthDate: "2000-01-02"})
	practitioner := BuildPractitionerProfileResponse(&fhir_dto.Practitioner{Name: name, Telecom: telecom, Extension: extensions, Address: addresses, Gender: "female", BirthDate: "2000-01-02"})
	require.Equal(t, "Dr Ada Example", patient.Fullname)
	require.Equal(t, "ada@example.invalid", patient.Email)
	require.Equal(t, "6281234567890", patient.WhatsAppNumber)
	require.Equal(t, []string{"Degree"}, patient.Educations)
	require.Equal(t, "02 January 2000", patient.BirthDate)
	require.Equal(t, "female", patient.Gender)
	require.Equal(t, "Home, Street", patient.Address)
	require.Equal(t, "Clinic, Street", practitioner.Address)
	practitioner.Address = patient.Address
	require.Equal(t, patient, practitioner)
	require.Empty(t, GetFullName(nil))
	require.Empty(t, GetFullName([]fhir_dto.HumanName{{}}))
	require.Empty(t, GetHomeAddress(nil))
	require.Empty(t, GetWorkAddress(nil))
	require.Empty(t, FormatBirthDate(""))
	require.Equal(t, "invalid", FormatBirthDate("invalid"))
	clinician := MapPractitionerToClinicClinician(&fhir_dto.Practitioner{ID: "p1", Name: name}, []fhir_dto.CodeableConcept{{Text: "Psychology"}}, "Clinic")
	require.Equal(t, "p1", clinician.PractitionerID)
	require.Equal(t, "Dr Ada Example", clinician.Name)
	require.Equal(t, []string{"Psychology"}, clinician.Specialties)
	require.Equal(t, "Clinic", clinician.ClinicName)
	require.Equal(t, "Clinic", clinician.Affiliation)
}

func TestFHIRCollections(t *testing.T) {
	roles := []fhir_dto.PractitionerRole{{Organization: fhir_dto.Reference{Reference: "Organization/o1"}}, {Organization: fhir_dto.Reference{Reference: "Patient/p1"}}, {Organization: fhir_dto.Reference{Reference: "bad"}}}
	require.Equal(t, []string{"o1"}, ExtractOrganizationIDsFromPractitionerRoles(roles))
	codes := []fhir_dto.CodeableConcept{{Coding: []fhir_dto.Coding{{Display: "A"}, {Display: "B"}}}, {}}
	require.Equal(t, []string{"A", "B"}, ExtractSpecialties(codes))
	require.Equal(t, []string{"A", "B"}, ExtractQualifications([]fhir_dto.Qualification{{Code: codes[0]}, {}}))
	for i, short := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
		day := time.Weekday((i + 1) % 7).String()
		require.True(t, DaysContains([]string{short}, day))
	}
	require.True(t, DaysContains([]string{"Monday"}, "Monday"))
	require.False(t, DaysContains([]string{"invalid"}, "Monday"))
	require.False(t, DaysContains(nil, "Monday"))
	require.True(t, Contains([]string{"a", "b"}, "b"))
	require.False(t, Contains([]string{"a"}, "b"))
	slice := []string{"a", "b", "a"}
	RemoveFromSlice(&slice, "a")
	require.Equal(t, []string{"b", "a"}, slice)
	RemoveFromSlice(&slice, "missing")
	require.Equal(t, []string{"b", "a"}, slice)
	require.Equal(t, []string{"09:00", "09:30", "10:00"}, GenerateTimeSlots("09:00:00", "10:30:00"))
	require.Empty(t, GenerateTimeSlots("10:00:00", "09:00:00"))
}

func TestJournalObservationRoundTrip(t *testing.T) {
	request := &requests.CreateJournal{PatientID: "p1", Title: "Journal", JournalBody: []string{"first", "second"}, JournalDate: "2026-01-02"}
	observation, err := MapJournalRequestToCreateObserVationRequest(request)
	require.NoError(t, err)
	require.Equal(t, "Observation", observation.ResourceType)
	require.Equal(t, "final", observation.Status)
	require.Equal(t, "Patient/p1", observation.Subject.Reference)
	require.Len(t, observation.Component, 3)
	observation.ID = "journal-1"
	journal, err := MapObservationToJournalResponse(observation)
	require.NoError(t, err)
	require.Equal(t, "journal-1", journal.JournalID)
	require.Equal(t, request.PatientID, journal.PatientID)
	require.Equal(t, request.Title, journal.Title)
	require.Equal(t, request.JournalBody, journal.JournalBody)
	require.Equal(t, request.JournalDate, journal.JournalDate.Format("2006-01-02"))
	id, err := GetPatientIDFromObservation(observation)
	require.NoError(t, err)
	require.Equal(t, "p1", id)
	updated, err := MapUpdateJournalToUpdateObservationRequest(&requests.UpdateJournal{JournalID: "journal-1", PatientID: "p1", Title: "Updated", JournalBody: []string{"new"}, JournalDate: "2026-01-03"})
	require.NoError(t, err)
	require.Equal(t, "journal-1", updated.ID)
	require.Equal(t, "amended", updated.Status)
	require.Equal(t, "Updated", updated.Component[0].ValueString)
	require.Equal(t, "new", updated.Component[1].ValueString)
	request.JournalDate = "invalid"
	_, err = MapJournalRequestToCreateObserVationRequest(request)
	require.Error(t, err)
	_, err = MapUpdateJournalToUpdateObservationRequest(&requests.UpdateJournal{JournalDate: "invalid"})
	require.Error(t, err)
	_, err = MapObservationToJournalResponse(&fhir_dto.Observation{})
	require.Error(t, err)
	for _, reference := range []string{"", "bad", "Practitioner/p1", "Patient/p1/extra"} {
		_, err = GetPatientIDFromObservation(&fhir_dto.Observation{Subject: fhir_dto.Reference{Reference: reference}})
		require.Error(t, err)
	}
	for _, reference := range []string{"", "bad", "Patient/p1/extra"} {
		journal, err = MapObservationToJournalResponse(&fhir_dto.Observation{Subject: fhir_dto.Reference{Reference: reference}, EffectiveDateTime: "2026-01-02T00:00:00Z", Component: []fhir_dto.Component{{Code: fhir_dto.CodeableConcept{Text: "unrelated"}}}})
		require.NoError(t, err)
		require.Empty(t, journal.PatientID)
	}
}
