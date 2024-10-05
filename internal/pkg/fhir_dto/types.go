package fhir_dto

import "time"

type Reference struct {
	Reference  string     `json:"reference,omitempty" bson:"reference,omitempty"`
	Type       string     `json:"type,omitempty" bson:"type,omitempty"`
	Identifier Identifier `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Display    string     `json:"display,omitempty" bson:"display,omitempty"`
}

type Identifier struct {
	Use    string          `json:"use,omitempty" bson:"use,omitempty"`
	System string          `json:"system,omitempty" bson:"system,omitempty"`
	Value  string          `json:"value,omitempty" bson:"value,omitempty"`
	Period Period          `json:"period,omitempty" bson:"period,omitempty"`
	Type   CodeableConcept `json:"type,omitempty" bson:"type,omitempty"`
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
type ContactPoint struct {
	System string `json:"system"`
	Value  string `json:"value"`
	Use    string `json:"use"`
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
	Url         string `json:"url,omitempty"`
	ValueString string `json:"valueString,omitempty"`
	ValueCode   string `json:"valueCode,omitempty"`
	ValueInt    int    `json:"valueInt,omitempty"`
	ValueMoney  *Money `json:"valueMoney,omitempty"`
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
