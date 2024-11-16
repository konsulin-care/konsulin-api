package requests

type Pagination struct {
	Page     int
	PageSize int
}

type QueryParams struct {
	Search         string
	FetchType      string
	PatientID      string
	PractitionerID string
}
