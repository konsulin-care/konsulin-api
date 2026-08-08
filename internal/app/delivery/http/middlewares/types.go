package middlewares

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// newEnforcer creates a Casbin enforcer with RBAC model and custom pathMatch function.
func newEnforcer(logger *zap.Logger) *casbin.Enforcer {
	enforcer, err := casbin.NewEnforcer("resources/rbac_model.conf", "resources/rbac_policy.csv")
	if err != nil {
		logger.Fatal("failed to load RBAC policies", zap.Error(err))
	}

	enforcer.AddFunction("pathMatch", func(args ...interface{}) (interface{}, error) {
		requestPath, policyPath, ok := matchPathArgs(args...)
		if !ok {
			return false, nil
		}
		return utils.PathMatch(requestPath, policyPath), nil
	})

	return enforcer
}

// matchPathArgs extracts the two pathMatch arguments, returning false when the
// argument shape is invalid.
func matchPathArgs(args ...interface{}) (requestPath, policyPath string, ok bool) {
	if len(args) != 2 {
		return "", "", false
	}
	requestPath, ok1 := args[0].(string)
	policyPath, ok2 := args[1].(string)
	if !ok1 || !ok2 {
		return "", "", false
	}
	return requestPath, policyPath, true
}

// handlePolicyEvent processes a single fsnotify event.
func handlePolicyEvent(event fsnotify.Event, enforcer *casbin.Enforcer, logger *zap.Logger) {
	if event.Op&fsnotify.Write != fsnotify.Write {
		return
	}
	if err := enforcer.LoadPolicy(); err != nil {
		logger.Error("failed to reload RBAC policy", zap.Error(err))
	} else {
		logger.Info("RBAC policy reloaded", zap.String("file", event.Name))
	}
}

// watchPolicyEvents processes fsnotify events in a loop until the watcher is closed.
func watchPolicyEvents(watcher *fsnotify.Watcher, enforcer *casbin.Enforcer, logger *zap.Logger) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			handlePolicyEvent(event, enforcer, logger)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logger.Error("policy watcher error", zap.Error(err))
		}
	}
}

// startPolicyWatcher monitors the RBAC policy CSV file for changes and reloads automatically.
func startPolicyWatcher(enforcer *casbin.Enforcer, logger *zap.Logger) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Fatal("failed to create policy watcher", zap.Error(err))
	}

	go watchPolicyEvents(watcher, enforcer, logger)

	if err := watcher.Add("resources/rbac_policy.csv"); err != nil {
		logger.Error("failed to watch policy file", zap.Error(err))
	}
}

// newHTTPClient creates an HTTP client with sensible defaults.
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{MaxIdleConnsPerHost: 100},
	}
}

func NewMiddlewares(
	logger *zap.Logger,
	authUsecase contracts.AuthUsecase,
	internalConfig *config.InternalConfig,
	practitionerFhirClient contracts.PractitionerFhirClient,
	patientFhirClient contracts.PatientFhirClient,
	practitionerRoleFhirClient contracts.PractitionerRoleFhirClient,
	scheduleFhirClient contracts.ScheduleFhirClient,
	questionnaireResponseFhirClient contracts.QuestionnaireResponseFhirClient,
	planDefinitionFhirClient contracts.PlanDefinitionFinder,
) *Middlewares {
	enforcer := newEnforcer(logger)
	startPolicyWatcher(enforcer, logger)
	return &Middlewares{
		Log:                             logger,
		AuthUsecase:                     authUsecase,
		InternalConfig:                  internalConfig,
		PractitionerFhirClient:          practitionerFhirClient,
		PatientFhirClient:               patientFhirClient,
		PractitionerRoleFhirClient:      practitionerRoleFhirClient,
		ScheduleFhirClient:              scheduleFhirClient,
		QuestionnaireResponseFhirClient: questionnaireResponseFhirClient,
		PlanDefinitionFinder:            planDefinitionFhirClient,
		Enforcer:                        enforcer,
		HTTPClient:                      newHTTPClient(),
	}
}

type (
	ContextKey  string
	Middlewares struct {
		Log                             *zap.Logger
		AuthUsecase                     contracts.AuthUsecase
		InternalConfig                  *config.InternalConfig
		PractitionerFhirClient          contracts.PractitionerFhirClient
		PatientFhirClient               contracts.PatientFhirClient
		PractitionerRoleFhirClient      contracts.PractitionerRoleFhirClient
		ScheduleFhirClient              contracts.ScheduleFhirClient
		QuestionnaireResponseFhirClient contracts.QuestionnaireResponseFhirClient
		PlanDefinitionFinder            contracts.PlanDefinitionFinder
		Enforcer                        *casbin.Enforcer

		// HTTPClient is a client for sending HTTP requests and can be reused for all requests.
		HTTPClient *http.Client

		// PostFHIRProxyHooks run after a successful FHIR proxy response (status < 400), before response filtering.
		// Hooks are called synchronously; on error the middleware only logs and continues.
		PostFHIRProxyHooks []PostFHIRProxyHook
	}
)

// PostFHIRProxyUserRequestDetail carries request data for post-FHIR-proxy hooks.
type PostFHIRProxyUserRequestDetail struct {
	Context context.Context // Request context (for cancellation, etc.)
	Method  string          // HTTP method (GET, POST, PUT, PATCH, DELETE)
	Path    string          // Request path (e.g. /fhir/PractitionerRole/123 or /fhir)
	Body    []byte          // Raw request body (e.g. for Bundle or single resource)
}

// PostFHIRProxyFHIRServerResponse carries the FHIR server response for post-FHIR-proxy hooks.
type PostFHIRProxyFHIRServerResponse struct {
	StatusCode int    // HTTP status from FHIR server
	Body       []byte // Raw response body
}

// PostFHIRProxyHook is called after a successful proxied FHIR request. Both params are structs for extensibility.
type PostFHIRProxyHook func(PostFHIRProxyUserRequestDetail, PostFHIRProxyFHIRServerResponse) error
