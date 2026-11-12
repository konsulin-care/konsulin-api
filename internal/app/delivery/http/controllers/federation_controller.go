package controllers

import (
	"net/http"
	"sync"

	"konsulin-service/internal/app/drivers/gateway"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Federation purpose keys: the four ks-event+jwt purposes the gateway sends
// to the party's endpoints.callbacks. The registry on FederationController
// is keyed by these values.
const (
	PurposeSettlementNotification = "settlement-notification"
	PurposeServiceResult          = "service-result"
	PurposePaymentRequired        = "payment-required"
	PurposeServiceReceipt         = "service-receipt"
)

// EventHandler processes one verified federation event. It receives the
// inbound request whose context carries the verified gateway claims
// (gateway.ClaimsFromContext) and whose body is the exact verified bytes;
// it must not write the HTTP response — the controller owns the status.
//
// Parameters: r is the inbound callback request.
// Returns: nil when the event was handled, or an error the controller
// maps to 500 so the gateway retries.
type EventHandler func(r *http.Request) error

// DeliveryHandler processes one verified federation delivery (POST
// /api/v1/federation/deliveries/{listing}). It receives the inbound request
// whose context carries the verified gateway claims (gateway.ClaimsFromContext)
// and whose body is the exact verified bytes; it must not write the HTTP
// response — the controller owns the status.
//
// Parameters: r is the inbound delivery request.
// Returns: nil when the delivery was handled, or an error the controller
// maps to 500 so the gateway retries.
type DeliveryHandler func(r *http.Request) error

// FederationController dispatches inbound federation events (POST
// /api/v1/federation/events) to handlers registered by purpose and
// inbound federation deliveries (POST /api/v1/federation/deliveries/{listing})
// to handlers registered by listing name. It sits behind RequireGateway and
// the T20 VerifyMiddleware, outside the session/RBAC chain, so it reads the
// purpose/service from the verified token claims, never from the body.
type FederationController struct {
	Log              *zap.Logger
	handlers         map[string]EventHandler
	deliveries       map[string]DeliveryHandler
	writerDeliveries map[string]func(http.ResponseWriter, *http.Request)
	mu               sync.RWMutex
}

// NewFederationController builds the federation dispatcher with empty event
// and delivery registries; a nil logger is replaced by a no-op logger so
// dispatch never panics on logging.
//
// Parameters: logger is the application logger; nil is tolerated.
// Returns: a ready-to-wire controller. Never panics.
func NewFederationController(logger *zap.Logger) *FederationController {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &FederationController{
		Log:              logger,
		handlers:         make(map[string]EventHandler),
		deliveries:       make(map[string]DeliveryHandler),
		writerDeliveries: make(map[string]func(http.ResponseWriter, *http.Request)),
	}
}

// RegisterEventHandler binds a handler to one federation purpose key
// (PurposeSettlementNotification, PurposeServiceResult,
// PurposePaymentRequired, PurposeServiceReceipt). Registering the same
// purpose twice replaces the previous handler; a nil handler unregisters.
// Safe for concurrent use.
//
// Parameters: purpose is the purpose key the handler answers for; handler
// is the event handler to dispatch to.
// Returns: nothing.
func (c *FederationController) RegisterEventHandler(purpose string, handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if handler == nil {
		delete(c.handlers, purpose)
		return
	}
	c.handlers[purpose] = handler
}

// RegisterDeliveryHandler binds a handler to one listing name — the
// {listing} path segment of POST /api/v1/federation/deliveries/{listing},
// which must equal the verified service claim of the delivery token.
// Registering the same listing twice replaces the previous handler; a nil
// handler unregisters the listing (it then answers 404). Safe for
// concurrent use.
//
// Parameters: listing is the listing name the handler answers for; handler
// is the delivery handler to dispatch to.
// Returns: nothing.
func (c *FederationController) RegisterDeliveryHandler(listing string, handler DeliveryHandler) {
	if handler == nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.deliveries, listing)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deliveries[listing] = handler
}

// RegisterDeliveryWriterHandler binds a response-writing handler to one
// listing name — for deliveries whose handler owns its own HTTP response
// (custom statuses and bodies such as 403/503 or an application/jose
// reply) and therefore cannot run through DeliveryHandler. Registering the
// same listing twice replaces the previous handler; a nil handler
// unregisters the listing. The verified service claim check in HandleDelivery
// still wins before this branch. Safe for concurrent use.
//
// Parameters: listing is the listing name the handler answers for; handler
// writes the response itself.
// Returns: nothing.
func (c *FederationController) RegisterDeliveryWriterHandler(listing string, handler func(http.ResponseWriter, *http.Request)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if handler == nil {
		delete(c.writerDeliveries, listing)
		return
	}
	c.writerDeliveries[listing] = handler
}

// HandleEvent serves POST /api/v1/federation/events: it reads the purpose
// from the verified claims, dispatches to the registered handler and
// answers 200 on success. A purpose with no registered handler is
// acknowledged 200 and logged (the gateway treats non-429 4xx/5xx as
// terminal, so an unhandled purpose must not retry forever). A handler
// error answers 500 so the gateway retries the event.
//
// Parameters: w is the response writer; r is the verified inbound event
// request.
// Returns: nothing; every path writes exactly one status.
func (c *FederationController) HandleEvent(w http.ResponseWriter, r *http.Request) {
	purpose := gateway.ClaimsFromContext(r.Context()).Purpose

	c.mu.RLock()
	handler, ok := c.handlers[purpose]
	c.mu.RUnlock()

	if !ok {
		c.Log.Info("federation: event purpose has no registered handler, acknowledged",
			zap.String("purpose", purpose))
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := handler(r); err != nil {
		c.Log.Error("federation: event handler failed, answering 500 for retry",
			zap.String("purpose", purpose),
			zap.Error(err))
		utils.BuildErrorResponse(c.Log, w, exceptions.BuildNewCustomError(err,
			constvars.StatusInternalServerError,
			"federation event handling failed",
			"FEDERATION_EVENT_FAILED"))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleDelivery serves POST /api/v1/federation/deliveries/{listing}: it
// reads the {listing} path segment and the verified service claim, answers
// 401 when the service claim differs from the path listing (the token is
// not authorized for that path), answers 404 when no handler is registered
// for the listing name, dispatches to the registered handler otherwise, and
// answers 200 on success. A handler error answers 500 so the gateway
// retries the delivery.
//
// Parameters: w is the response writer; r is the verified inbound delivery
// request.
// Returns: nothing; every path writes exactly one status.
func (c *FederationController) HandleDelivery(w http.ResponseWriter, r *http.Request) {
	listing := chi.URLParam(r, "listing")
	service := gateway.ClaimsFromContext(r.Context()).Service

	if service != listing {
		c.Log.Warn("federation: delivery service claim differs from path listing, denied",
			zap.String("listing", listing),
			zap.String("service", service))
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// A registered writer handler owns its whole response (status and
	// body); it runs after the service-claim check and returns without a
	// second status write.
	c.mu.RLock()
	writer, hasWriter := c.writerDeliveries[listing]
	c.mu.RUnlock()
	if hasWriter {
		writer(w, r)
		return
	}

	c.mu.RLock()
	handler, ok := c.deliveries[listing]
	c.mu.RUnlock()

	if !ok {
		c.Log.Info("federation: delivery listing has no registered handler, answered 404",
			zap.String("listing", listing))
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if err := handler(r); err != nil {
		c.Log.Error("federation: delivery handler failed, answering 500 for retry",
			zap.String("listing", listing),
			zap.Error(err))
		utils.BuildErrorResponse(c.Log, w, exceptions.BuildNewCustomError(err,
			constvars.StatusInternalServerError,
			"federation delivery handling failed",
			"FEDERATION_DELIVERY_FAILED"))
		return
	}

	w.WriteHeader(http.StatusOK)
}
