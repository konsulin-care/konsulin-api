package main

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/routers"
	"konsulin-service/internal/app/drivers/gateway"
	"konsulin-service/internal/app/services/core/federation"
	"konsulin-service/internal/app/services/core/payments"

	"go.uber.org/zap"
)

// settlementEventMaxBytes bounds the body read of one settlement notification.
const settlementEventMaxBytes = 1 << 20

// gatewayWiring bundles the federation components built once per process by
// newGateway and consumed by registerFederation (and later inbound gateway
// handlers): the outbound gateway adapter — the sole SDK importer — and the
// Redis replay cache that guards callback replays.
type gatewayWiring struct {
	client *gateway.Client
	replay *gateway.ReplayCache
}

// wiredGateway is the process-wide wiring newGateway stores and
// registerFederation reads. registerFederation's contract signature carries
// no wiring parameter, so the two calls made from bootstrapingTheApp hand
// the components over through this variable.
var wiredGateway *gatewayWiring

// newGateway builds the gateway federation components from bootstrap — the
// gateway adapter (an empty APP_GATEWAY_URL yields a disabled adapter that
// performs no I/O) plus the Redis replay cache — and stores them for
// registerFederation. It runs early in bootstrapingTheApp, before the
// usecases, so later wiring can depend on the adapter without editing
// main.go.
// Parameters: bootstrap carries the internal config (gateway env), logger
// and Redis client.
// Returns the wiring and nil, or nil and the adapter construction error.
// Never panics.
func newGateway(bootstrap *config.Bootstrap) (*gatewayWiring, error) {
	cfg := bootstrap.InternalConfig.Gateway
	client, err := gateway.New(cfg, bootstrap.InternalConfig.App.Env, bootstrap.Logger)
	if err != nil {
		return nil, err
	}
	wiring := &gatewayWiring{
		client: client,
		replay: gateway.NewReplayCache(bootstrap.Redis),
	}
	wiredGateway = wiring
	return wiring, nil
}

// registerFederation publishes the gateway status on ctrls.Gateway for the
// routers to consult per request, constructs the FederationController the
// federation router dispatches inbound events and deliveries through
// (handlers register on it by purpose and the T21B delivery listing registry
// registers on it by listing name inside this function), and — when the
// gateway is enabled — starts party enrollment in the background (first pass
// immediately, bounded backoff retries until enrolled). With an empty
// APP_GATEWAY_URL the status stays disabled and no enrollment, and therefore
// no network call, happens.
// gatewayEnv and paymentUsecase are parameters from day one so later tasks
// register inbound gateway handlers inside this function without editing
// main.go.
// Parameters: bootstrap supplies the logger; ctrls is the controller set
// SetupRoutes consumes; gatewayEnv is the gateway/party env configuration
// (APP_GATEWAY_URL plus APP_PARTY_*); paymentUsecase is the payment usecase
// inbound handlers will need. Returns nothing; never panics.
func registerFederation(bootstrap *config.Bootstrap, ctrls *routers.Controllers, gatewayEnv config.AppGateway, paymentUsecase contracts.PaymentUsecase) {
	logger := bootstrap.Logger
	// Constructed unconditionally: the federation router mounts even in
	// standalone mode (RequireGateway answers 503 before the handler runs).
	// The controller carries the T21B delivery registry: deliveries for
	// POST /federation/deliveries/{listing} dispatch through it, and
	// RegisterDeliveryHandler binds a handler per listing name here as
	// inbound delivery handlers land.
	ctrls.Federation = controllers.NewFederationController(logger)
	registerSettlementHandler(ctrls.Federation, paymentUsecase, logger)
	if wiredGateway == nil || wiredGateway.client == nil {
		if logger != nil {
			logger.Warn("federation: gateway wiring missing (newGateway not called), gateway stays disabled")
		}
		return
	}
	client := wiredGateway.client
	cfg := gatewayEnv
	// The adapter is the single source of truth: it is enabled only for a
	// non-empty APP_GATEWAY_URL.
	cfg.Enabled = client.Enabled()

	enroller := federation.NewEnroller(client, cfg, logger, nil)
	ctrls.Gateway = enroller
	if !cfg.Enabled {
		// Standalone mode: disabled status, no enrollment, no network calls.
		return
	}
	go enroller.Start(context.Background())
}

// registerSettlementHandler binds the payment usecase's HandleSettlement to
// the settlement-notification federation purpose: the controller dispatches
// the exact verified body, this function decodes it into a
// payments.SettlementEvent and hands it to the usecase. A payment usecase that
// does not implement payments.SettlementEventSink, or a missing controller,
// leaves the purpose unregistered (the controller then acknowledges the event).
// Parameters: fed is the federation controller to register on;
// paymentUsecase is the payment usecase from bootstrap; logger reports a
// skipped registration and tolerates nil. Returns nothing; never panics.
func registerSettlementHandler(fed *controllers.FederationController, paymentUsecase contracts.PaymentUsecase, logger *zap.Logger) {
	if fed == nil || paymentUsecase == nil {
		return
	}
	sink, ok := paymentUsecase.(payments.SettlementEventSink)
	if !ok {
		if logger != nil {
			logger.Warn("federation: payment usecase does not implement HandleSettlement, settlement-notification stays unregistered")
		}
		return
	}
	fed.RegisterEventHandler(controllers.PurposeSettlementNotification, func(r *http.Request) error {
		body, err := io.ReadAll(io.LimitReader(r.Body, settlementEventMaxBytes))
		if err != nil {
			return fmt.Errorf("settlement event: read body: %w", err)
		}
		event, err := payments.DecodeSettlementEvent(body)
		if err != nil {
			return err
		}
		return sink.HandleSettlement(r.Context(), event)
	})
}
