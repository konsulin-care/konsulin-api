package responses

type Encounter struct {
	ID              string                    `json:"id,omitempty" bson:"_id,omitempty"`
	Meta            Meta                      `json:"meta,omitempty" bson:"meta,omitempty"`
	Identifier      []Identifier              `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Status          string                    `json:"status" bson:"status"`
	StatusHistory   []EncounterStatusHistory  `json:"statusHistory,omitempty" bson:"statusHistory,omitempty"`
	Class           Coding                    `json:"class" bson:"class"`
	ClassHistory    []EncounterClassHistory   `json:"classHistory,omitempty" bson:"classHistory,omitempty"`
	Type            []CodeableConcept         `json:"type,omitempty" bson:"type,omitempty"`
	ServiceType     *CodeableConcept          `json:"serviceType,omitempty" bson:"serviceType,omitempty"`
	Priority        *CodeableConcept          `json:"priority,omitempty" bson:"priority,omitempty"`
	Subject         Reference                 `json:"subject,omitempty" bson:"subject,omitempty"`
	EpisodeOfCare   []Reference               `json:"episodeOfCare,omitempty" bson:"episodeOfCare,omitempty"`
	BasedOn         []Reference               `json:"basedOn,omitempty" bson:"basedOn,omitempty"`
	Participant     []EncounterParticipant    `json:"participant,omitempty" bson:"participant,omitempty"`
	Appointment     []Reference               `json:"appointment,omitempty" bson:"appointment,omitempty"`
	Period          *Period                   `json:"period,omitempty" bson:"period,omitempty"`
	Length          *Quantity                 `json:"length,omitempty" bson:"length,omitempty"`
	ReasonCode      []CodeableConcept         `json:"reasonCode,omitempty" bson:"reasonCode,omitempty"`
	ReasonReference []Reference               `json:"reasonReference,omitempty" bson:"reasonReference,omitempty"`
	Diagnosis       []EncounterDiagnosis      `json:"diagnosis,omitempty" bson:"diagnosis,omitempty"`
	Account         []Reference               `json:"account,omitempty" bson:"account,omitempty"`
	Hospitalization *EncounterHospitalization `json:"hospitalization,omitempty" bson:"hospitalization,omitempty"`
	Location        []EncounterLocation       `json:"location,omitempty" bson:"location,omitempty"`
	ServiceProvider *Reference                `json:"serviceProvider,omitempty" bson:"serviceProvider,omitempty"`
	PartOf          *Reference                `json:"partOf,omitempty" bson:"partOf,omitempty"`
}
