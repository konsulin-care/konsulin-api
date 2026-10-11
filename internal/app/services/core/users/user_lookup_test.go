package users

import (
	"context"
	"errors"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLookupUserFHIRResourceIDs(t *testing.T) {
	practitionerErr := errors.New("practitioner unavailable")
	patientErr := errors.New("patient unavailable")
	for _, tc := range []struct {
		name                        string
		practitioners               []fhir_dto.Practitioner
		patients                    []fhir_dto.Patient
		practitionerErr, patientErr error
		want                        contracts.InitializeNewUserFHIRResourcesOutput
	}{
		{name: "no resources"},
		{name: "practitioner only", practitioners: []fhir_dto.Practitioner{{ID: "p1"}}, want: contracts.InitializeNewUserFHIRResourcesOutput{PractitionerID: "p1"}},
		{name: "patient only", patients: []fhir_dto.Patient{{ID: "u1"}}, want: contracts.InitializeNewUserFHIRResourcesOutput{PatientID: "u1"}},
		{name: "first matches for both", practitioners: []fhir_dto.Practitioner{{ID: "p1"}, {ID: "p2"}}, patients: []fhir_dto.Patient{{ID: "u1"}, {ID: "u2"}}, want: contracts.InitializeNewUserFHIRResourcesOutput{PractitionerID: "p1", PatientID: "u1"}},
		{name: "practitioner error discards partial patient", practitionerErr: practitionerErr, patients: []fhir_dto.Patient{{ID: "u1"}}},
		{name: "patient error discards partial practitioner", patientErr: patientErr, practitioners: []fhir_dto.Practitioner{{ID: "p1"}}},
		{name: "both errors preserved", practitionerErr: practitionerErr, patientErr: patientErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			practitioner := new(MockPractitionerFhirClient)
			patient := new(MockPatientFhirClient)
			practitioner.On("FindPractitionerByIdentifier", ctx, constvars.FhirSupertokenSystemIdentifier, "user-1").Return(tc.practitioners, tc.practitionerErr).Once()
			patient.On("FindPatientByIdentifier", ctx, constvars.FhirSupertokenSystemIdentifier+"|user-1").Return(tc.patients, tc.patientErr).Once()
			uc := &userUsecase{PractitionerFhirClient: practitioner, PatientFhirClient: patient, Log: zap.NewNop()}
			got, err := uc.LookupUserFHIRResourceIDs(ctx, &contracts.LookupUserFHIRResourceIDsInput{SuperTokenUserID: "user-1"})
			if tc.practitionerErr != nil || tc.patientErr != nil {
				require.Nil(t, got)
				if tc.practitionerErr != nil {
					require.ErrorIs(t, err, tc.practitionerErr)
				}
				if tc.patientErr != nil {
					require.ErrorIs(t, err, tc.patientErr)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, &tc.want, got)
			}
			practitioner.AssertExpectations(t)
			patient.AssertExpectations(t)
		})
	}
	t.Run("missing identifier performs no lookup", func(t *testing.T) {
		uc := &userUsecase{Log: zap.NewNop()}
		got, err := uc.LookupUserFHIRResourceIDs(context.Background(), &contracts.LookupUserFHIRResourceIDsInput{})
		require.Error(t, err)
		require.Nil(t, got)
		got, err = uc.LookupUserFHIRResourceIDs(context.Background(), nil)
		require.Error(t, err)
		require.Nil(t, got)
	})
}
