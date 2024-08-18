package requests

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
	Use    string   `json:"use"`
	Family string   `json:"family"`
	Given  []string `json:"given"`
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

type Money struct {
	Value    float64 `json:"value,omitempty"`
	Currency string  `json:"currency,omitempty"`
}

type Extension struct {
	Url         string `json:"url"`
	ValueString string `json:"valueString,omitempty"`
	ValueCode   string `json:"valueCode,omitempty"`
	ValueInt    int    `json:"valueInt,omitempty"`
	ValueMoney  Money  `json:"valueMoney,omitempty"`
}

type Address struct {
	Use        string   `json:"use"`
	Line       []string `json:"line"`
	City       string   `json:"city"`
	State      string   `json:"state"`
	PostalCode string   `json:"postalCode"`
	Country    string   `json:"country"`
}

type AvailableTime struct {
	DaysOfWeek         []string `json:"daysOfWeek"`
	AvailableStartTime string   `json:"availableStartTime"`
	AvailableEndTime   string   `json:"availableEndTime"`
}
