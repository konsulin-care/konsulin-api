package fhir_dto

import "time"

type Range struct {
	ID        string      `json:"id,omitempty"`
	Extension []Extension `json:"extension,omitempty"`
	Low       *Quantity   `json:"low,omitempty"`
	High      *Quantity   `json:"high,omitempty"`
}
type Duration struct {
	ID         string      `json:"id,omitempty"`
	Extension  []Extension `json:"extension,omitempty"`
	Value      *float64    `json:"value,omitempty"`
	Comparator string      `json:"comparator,omitempty"`
	Unit       string      `json:"unit,omitempty"`
	System     string      `json:"system,omitempty"`
	Code       string      `json:"code,omitempty"`
}
type Timing struct {
	Event  []string         `json:"event,omitempty"`
	Repeat *TimingRepeat    `json:"repeat,omitempty"`
	Code   *CodeableConcept `json:"code,omitempty"`
}

type TimingRepeat struct {
	BoundsDuration *Duration `json:"boundsDuration,omitempty"`
	BoundsRange    *Range    `json:"boundsRange,omitempty"`
	BoundsPeriod   *Period   `json:"boundsPeriod,omitempty"`
	Count          *int      `json:"count,omitempty"`
	CountMax       *int      `json:"countMax,omitempty"`
	Duration       *float64  `json:"duration,omitempty"`
	DurationMax    *float64  `json:"durationMax,omitempty"`
	DurationUnit   string    `json:"durationUnit,omitempty"`
	Frequency      *int      `json:"frequency,omitempty"`
	FrequencyMax   *int      `json:"frequencyMax,omitempty"`
	Period         *float64  `json:"period,omitempty"`
	PeriodMax      *float64  `json:"periodMax,omitempty"`
	PeriodUnit     string    `json:"periodUnit,omitempty"`
	DayOfWeek      []string  `json:"dayOfWeek,omitempty"`
	TimeOfDay      []string  `json:"timeOfDay,omitempty"`
	When           []string  `json:"when,omitempty"`
	Offset         *int      `json:"offset,omitempty"`
}

type Annotation struct {
	AuthorReference *Reference `json:"authorReference,omitempty"`
	AuthorString    string     `json:"authorString,omitempty"`
	Time            string     `json:"time,omitempty"`
	Text            string     `json:"text" validate:"required"`
}

type Reference struct {
	Reference  string      `json:"reference,omitempty" bson:"reference,omitempty"`
	Type       string      `json:"type,omitempty" bson:"type,omitempty"`
	Identifier *Identifier `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Display    string      `json:"display,omitempty" bson:"display,omitempty"`
}

type Identifier struct {
	Use    string           `json:"use,omitempty" bson:"use,omitempty"`
	System string           `json:"system,omitempty" bson:"system,omitempty"`
	Value  string           `json:"value,omitempty" bson:"value,omitempty"`
	Period *Period          `json:"period,omitempty" bson:"period,omitempty"`
	Type   *CodeableConcept `json:"type,omitempty" bson:"type,omitempty"`
}

type CodeableConcept struct {
	Coding []Coding `json:"coding,omitempty" bson:"coding,omitempty"`
	Text   string   `json:"text,omitempty" bson:"text,omitempty"`
}

type Coding struct {
	System  string `json:"system,omitempty" bson:"system,omitempty"`
	Version string `json:"version,omitempty" bson:"version,omitempty"`
	Code    string `json:"code,omitempty" bson:"code,omitempty"`
	Display string `json:"display,omitempty" bson:"display,omitempty"`
}

type Period struct {
	Start string `json:"start,omitempty" bson:"start,omitempty"`
	End   string `json:"end,omitempty" bson:"end,omitempty"`
}

type HumanName struct {
	Use    string   `json:"use,omitempty" bson:"use,omitempty"`
	Text   string   `json:"text,omitempty" bson:"text,omitempty"`
	Family string   `json:"family,omitempty" bson:"family,omitempty"`
	Given  []string `json:"given,omitempty" bson:"given,omitempty"`
	Prefix []string `json:"prefix,omitempty" bson:"prefix,omitempty"`
}

type Meta struct {
	VersionId   string    `json:"versionId,omitempty" bson:"versionId,omitempty"`
	LastUpdated time.Time `json:"lastUpdated,omitempty" bson:"lastUpdated,omitempty"`
	Source      string    `json:"source,omitempty" bson:"source,omitempty"`
	Profile     []string  `json:"profile,omitempty" bson:"profile,omitempty"`
	Security    []Coding  `json:"security,omitempty" bson:"security,omitempty"`
	Tag         []Coding  `json:"tag,omitempty" bson:"tag,omitempty"`
}

// ContactPointSystemCode represents FHIR R4 ContactPoint.system code values
// Reference: https://hl7.org/fhir/R4/datatypes.html#ContactPoint
type ContactPointSystemCode string

const (
	ContactPointSystemPhone ContactPointSystemCode = "phone"
	ContactPointSystemFax   ContactPointSystemCode = "fax"
	ContactPointSystemEmail ContactPointSystemCode = "email"
	ContactPointSystemPager ContactPointSystemCode = "pager"
	ContactPointSystemURL   ContactPointSystemCode = "url"
	ContactPointSystemSMS   ContactPointSystemCode = "sms"
	ContactPointSystemOther ContactPointSystemCode = "other"
)

type ContactPoint struct {
	System ContactPointSystemCode `json:"system"`
	Value  string                 `json:"value"`
	Use    string                 `json:"use"`
}

type Qualification struct {
	Identifier []Identifier    `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Code       CodeableConcept `json:"code" bson:"code"`
	Period     Period          `json:"period,omitempty" bson:"period,omitempty"`
	Issuer     Reference       `json:"issuer,omitempty" bson:"issuer,omitempty"`
}

type Attachment struct {
	ContentType string `json:"contentType,omitempty"`
	Language    string `json:"language,omitempty"`
	Data        string `json:"data,omitempty"`
	Url         string `json:"url,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Hash        string `json:"hash,omitempty"`
	Title       string `json:"title,omitempty"`
	Creation    string `json:"creation,omitempty"`
}

type Narrative struct {
	Status string `json:"status,omitempty"`
	Div    string `json:"div,omitempty"`
}

type ContactDetail struct {
	Name    string         `json:"name,omitempty"`
	Telecom []ContactPoint `json:"telecom,omitempty"`
}

type UsageContext struct {
	Code  Coding      `json:"code,omitempty"`
	Value interface{} `json:"value,omitempty"`
}
type Quantity struct {
	Value      float64 `json:"value,omitempty"`
	Comparator string  `json:"comparator,omitempty"`
	Unit       string  `json:"unit,omitempty"`
	System     string  `json:"system,omitempty"`
	Code       string  `json:"code,omitempty"`
}
type Money struct {
	Value    float64 `json:"value,omitempty"`
	Currency string  `json:"currency,omitempty"`
}

type Extension struct {
	Url            string    `json:"url,omitempty"`
	ValueString    string    `json:"valueString,omitempty"`
	ValueCode      string    `json:"valueCode,omitempty"`
	ValueInt       int       `json:"valueInt,omitempty"`
	ValueMoney     *Money    `json:"valueMoney,omitempty"`
	ValueBoolean   bool      `json:"valueBoolean,omitempty" bson:"valueBoolean,omitempty"`
	ValueInteger   int       `json:"valueInteger,omitempty" bson:"valueInteger,omitempty"`
	ValueDecimal   float64   `json:"valueDecimal,omitempty" bson:"valueDecimal,omitempty"`
	ValueUri       string    `json:"valueUri,omitempty" bson:"valueUri,omitempty"`
	ValueId        string    `json:"valueId,omitempty" bson:"valueId,omitempty"`
	ValueDate      string    `json:"valueDate,omitempty" bson:"valueDate,omitempty"`
	ValueDateTime  time.Time `json:"valueDateTime,omitempty" bson:"valueDateTime,omitempty"`
	ValueTime      string    `json:"valueTime,omitempty" bson:"valueTime,omitempty"`
	ValueCoding    Coding    `json:"valueCoding,omitempty" bson:"valueCoding,omitempty"`
	ValueQuantity  Quantity  `json:"valueQuantity,omitempty" bson:"valueQuantity,omitempty"`
	ValueReference Reference `json:"valueReference,omitempty" bson:"valueReference,omitempty"`
}

type Component struct {
	Code        CodeableConcept `json:"code"`
	ValueString string          `json:"valueString,omitempty"`
}
type Address struct {
	Use        string   `json:"use,omitempty"`
	Line       []string `json:"line,omitempty"`
	City       string   `json:"city,omitempty"`
	State      string   `json:"state,omitempty"`
	PostalCode string   `json:"postalCode,omitempty"`
	Country    string   `json:"country,omitempty"`
}

type AvailableTime struct {
	DaysOfWeek         []string `json:"daysOfWeek,omitempty"`
	AvailableStartTime string   `json:"availableStartTime,omitempty"`
	AvailableEndTime   string   `json:"availableEndTime,omitempty"`
}
