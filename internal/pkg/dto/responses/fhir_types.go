package responses

import (
	"time"
)

type Duration struct {
	Value  float64 `json:"value,omitempty" bson:"value,omitempty"`
	Unit   string  `json:"unit,omitempty" bson:"unit,omitempty"`
	System string  `json:"system,omitempty" bson:"system,omitempty"`
	Code   string  `json:"code,omitempty" bson:"code,omitempty"`
}

type Identifier struct {
	Use    string          `json:"use,omitempty" bson:"use,omitempty"`
	System string          `json:"system,omitempty" bson:"system,omitempty"`
	Value  string          `json:"value,omitempty" bson:"value,omitempty"`
	Period Period          `json:"period,omitempty" bson:"period,omitempty"`
	Type   CodeableConcept `json:"type,omitempty" bson:"type,omitempty"`
}

type HumanName struct {
	Use    string   `json:"use,omitempty" bson:"use,omitempty"`
	Text   string   `json:"text,omitempty" bson:"text,omitempty"`
	Family string   `json:"family,omitempty" bson:"family,omitempty"`
	Given  []string `json:"given,omitempty" bson:"given,omitempty"`
}

type Address struct {
	Use        string   `json:"use,omitempty" bson:"use,omitempty"`
	Type       string   `json:"type,omitempty" bson:"type,omitempty"`
	Text       string   `json:"text,omitempty" bson:"text,omitempty"`
	Line       []string `json:"line,omitempty" bson:"line,omitempty"`
	City       string   `json:"city,omitempty" bson:"city,omitempty"`
	District   string   `json:"district,omitempty" bson:"district,omitempty"`
	State      string   `json:"state,omitempty" bson:"state,omitempty"`
	PostalCode string   `json:"postalCode,omitempty" bson:"postalCode,omitempty"`
	Country    string   `json:"country,omitempty" bson:"country,omitempty"`
}

type ContactPoint struct {
	System string `json:"system,omitempty" bson:"system,omitempty"`
	Value  string `json:"value,omitempty" bson:"value,omitempty"`
	Use    string `json:"use,omitempty" bson:"use,omitempty"`
	Rank   int    `json:"rank,omitempty" bson:"rank,omitempty"`
}

type Coding struct {
	System  string `json:"system,omitempty" bson:"system,omitempty"`
	Version string `json:"version,omitempty" bson:"version,omitempty"`
	Code    string `json:"code,omitempty" bson:"code,omitempty"`
	Display string `json:"display,omitempty" bson:"display,omitempty"`
}

type CodeableConcept struct {
	Coding []Coding `json:"coding,omitempty" bson:"coding,omitempty"`
	Text   string   `json:"text,omitempty" bson:"text,omitempty"`
}

type Period struct {
	Start string `json:"start,omitempty" bson:"start,omitempty"`
	End   string `json:"end,omitempty" bson:"end,omitempty"`
}
type PeriodBackup struct {
	Start time.Time `json:"start,omitempty" bson:"start,omitempty"`
	End   time.Time `json:"end,omitempty" bson:"end,omitempty"`
}

type Quantity struct {
	Value  float64 `json:"value,omitempty" bson:"value,omitempty"`
	Unit   string  `json:"unit,omitempty" bson:"unit,omitempty"`
	System string  `json:"system,omitempty" bson:"system,omitempty"`
	Code   string  `json:"code,omitempty" bson:"code,omitempty"`
}

type Range struct {
	Low  Quantity `json:"low,omitempty" bson:"low,omitempty"`
	High Quantity `json:"high,omitempty" bson:"high,omitempty"`
}

type Ratio struct {
	Numerator   Quantity `json:"numerator,omitempty" bson:"numerator,omitempty"`
	Denominator Quantity `json:"denominator,omitempty" bson:"denominator,omitempty"`
}

type SampledData struct {
	Origin     Quantity `json:"origin" bson:"origin"`
	Period     float64  `json:"period" bson:"period"`
	Factor     float64  `json:"factor,omitempty" bson:"factor,omitempty"`
	LowerLimit float64  `json:"lowerLimit,omitempty" bson:"lowerLimit,omitempty"`
	UpperLimit float64  `json:"upperLimit,omitempty" bson:"upperLimit,omitempty"`
	Dimensions int      `json:"dimensions" bson:"dimensions"`
	Data       string   `json:"data" bson:"data"`
}

type Annotation struct {
	AuthorReference Reference `json:"authorReference,omitempty" bson:"authorReference,omitempty"`
	AuthorString    string    `json:"authorString,omitempty" bson:"authorString,omitempty"`
	Time            time.Time `json:"time,omitempty" bson:"time,omitempty"`
	Text            string    `json:"text" bson:"text"`
}

type Attachment struct {
	ContentType string `json:"contentType,omitempty" bson:"contentType,omitempty"`
	Language    string `json:"language,omitempty" bson:"language,omitempty"`
	Data        []byte `json:"data,omitempty" bson:"data,omitempty"`
	Url         string `json:"url,omitempty" bson:"url,omitempty"`
	Size        int    `json:"size,omitempty" bson:"size,omitempty"`
	Hash        []byte `json:"hash,omitempty" bson:"hash,omitempty"`
	Title       string `json:"title,omitempty" bson:"title,omitempty"`
}

type Reference struct {
	Reference  string     `json:"reference,omitempty" bson:"reference,omitempty"`
	Type       string     `json:"type,omitempty" bson:"type,omitempty"`
	Identifier Identifier `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Display    string     `json:"display,omitempty" bson:"display,omitempty"`
}

type Meta struct {
	VersionId   string    `json:"versionId,omitempty" bson:"versionId,omitempty"`
	LastUpdated time.Time `json:"lastUpdated,omitempty" bson:"lastUpdated,omitempty"`
	Source      string    `json:"source,omitempty" bson:"source,omitempty"`
	Profile     []string  `json:"profile,omitempty" bson:"profile,omitempty"`
	Security    []Coding  `json:"security,omitempty" bson:"security,omitempty"`
	Tag         []Coding  `json:"tag,omitempty" bson:"tag,omitempty"`
}

type Extension struct {
	Url            string    `json:"url" bson:"url"`
	ValueString    string    `json:"valueString,omitempty" bson:"valueString,omitempty"`
	ValueBoolean   bool      `json:"valueBoolean,omitempty" bson:"valueBoolean,omitempty"`
	ValueInteger   int       `json:"valueInteger,omitempty" bson:"valueInteger,omitempty"`
	ValueDecimal   float64   `json:"valueDecimal,omitempty" bson:"valueDecimal,omitempty"`
	ValueUri       string    `json:"valueUri,omitempty" bson:"valueUri,omitempty"`
	ValueCode      string    `json:"valueCode,omitempty" bson:"valueCode,omitempty"`
	ValueId        string    `json:"valueId,omitempty" bson:"valueId,omitempty"`
	ValueDate      string    `json:"valueDate,omitempty" bson:"valueDate,omitempty"`
	ValueDateTime  time.Time `json:"valueDateTime,omitempty" bson:"valueDateTime,omitempty"`
	ValueTime      string    `json:"valueTime,omitempty" bson:"valueTime,omitempty"`
	ValueCoding    Coding    `json:"valueCoding,omitempty" bson:"valueCoding,omitempty"`
	ValueQuantity  Quantity  `json:"valueQuantity,omitempty" bson:"valueQuantity,omitempty"`
	ValueReference Reference `json:"valueReference,omitempty" bson:"valueReference,omitempty"`
}

type Qualification struct {
	Identifier []Identifier    `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Code       CodeableConcept `json:"code" bson:"code"`
	Period     *Period         `json:"period,omitempty" bson:"period,omitempty"`
	Issuer     *Reference      `json:"issuer,omitempty" bson:"issuer,omitempty"`
}

type BackboneElement struct {
	ModifierExtension []Extension `json:"modifierExtension,omitempty" bson:"modifierExtension,omitempty"`
}

type MedicationIngredient struct {
	Item     *Reference `json:"item,omitempty" bson:"item,omitempty"`
	IsActive *bool      `json:"isActive,omitempty" bson:"isActive,omitempty"`
	Strength *Ratio     `json:"strength,omitempty" bson:"strength,omitempty"`
}

type MedicationBatch struct {
	LotNumber      string     `json:"lotNumber,omitempty" bson:"lotNumber,omitempty"`
	ExpirationDate *time.Time `json:"expirationDate,omitempty" bson:"expirationDate,omitempty"`
}

type ProcedurePerformer struct {
	Function   *CodeableConcept `json:"function,omitempty" bson:"function,omitempty"`
	Actor      Reference        `json:"actor" bson:"actor"`
	OnBehalfOf *Reference       `json:"onBehalfOf,omitempty" bson:"onBehalfOf,omitempty"`
}

type ProcedureFocalDevice struct {
	Action      *CodeableConcept `json:"action,omitempty" bson:"action,omitempty"`
	Manipulated Reference        `json:"manipulated" bson:"manipulated"`
}

type AllergyIntoleranceReaction struct {
	Substance     *CodeableConcept  `json:"substance,omitempty" bson:"substance,omitempty"`
	Manifestation []CodeableConcept `json:"manifestation" bson:"manifestation"`
	Description   string            `json:"description,omitempty" bson:"description,omitempty"`
	Onset         *time.Time        `json:"onset,omitempty" bson:"onset,omitempty"`
	Severity      string            `json:"severity,omitempty" bson:"severity,omitempty"`
	ExposureRoute *CodeableConcept  `json:"exposureRoute,omitempty" bson:"exposureRoute,omitempty"`
	Note          []Annotation      `json:"note,omitempty" bson:"note,omitempty"`
}
type CarePlanActivity struct {
	OutcomeCodeableConcept []CodeableConcept       `json:"outcomeCodeableConcept,omitempty" bson:"outcomeCodeableConcept,omitempty"`
	OutcomeReference       []Reference             `json:"outcomeReference,omitempty" bson:"outcomeReference,omitempty"`
	Progress               []Annotation            `json:"progress,omitempty" bson:"progress,omitempty"`
	Reference              *Reference              `json:"reference,omitempty" bson:"reference,omitempty"`
	Detail                 *CarePlanActivityDetail `json:"detail,omitempty" bson:"detail,omitempty"`
}

type CarePlanActivityDetail struct {
	Kind                   string            `json:"kind,omitempty" bson:"kind,omitempty"`
	InstantiatesCanonical  []string          `json:"instantiatesCanonical,omitempty" bson:"instantiatesCanonical,omitempty"`
	InstantiatesUri        []string          `json:"instantiatesUri,omitempty" bson:"instantiatesUri,omitempty"`
	Code                   *CodeableConcept  `json:"code,omitempty" bson:"code,omitempty"`
	ReasonCode             []CodeableConcept `json:"reasonCode,omitempty" bson:"reasonCode,omitempty"`
	ReasonReference        []Reference       `json:"reasonReference,omitempty" bson:"reasonReference,omitempty"`
	Goal                   []Reference       `json:"goal,omitempty" bson:"goal,omitempty"`
	Status                 string            `json:"status" bson:"status"`
	StatusReason           *CodeableConcept  `json:"statusReason,omitempty" bson:"statusReason,omitempty"`
	DoNotPerform           *bool             `json:"doNotPerform,omitempty" bson:"doNotPerform,omitempty"`
	ScheduledTiming        *Timing           `json:"scheduledTiming,omitempty" bson:"scheduledTiming,omitempty"`
	ScheduledPeriod        *Period           `json:"scheduledPeriod,omitempty" bson:"scheduledPeriod,omitempty"`
	ScheduledString        *string           `json:"scheduledString,omitempty" bson:"scheduledString,omitempty"`
	Location               *Reference        `json:"location,omitempty" bson:"location,omitempty"`
	Performer              []Reference       `json:"performer,omitempty" bson:"performer,omitempty"`
	ProductCodeableConcept *CodeableConcept  `json:"productCodeableConcept,omitempty" bson:"productCodeableConcept,omitempty"`
	ProductReference       *Reference        `json:"productReference,omitempty" bson:"productReference,omitempty"`
	DailyAmount            *Quantity         `json:"dailyAmount,omitempty" bson:"dailyAmount,omitempty"`
	Quantity               *Quantity         `json:"quantity,omitempty" bson:"quantity,omitempty"`
	Description            *string           `json:"description,omitempty" bson:"description,omitempty"`
}

type Timing struct {
	Event  []time.Time      `json:"event,omitempty" bson:"event,omitempty"`
	Repeat *TimingRepeat    `json:"repeat,omitempty" bson:"repeat,omitempty"`
	Code   *CodeableConcept `json:"code,omitempty" bson:"code,omitempty"`
}

// TimingRepeat type
type TimingRepeat struct {
	BoundsDuration *Duration   `json:"boundsDuration,omitempty" bson:"boundsDuration,omitempty"`
	BoundsRange    *Range      `json:"boundsRange,omitempty" bson:"boundsRange,omitempty"`
	BoundsPeriod   *Period     `json:"boundsPeriod,omitempty" bson:"boundsPeriod,omitempty"`
	Count          *int        `json:"count,omitempty" bson:"count,omitempty"`
	CountMax       *int        `json:"countMax,omitempty" bson:"countMax,omitempty"`
	Duration       *float64    `json:"duration,omitempty" bson:"duration,omitempty"`
	DurationMax    *float64    `json:"durationMax,omitempty" bson:"durationMax,omitempty"`
	DurationUnit   *string     `json:"durationUnit,omitempty" bson:"durationUnit,omitempty"`
	Frequency      *int        `json:"frequency,omitempty" bson:"frequency,omitempty"`
	FrequencyMax   *int        `json:"frequencyMax,omitempty" bson:"frequencyMax,omitempty"`
	Period         *float64    `json:"period,omitempty" bson:"period,omitempty"`
	PeriodMax      *float64    `json:"periodMax,omitempty" bson:"periodMax,omitempty"`
	PeriodUnit     *string     `json:"periodUnit,omitempty" bson:"periodUnit,omitempty"`
	DayOfWeek      []string    `json:"dayOfWeek,omitempty" bson:"dayOfWeek,omitempty"`
	TimeOfDay      []time.Time `json:"timeOfDay,omitempty" bson:"timeOfDay,omitempty"`
	When           []string    `json:"when,omitempty" bson:"when,omitempty"`
	Offset         *int        `json:"offset,omitempty" bson:"offset,omitempty"`
}

type DiagnosticReportMedia struct {
	Comment *string   `json:"comment,omitempty" bson:"comment,omitempty"`
	Link    Reference `json:"link" bson:"link"`
}

// EncounterStatusHistory type
type EncounterStatusHistory struct {
	Status string `json:"status" bson:"status"`
	Period Period `json:"period" bson:"period"`
}

// EncounterClassHistory type
type EncounterClassHistory struct {
	Class  Coding `json:"class" bson:"class"`
	Period Period `json:"period" bson:"period"`
}

// EncounterParticipant type
type EncounterParticipant struct {
	Type       []CodeableConcept `json:"type,omitempty" bson:"type,omitempty"`
	Period     *Period           `json:"period,omitempty" bson:"period,omitempty"`
	Individual *Reference        `json:"individual,omitempty" bson:"individual,omitempty"`
}

// EncounterDiagnosis type
type EncounterDiagnosis struct {
	Condition Reference        `json:"condition" bson:"condition"`
	Use       *CodeableConcept `json:"use,omitempty" bson:"use,omitempty"`
	Rank      *int             `json:"rank,omitempty" bson:"rank,omitempty"`
}

// EncounterHospitalization type
type EncounterHospitalization struct {
	PreAdmissionIdentifier *Identifier       `json:"preAdmissionIdentifier,omitempty" bson:"preAdmissionIdentifier,omitempty"`
	Origin                 *Reference        `json:"origin,omitempty" bson:"origin,omitempty"`
	AdmitSource            *CodeableConcept  `json:"admitSource,omitempty" bson:"admitSource,omitempty"`
	ReAdmission            *CodeableConcept  `json:"reAdmission,omitempty" bson:"reAdmission,omitempty"`
	DietPreference         []CodeableConcept `json:"dietPreference,omitempty" bson:"dietPreference,omitempty"`
	SpecialCourtesy        []CodeableConcept `json:"specialCourtesy,omitempty" bson:"specialCourtesy,omitempty"`
	SpecialArrangement     []CodeableConcept `json:"specialArrangement,omitempty" bson:"specialArrangement,omitempty"`
	Destination            *Reference        `json:"destination,omitempty" bson:"destination,omitempty"`
	DischargeDisposition   *CodeableConcept  `json:"dischargeDisposition,omitempty" bson:"dischargeDisposition,omitempty"`
}

// EncounterLocation type
type EncounterLocation struct {
	Location     Reference        `json:"location" bson:"location"`
	Status       *string          `json:"status,omitempty" bson:"status,omitempty"`
	PhysicalType *CodeableConcept `json:"physicalType,omitempty" bson:"physicalType,omitempty"`
	Period       *Period          `json:"period,omitempty" bson:"period,omitempty"`
}
