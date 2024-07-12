package requests

type HumanName struct {
	Use    string   `json:"use"`
	Family string   `json:"family"`
	Given  []string `json:"given"`
}

type ContactPoint struct {
	System string `json:"system"`
	Value  string `json:"value"`
	Use    string `json:"use"`
}

type Extension struct {
	Url         string `json:"url"`
	ValueString string `json:"valueString,omitempty"`
	ValueCode   string `json:"valueCode,omitempty"`
	ValueInt    int    `json:"valueInt,omitempty"`
}

type Address struct {
	Use        string   `json:"use"`
	Line       []string `json:"line"`
	City       string   `json:"city"`
	State      string   `json:"state"`
	PostalCode string   `json:"postalCode"`
	Country    string   `json:"country"`
}
