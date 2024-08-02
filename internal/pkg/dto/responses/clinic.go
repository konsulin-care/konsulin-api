package responses

type Clinic struct {
	ID          string   `json:"clinic_id"`
	ClinicName  string   `json:"clinic_name"`
	Affiliation string   `json:"affiliation"`
	Address     string   `json:"address"`
	Tags        []string `json:"tags"`
}
