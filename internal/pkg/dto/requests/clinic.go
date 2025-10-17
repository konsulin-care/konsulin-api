package requests

type FindAllCliniciansByClinicID struct {
	Page             int
	PageSize         int
	ClinicID         string
	PractitionerName string
	City             string
	Days             string
	StartTime        string
	EndTime          string
}
