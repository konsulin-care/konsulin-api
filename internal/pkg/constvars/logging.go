package constvars

const (
	LoggingRequestIDKey      = "request_id"
	LoggingDataKey           = "data"
	LoggingSessionDataKey    = "session_data"
	LoggingSessionIDKey      = "session_id"
	LoggingRawSessionDataKey = "raw_session_data"
	LoggingQueryParamsKey    = "query_params"

	LoggingFhirUrlKey = "fhir_url"

	LoggingResponseKey      = "response"
	LoggingResponseCountKey = "response_count"

	LoggingPractitionerIDKey = "practitioner_id"
	LoggingPatientIDKey      = "patient_id"

	LoggingRequestKey = "request"

	LoggingResponseLengthKey = "response_length"
	LoggingStepsKey          = "steps"

	LoggingRedisKey               = "redis_key"
	LoggingRedisValuesKey         = "redis_values"
	LoggingRedisMembersKey        = "redis_members"
	LoggingRedisAcquiredKey       = "redis_is_acquired"
	LoggingRedisExpirationTimeKey = "redis_expiration_time"

	LoggingLockValueKey          = "lock_value"
	LoggingLockExpirationTimeKey = "lock_expiration_time"
	LoggingLockStoredValueKey    = "lock_stored_value"
	LoggingLockExpectedValueKey  = "lock_expectedvalue"

	LoggingAppointmentCountKey = "appointments_count"
	LoggingAppointmentIDKey    = "appointment_id"

	LoggingQuestionnaireCountKey = "questionnaires_count"
	LoggingQuestionnaireIDKey    = "questionnaire_id"

	LoggingQuestionnaireResponseIDKey    = "questionnaire_response_id"
	LoggingQuestionnaireResponseCountKey = "questionnaire_response_count"

	LoggingAssessmentCountKey = "assessment_count"

	LoggingSlotsCountKey = "slots_count"
	LoggingSlotsIDKey    = "slot_id"
	LoggingSlotsStartKey = "slots_start_time"
	LoggingSlotsEndKey   = "slots_end_time"

	LoggingChargeItemDefinitionIDKey = "charge_item_definition_id"

	LoggingPractitionerRoleIDKey    = "practitioner_role_id"
	LoggingPractitionerRoleCountKey = "practitioner_roles_count"

	LoggingObservationIDKey = "observation_id"

	LoggingOrganizationCountKey = "organization_count"
	LoggingOrganizationIDKey    = "organization_id"

	LoggingScheduleIDKey     = "schedule_id"
	LoggingScheduleStatusKey = "schedule_status"
	LoggingScheduleCountKey  = "schedules_count"

	LoggingPaymentResponseKey = "payment_response"

	LoggingQueueNameKey = "queue_name"

	LoggingOyPaymentID        = "oy_payment_id"
	LoggingOyPaymentStatusKey = "oy_payment_status"
	LoggingOyUrlKey           = "oy_url"
)
