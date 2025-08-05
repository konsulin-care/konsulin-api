package middlewares

import (
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"

	"github.com/casbin/casbin/v2"
	"go.uber.org/zap"
)

func NewMiddlewares(
	logger *zap.Logger,
	sessionService contracts.SessionService,
	authUsecase contracts.AuthUsecase,
	internalConfig *config.InternalConfig,
	practitionerFhirClient contracts.PractitionerFhirClient,
	patientFhirClient contracts.PatientFhirClient,
) *Middlewares {
	enforcer, err := casbin.NewEnforcer("resources/rbac_model.conf", "resources/rbac_policy.csv")
	if err != nil {
		logger.Fatal("failed to load RBAC policies", zap.Error(err))
	}

	enforcer.AddFunction("owner", func(args ...interface{}) (interface{}, error) {
		return false, nil
	})

	return &Middlewares{
		Log:                    logger,
		SessionService:         sessionService,
		AuthUsecase:            authUsecase,
		InternalConfig:         internalConfig,
		PractitionerFhirClient: practitionerFhirClient,
		PatientFhirClient:      patientFhirClient,
		Enforcer:               enforcer,
	}
}

type ContextKey string
type Middlewares struct {
	Log                    *zap.Logger
	AuthUsecase            contracts.AuthUsecase
	SessionService         contracts.SessionService
	InternalConfig         *config.InternalConfig
	PractitionerFhirClient contracts.PractitionerFhirClient
	PatientFhirClient      contracts.PatientFhirClient
	Enforcer               *casbin.Enforcer
}

type User struct {
	ID    string
	Roles []string
}

const UserContextKey ContextKey = "user_context"
