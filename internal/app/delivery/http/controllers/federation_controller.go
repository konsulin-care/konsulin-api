package controllers

import (
	"net/http"
	"sync"

	"konsulin-service/internal/app/drivers/gateway"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"

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

// FederationController dispatches inbound federation events (POST
// /api/v1/federation/events) to handlers registered by purpose. It sits
// behind RequireGateway and the T20 VerifyMiddleware, outside the
// session/RBAC chain, so it reads the purpose from the verified token
// claims, never from the body.
type FederationController struct {
	Log      *zap.Logger
	handlers map[string]EventHandler
	mu       sync.RWMutex
}

// NewFederationController builds the federation event dispatcher with an
// empty handler registry; a nil logger is replaced by a no-op logger so
// dispatch never panics on logging.
//
// Parameters: logger is the application logger; nil is tolerated.
// Returns: a ready-to-wire controller. Never panics.
func NewFederationController(logger *zap.Logger) *FederationController {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &FederationController{
		Log:      logger,
		handlers: make(map[string]EventHandler),
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
