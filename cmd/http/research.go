package main

import (
	"context"
	"time"

	"github.com/konsulin-care/konsulin-api/internal/app/config"
	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/controllers"
	"github.com/konsulin-care/konsulin-api/internal/app/delivery/http/routers"
	"github.com/konsulin-care/konsulin-api/internal/app/drivers/sqlite"
	"github.com/konsulin-care/konsulin-api/internal/app/services/core/research"
	"github.com/konsulin-care/konsulin-api/internal/app/services/fhir_spark/resources"

	"go.uber.org/zap"
)

// registerResearch wires the research databank when it is enabled
// (BK-DATABANK-002): it opens the SQLite store at <APP_DATA_DIR>/research.db,
// builds the extractor over the generic FHIR resource searcher and starts
// the nightly scheduler, which runs Extract at 00:00 in APP_TIMEZONE. The
// store is closed through bootstrap.WorkerStop during shutdown, which also
// cancels the scheduler context. When the databank is disabled (empty
// APP_RESEARCH_PSEUDONYM_KEY) nothing is opened or started.
// Parameters: bootstrap carries the internal config, the FHIR base URL,
// the logger and the WorkerStop shutdown hook. Returns nil when the
// databank is disabled or wiring succeeded, or the store/scheduler
// construction error; never panics.
func registerResearch(bootstrap *config.Bootstrap) error {
	if bootstrap == nil || bootstrap.InternalConfig == nil {
		return nil
	}
	cfg := bootstrap.InternalConfig
	if !cfg.Research.DatabankEnabled() {
		// T76: publish the controller anyway with nil collaborators so the
		// routes register and every handler fails closed (503) instead of
		// the router hiding them behind a 404.
		routers.RegisterResearch(controllers.NewResearchController(bootstrap.Logger, nil, nil, nil))
		return nil // databank disabled: no store, no scheduler
	}

	store, err := sqlite.OpenResearchStore(cfg.App.DataDir)
	if err != nil {
		return err
	}

	loc, err := time.LoadLocation(cfg.App.Timezone)
	if err != nil {
		_ = store.Close()
		return err
	}

	searcher := resources.New(cfg.FHIR.BaseUrl, bootstrap.Logger)
	// T76: publish the research controller with the withdrawal flow wired
	// over the open store; the requester and exporter need gateway-driven
	// dependencies and stay nil for now, so their handlers fail closed.
	routers.RegisterResearch(controllers.NewResearchController(bootstrap.Logger,
		research.NewWithdrawer(searcher, store), nil, nil))
	extractor := research.NewExtractor(searcher, store,
		cfg.Research.PseudonymKey, cfg.Gateway.PartyID, nil)
	scheduler := research.NewScheduler(extractor, loc, nil, bootstrap.Logger)

	ctx, cancel := context.WithCancel(context.Background())
	go scheduler.Start(ctx)

	previousStop := bootstrap.WorkerStop
	bootstrap.WorkerStop = func() {
		cancel()
		if err := store.Close(); err != nil && bootstrap.Logger != nil {
			bootstrap.Logger.Warn("research: close store failed", zap.Error(err))
		}
		if previousStop != nil {
			previousStop()
		}
	}
	if bootstrap.Logger != nil {
		bootstrap.Logger.Info("research: databank enabled, nightly extraction scheduled",
			zap.String("timezone", cfg.App.Timezone))
	}
	return nil
}
