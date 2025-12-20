package middlewares

import (
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/utils"

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
	practitionerRoleFhirClient contracts.PractitionerRoleFhirClient,
	scheduleFhirClient contracts.ScheduleFhirClient,
	questionnaireResponseFhirClient contracts.QuestionnaireResponseFhirClient,
) *Middlewares {
	enforcer, err := casbin.NewEnforcer("resources/rbac_model.conf", "resources/rbac_policy.csv")
	if err != nil {
		logger.Fatal("failed to load RBAC policies", zap.Error(err))
	}

	enforcer.AddFunction("pathMatch", func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return false, nil
		}
		requestPath, ok1 := args[0].(string)
		policyPath, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return false, nil
		}
		return utils.PathMatch(requestPath, policyPath), nil
	})

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
		Log:                             logger,
		SessionService:                  sessionService,
		AuthUsecase:                     authUsecase,
		InternalConfig:                  internalConfig,
		PractitionerFhirClient:          practitionerFhirClient,
		PatientFhirClient:               patientFhirClient,
		PractitionerRoleFhirClient:      practitionerRoleFhirClient,
		ScheduleFhirClient:              scheduleFhirClient,
		QuestionnaireResponseFhirClient: questionnaireResponseFhirClient,
		Enforcer:                        enforcer,
	}
}

type ContextKey string
type Middlewares struct {
	Log                             *zap.Logger
	AuthUsecase                     contracts.AuthUsecase
	SessionService                  contracts.SessionService
	InternalConfig                  *config.InternalConfig
	PractitionerFhirClient          contracts.PractitionerFhirClient
	PatientFhirClient               contracts.PatientFhirClient
	PractitionerRoleFhirClient      contracts.PractitionerRoleFhirClient
	ScheduleFhirClient              contracts.ScheduleFhirClient
	QuestionnaireResponseFhirClient contracts.QuestionnaireResponseFhirClient
	Enforcer                        *casbin.Enforcer
}

type User struct {
	ID    string
	Roles []string
}

const UserContextKey ContextKey = "user_context"
