package constvars

// ServiceRequestSubject enumerates canonical subject references for non-Patient roles.
// Patient subjects require the specific Patient/<id> and are not represented here.
type ServiceRequestSubject string

const (
	// Group-based subjects for non-Patient roles (lowercased role slug)
	ServiceRequestSubjectGuest        ServiceRequestSubject = "Group/guest"
	ServiceRequestSubjectClinician    ServiceRequestSubject = "Group/clinician"
	ServiceRequestSubjectResearcher   ServiceRequestSubject = "Group/researcher"
	ServiceRequestSubjectSuperadmin   ServiceRequestSubject = "Group/superadmin"
	ServiceRequestSubjectClinicAdmin  ServiceRequestSubject = "Group/clinic-admin"
	ServiceRequestSubjectPractitioner ServiceRequestSubject = "Group/practitioner"
)

// DefaultGroups enumerates the group IDs required for ServiceRequest.subject references.
// These are the trailing IDs after "Group/".
var DefaultGroups = []string{
	"guest",
	"clinician",
	"researcher",
	"superadmin",
	"clinic-admin",
	"practitioner",
}
