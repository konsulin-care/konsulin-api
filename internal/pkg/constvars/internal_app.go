package constvars

type ContextKey string

const (
	ResourceUsers           = "users"
	ResourceRoles           = "roles"
	ResourceAuth            = "auth"
	ResourceClinics         = "clinics"
	ResourceGenders         = "genders"
	ResourceEducationLevels = "education-levels"
	ResourcePerson          = "Person"
)

const (
	RespondentTypeUser  = "user"
	RespondentTypeGuest = "guest"
)

const (
	AppPaginationUrlFormat = "%s?page=%d&page_size=%d"
)

const (
	WHATSAPP_OTP_LENGTH = 6
)
const (
	TIME_DIFFERENCE_JAKARTA       = 7
	TIME_DIFFERENCE_BANGKOK       = 7
	TIME_DIFFERENCE_NEW_YORK      = -5
	TIME_DIFFERENCE_LONDON        = 0
	TIME_DIFFERENCE_TOKYO         = 9
	TIME_DIFFERENCE_SYDNEY        = 11
	TIME_DIFFERENCE_DUBAI         = 4
	TIME_DIFFERENCE_SINGAPORE     = 8
	TIME_DIFFERENCE_HONG_KONG     = 8
	TIME_DIFFERENCE_BEIJING       = 8
	TIME_DIFFERENCE_DELHI         = 5.5
	TIME_DIFFERENCE_KARACHI       = 5
	TIME_DIFFERENCE_CAPE_TOWN     = 2
	TIME_DIFFERENCE_MOSCOW        = 3
	TIME_DIFFERENCE_RIO           = -3
	TIME_DIFFERENCE_SAN_FRANCISCO = -8
	TIME_DIFFERENCE_PARIS         = 1
	TIME_DIFFERENCE_BERLIN        = 1
	TIME_DIFFERENCE_ROME          = 1
	TIME_DIFFERENCE_CAIRO         = 2
)

const (
	CONTEXT_REQUEST_ID_KEY           ContextKey = "request_id"
	CONTEXT_SESSION_DATA_KEY         ContextKey = "session_data"
	CONTEXT_IS_CLIENT_REQUEST_ID_KEY ContextKey = "is_client_request_id"
	CONTEXT_STEPS_KEY                ContextKey = "steps"
	CONTEXT_RAW_BODY                 ContextKey = "raw_body"
)

const (
	REQUEST_ID_PREFIX = "KNSLN_SVC_"
)

const (
	KonsulinRoleGuest        = "Guest"
	KonsulinRolePatient      = "Patient"
	KonsulinRoleClinician    = "Clinician"
	KonsulinRoleResearcher   = "Researcher"
	KonsulinRoleSuperadmin   = "Superadmin"
	KonsulinRoleClinicAdmin  = "Clinic Admin"
	KonsulinRolePractitioner = "Practitioner"
)
