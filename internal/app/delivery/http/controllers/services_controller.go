package controllers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/core/services"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"

	"github.com/casbin/casbin/v2"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// servicePathPrefix is the API prefix every service POST path is built on.
const servicePathPrefix = "/api/v1/services"

// serviceInvokeMaxBytes bounds the client body read of one invocation.
const serviceInvokeMaxBytes = 1 << 20

// ServicesController serves marketplace service discovery (GET /services,
// T30 listings filtered by Casbin Enforce on each listing's POST path) and
// async invocation (POST /services/{provider}/{name}: T33 resolve, T37
// per-user limiter, T34 async invoke). Every deny fails closed with 404
// before any gateway call.
type ServicesController struct {
	Log        *zap.Logger
	Directory  contracts.ListingDirectory
	Resolver   *services.ProviderResolver
	Limiter    *services.ServiceLimiter
	Invoker    *services.Invoker
	Enforcer   *casbin.Enforcer
	GatewayEnv config.AppGateway
	AppConfig  *config.InternalConfig
}

// serviceListingResponse is one discovery row returned by GET /services.
type serviceListingResponse struct {
	ListingID   string `json:"listingId"`
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Price       int64  `json:"price"`
	Currency    string `json:"currency,omitempty"`
	PriceScope  string `json:"priceScope,omitempty"`
	Invocation  string `json:"invocation,omitempty"`
}

// NewServicesController builds the controller over the T30 directory, T33
// resolver, T37 limiter and T34 invoker, plus the gateway env used to
// reverse-map a party id to the alias segment its POST path uses. Logger,
// limiter, invoker and cfg may be nil (fail closed / quota 0 = unlimited).
// Returns a controller ready to mount. Never panics.
func NewServicesController(logger *zap.Logger, directory contracts.ListingDirectory,
	resolver *services.ProviderResolver, limiter *services.ServiceLimiter,
	invoker *services.Invoker, gatewayEnv config.AppGateway, cfg *config.InternalConfig,
) *ServicesController {
	return &ServicesController{
		Log:        logger,
		Directory:  directory,
		Resolver:   resolver,
		Limiter:    limiter,
		Invoker:    invoker,
		GatewayEnv: gatewayEnv,
		AppConfig:  cfg,
	}
}

// ListServices serves GET /api/v1/services: the T30 cached webhook
// listings filtered to the rows whose POST path at least one caller role
// is granted by Casbin Enforce. A nil enforcer or missing roles fail
// closed to an empty listing. Never panics.
func (ctrl *ServicesController) ListServices(w http.ResponseWriter, r *http.Request) {
	log := zap.NewNop()
	if ctrl != nil && ctrl.Log != nil {
		log = ctrl.Log
	}
	if ctrl == nil || ctrl.Directory == nil {
		utils.BuildErrorResponse(log, w, gatewayUnavailable())
		return
	}
	listings, err := ctrl.Directory.ListWebhookListings(r.Context())
	if err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	roles := serviceRoles(r.Context())
	out := make([]serviceListingResponse, 0, len(listings))
	for _, listing := range listings {
		path := servicePath(ctrl.providerSegment(listing.ProviderPartyID), listing.Name)
		if !ctrl.allowed(roles, http.MethodPost, path) {
			continue
		}
		out = append(out, serviceListingResponse{
			ListingID:   listing.ListingID,
			Provider:    ctrl.providerSegment(listing.ProviderPartyID),
			Name:        listing.Name,
			Description: listing.Description,
			Price:       listing.Price,
			Currency:    listing.Currency,
			PriceScope:  listing.PriceScope,
			Invocation:  listing.Invocation,
		})
	}
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResponseSuccess, out)
}

// InvokeService serves POST /api/v1/services/{provider}/{name}: authorize
// (T33 resolve + Casbin, both fail closed 404 before any gateway call),
// lookup the T30 listing (miss = 404), apply the T37 per-user limiter
// (deny = 429), queue the T34 async invocation and answer 202 with the
// receipt as requestId. Parameters: w receives the envelope; r carries
// the route params, roles and body. Never panics.
func (ctrl *ServicesController) InvokeService(w http.ResponseWriter, r *http.Request) {
	log := zap.NewNop()
	if ctrl != nil && ctrl.Log != nil {
		log = ctrl.Log
	}
	provider := strings.TrimSpace(chi.URLParam(r, "provider"))
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	partyID, err := ctrl.authorizeInvocation(r, provider, name)
	if err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	if err := ctrl.lookupServiceListing(r, partyID, name); err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	if ctrl.applyServiceRateLimit(w, r, log, name) {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, serviceInvokeMaxBytes))
	if err != nil {
		utils.BuildErrorResponse(log, w, exceptions.ErrReadBody(err))
		return
	}
	if ctrl.Invoker == nil {
		utils.BuildErrorResponse(log, w, gatewayUnavailable())
		return
	}
	out, err := ctrl.Invoker.Invoke(r.Context(), serviceCallerRef(r.Context()),
		partyID, name, raw, services.InvokeModeAsync)
	if err != nil {
		utils.BuildErrorResponse(log, w, err)
		return
	}
	if out == nil {
		utils.BuildErrorResponse(log, w, gatewayUnavailable())
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusAccepted, constvars.ResponseSuccess,
		map[string]string{"requestId": out.ServiceRequestID})
}

// authorizeInvocation resolves the provider (T33) and enforces the exact
// POST path through Casbin; every deny fails closed with the same 404
// before any gateway call. Returns the party id or the renderable error.
// Never panics.
func (ctrl *ServicesController) authorizeInvocation(r *http.Request, provider, name string) (string, error) {
	if provider == "" || name == "" || ctrl == nil || ctrl.Resolver == nil {
		return "", serviceNotFound()
	}
	partyID, err := ctrl.Resolver.ResolveProvider(provider)
	if err != nil {
		return "", serviceNotFound()
	}
	if !ctrl.allowed(serviceRoles(r.Context()), http.MethodPost, servicePath(provider, name)) {
		return "", serviceNotFound()
	}
	return partyID, nil
}

// lookupServiceListing resolves the listing through the T30 cached
// directory: nil directory = 503, miss = fail-closed 404, any other
// error passes through raw. Returns nil once the listing exists.
// Never panics.
func (ctrl *ServicesController) lookupServiceListing(r *http.Request, partyID, name string) error {
	if ctrl == nil || ctrl.Directory == nil {
		return gatewayUnavailable()
	}
	listing, err := ctrl.Directory.FindListing(r.Context(), partyID, name)
	if errors.Is(err, contracts.ErrListingNotFound) || (err == nil && listing == nil) {
		return serviceNotFound()
	}
	return err
}

// applyServiceRateLimit applies the T37 per-user fixed-window limiter:
// Redis error = 5xx, denial = 429 with Retry-After, nil limiter or quota
// 0 allows. Returns true when a response was written. Never panics.
func (ctrl *ServicesController) applyServiceRateLimit(w http.ResponseWriter, r *http.Request, log *zap.Logger, name string) bool {
	if ctrl == nil || ctrl.Limiter == nil {
		return false
	}
	var limit, window int
	if ctrl.AppConfig != nil {
		limit = ctrl.AppConfig.Webhook.SynchronousServiceRateLimit
		window = ctrl.AppConfig.Webhook.SynchronousServiceWindowSeconds
	}
	eval, err := ctrl.Limiter.Check(r.Context(), deriveActorID(r.Context()),
		name, limit, window, time.Now().UTC())
	if err != nil {
		utils.BuildErrorResponse(log, w, err)
		return true
	}
	return eval != nil && rejectRateLimited(log, w, eval.Allowed,
		eval.RetryAfterSecs, "SERVICE_RATE_LIMITED")
}

// allowed reports whether at least one caller role is granted method on
// path by Casbin Enforcer. A nil enforcer, nil controller or missing
// roles fail closed (false); a role that errs is skipped, never allowed.
// Parameters: roles are the caller's roles, method the HTTP method,
// path the exact POST path. Returns whether the call is granted.
// Never panics.
func (ctrl *ServicesController) allowed(roles []string, method, path string) bool {
	if ctrl == nil || ctrl.Enforcer == nil || len(roles) == 0 {
		return false
	}
	for _, role := range roles {
		normalized := strings.TrimSpace(role)
		if normalized == "" {
			continue
		}
		ok, err := ctrl.Enforcer.Enforce(normalized, method, path)
		if err != nil || !ok {
			continue
		}
		return true
	}
	return false
}

// providerSegment reverse-maps a listing's provider party id to the alias
// segment its POST path is addressed with: the automation party id maps to
// "automation", a configured alias=partyId entry maps to its alias, and
// anything else passes through as the raw party id.
// Parameters: partyID is the listing's providing party. Returns the
// provider path segment. Never panics.
func (ctrl *ServicesController) providerSegment(partyID string) string {
	gw := ctrl.GatewayEnv
	if gw.AutomationPartyID != "" && partyID == gw.AutomationPartyID {
		return "automation"
	}
	for _, entry := range strings.Split(gw.ServiceProviderAliases, ",") {
		alias, mapped, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		if mapped = strings.TrimSpace(mapped); mapped == "" || mapped != partyID {
			continue
		}
		if alias = strings.TrimSpace(alias); alias != "" {
			return alias
		}
	}
	return partyID
}

// servicePath builds /api/v1/services/{provider}/{name}. Never panics.
func servicePath(provider, name string) string {
	return fmt.Sprintf("%s/%s/%s", servicePathPrefix, provider, name)
}

// serviceRoles reads constvars.CONTEXT_FHIR_ROLE off ctx; a missing or
// non-[]string value yields nil. Never panics.
func serviceRoles(ctx context.Context) []string {
	roles, _ := ctx.Value(constvars.CONTEXT_FHIR_ROLE).([]string)
	return roles
}

// serviceCallerRef reads constvars.CONTEXT_UID off ctx for the async
// receipt requester; a missing uid yields "". Never panics.
func serviceCallerRef(ctx context.Context) string {
	uid, _ := ctx.Value(constvars.CONTEXT_UID).(string)
	return uid
}

// serviceNotFound renders the fail-closed 404 used for every unknown or
// forbidden provider/name so no distinction leaks to the caller.
func serviceNotFound() error {
	return exceptions.BuildNewCustomError(nil, constvars.StatusNotFound,
		"Service not found", "SERVICE_NOT_FOUND")
}

// gatewayUnavailable renders the 503 used when a required gateway seam
// (directory or invoker) is missing from the deployment wiring.
func gatewayUnavailable() error {
	return exceptions.BuildNewCustomError(nil, constvars.StatusServiceUnavailable,
		"Gateway not configured", "GATEWAY_NOT_CONFIGURED")
}
