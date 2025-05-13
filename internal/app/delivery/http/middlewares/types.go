package middlewares

import (
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"

	"go.uber.org/zap"
)

func NewMiddlewares(logger *zap.Logger, sessionService contracts.SessionService, authUsecase contracts.AuthUsecase, internalConfig *config.InternalConfig) *Middlewares {
	return &Middlewares{
		Log:            logger,
		SessionService: sessionService,
		AuthUsecase:    authUsecase,
		InternalConfig: internalConfig,
	}
}

type ContextKey string
type Middlewares struct {
	Log            *zap.Logger
	AuthUsecase    contracts.AuthUsecase
	SessionService contracts.SessionService
	InternalConfig *config.InternalConfig
}

type User struct {
	ID    string
	Roles []string
}

const UserContextKey ContextKey = "user_context"

var rolePerms = map[string]map[string][]string{
	constvars.KonsulinRoleClinicAdmin: {
		constvars.ResourceOrganization:     {constvars.MethodGet},
		constvars.ResourcePractitioner:     {constvars.MethodGet},
		constvars.ResourcePractitionerRole: {constvars.MethodGet, constvars.MethodPost, constvars.MethodPut},
		constvars.ResourceSchedule:         {constvars.MethodPost},
		constvars.ResourceSlot:             {constvars.MethodGet},
	},
	constvars.KonsulinRoleGuest: {
		constvars.ResourceOrganization:          {constvars.MethodGet},
		constvars.ResourcePractitionerRole:      {constvars.MethodGet},
		constvars.ResourceQuestionnaire:         {constvars.MethodGet},
		constvars.ResourceQuestionnaireResponse: {constvars.MethodPost},
		constvars.ResourceResearchStudy:         {constvars.MethodGet},
		constvars.ResourceSlot:                  {constvars.MethodGet},
	},
	constvars.KonsulinRolePatient: {
		constvars.ResourceAppointment:           {constvars.MethodGet},
		constvars.ResourceObservation:           {constvars.MethodPost, constvars.MethodPut},
		constvars.ResourceOrganization:          {constvars.MethodGet},
		constvars.ResourcePatient:               {constvars.MethodDelete, constvars.MethodPost, constvars.MethodPut},
		constvars.ResourcePractitionerRole:      {constvars.MethodGet},
		constvars.ResourceQuestionnaire:         {constvars.MethodGet},
		constvars.ResourceQuestionnaireResponse: {constvars.MethodGet, constvars.MethodPost},
		constvars.ResourceResearchStudy:         {constvars.MethodGet},
		constvars.ResourceSlot:                  {constvars.MethodGet},
	},
	constvars.KonsulinRolePractitioner: {
		constvars.ResourceAppointment:           {constvars.MethodGet},
		constvars.ResourcePractitioner:          {constvars.MethodDelete, constvars.MethodPost, constvars.MethodPut},
		constvars.ResourcePractitionerRole:      {constvars.MethodGet, constvars.MethodPut},
		constvars.ResourceQuestionnaire:         {constvars.MethodGet, constvars.MethodPost},
		constvars.ResourceQuestionnaireResponse: {constvars.MethodPut},
		constvars.ResourceResearchStudy:         {constvars.MethodGet},
		constvars.ResourceSlot:                  {constvars.MethodGet},
	},
	constvars.KonsulinRoleSuperadmin: {
		constvars.ResourcePerson:                {constvars.MethodPost},
		constvars.ResourcePractitioner:          {constvars.MethodGet},
		constvars.ResourcePractitionerRole:      {constvars.MethodPost, constvars.MethodPut},
		constvars.ResourceQuestionnaire:         {constvars.MethodGet, constvars.MethodPost},
		constvars.ResourceQuestionnaireResponse: {constvars.MethodGet},
		constvars.ResourceSchedule:              {constvars.MethodPost},
	},
}
