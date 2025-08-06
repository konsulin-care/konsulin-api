package middlewares

import (
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"

	"github.com/casbin/casbin/v2"
	"github.com/fsnotify/fsnotify"
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

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Fatal("failed to create policy watcher", zap.Error(err))
	}
	policyFile := "resources/rbac_policy.csv"
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					if err := enforcer.LoadPolicy(); err != nil {
						logger.Error("failed to reload RBAC policy", zap.Error(err))
					} else {
						logger.Info("RBAC policy reloaded", zap.String("file", event.Name))
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Error("policy watcher error", zap.Error(err))
			}
		}
	}()
	if err := watcher.Add(policyFile); err != nil {
		logger.Error("failed to watch policy file", zap.Error(err))
	}

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
