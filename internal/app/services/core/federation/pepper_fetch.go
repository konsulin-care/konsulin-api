package federation

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/utils"

	"go.uber.org/zap"
)

// pepperListingName is the free federation-pepper webhook listing every party
// fetches its sealed pepper through after enrollment (BK-PEPPER-001).
const pepperListingName = "federation-pepper"

// pepperFetchPollInterval is how often FetchPepperAfterEnrollment re-checks
// the enroller's enrolled status before running the fetch once.
const pepperFetchPollInterval = 25 * time.Millisecond

// pepperFetchMkdirMode is the directory mode used when APP_DATA_DIR does not
// exist yet — the same mode pepper.go's tests create it with.
const pepperFetchMkdirMode = 0o750

// pepperFetchFileMode is the sealed pepper file mode: read/write for the
// owner only (BK-PEPPER-001, ciphertext at rest).
const pepperFetchFileMode = 0o600

// EnsureSealedPepper performs the post-enrollment sealed pepper fetch
// (BK-PEPPER-001): when no sealed file exists at
// <APP_DATA_DIR>/federation-pepper.jwe it discovers the active webhook
// listings through the gateway ListingDirectory, and with exactly one
// federation-pepper listing Sync-invokes it and stores the reply bytes
// verbatim with mode 0600. An existing sealed file prevents any fetch; zero
// or multiple listings, a discovery or sync failure, or an empty reply log a
// warning and keep the APP_PEPPER fallback — nothing else is ever written to
// disk.
// Parameters: ctx bounds the discovery and sync calls; directory reads the
// gateway listing set (may be nil, then the fetch is skipped); hooks is the
// gateway Sync client (may be nil, then the fetch is skipped); logger
// reports the outcome and tolerates nil.
// Returns nothing; never panics, never logs reply or file contents.
func EnsureSealedPepper(ctx context.Context, directory contracts.ListingDirectory, hooks contracts.WebhookClient, logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	dataDir := utils.GetEnvString("APP_DATA_DIR", "./data")
	path := filepath.Join(dataDir, pepperFileName)
	if _, err := os.Stat(path); err == nil {
		return // An existing sealed file prevents any fetch.
	} else if !errors.Is(err, fs.ErrNotExist) {
		logger.Warn("federation: sealed pepper presence check failed, keeping APP_PEPPER fallback", zap.Error(err))
		return
	}
	if directory == nil || hooks == nil {
		logger.Warn("federation: gateway listing directory or webhook client missing, keeping APP_PEPPER fallback")
		return
	}
	listings, err := directory.ListWebhookListings(ctx)
	if err != nil {
		logger.Warn("federation: webhook listing discovery failed, keeping APP_PEPPER fallback", zap.Error(err))
		return
	}
	var matches []contracts.Listing
	for _, listed := range listings {
		if listed.Name == pepperListingName {
			matches = append(matches, listed)
		}
	}
	if len(matches) != 1 {
		logger.Warn("federation: expected exactly one federation-pepper listing, keeping APP_PEPPER fallback",
			zap.Int("found", len(matches)))
		return
	}
	target := matches[0]
	reply, err := hooks.Sync(ctx, target.ProviderPartyID, target.Name, "", []byte("{}"))
	if err != nil {
		logger.Warn("federation: federation-pepper sync invoke failed, keeping APP_PEPPER fallback", zap.Error(err))
		return
	}
	if len(reply) == 0 {
		logger.Warn("federation: federation-pepper sync returned an empty reply, keeping APP_PEPPER fallback")
		return
	}
	if err := os.MkdirAll(dataDir, pepperFetchMkdirMode); err != nil {
		logger.Warn("federation: create APP_DATA_DIR for sealed pepper failed, keeping APP_PEPPER fallback", zap.Error(err))
		return
	}
	if err := os.WriteFile(path, reply, pepperFetchFileMode); err != nil {
		logger.Warn("federation: store sealed pepper failed, keeping APP_PEPPER fallback", zap.Error(err))
		return
	}
	if err := os.Chmod(path, pepperFetchFileMode); err != nil {
		logger.Warn("federation: seal pepper file permissions failed", zap.Error(err))
		return
	}
	logger.Info("federation: stored the sealed federation pepper fetched through the federation-pepper listing")
}

// FetchPepperAfterEnrollment polls the status until it reports enrolled (or
// ctx is done), then runs EnsureSealedPepper exactly once — the
// post-enrollment pepper fetch wired by registerFederation, which starts it
// in the background beside the enroller's own retry loop.
// Parameters: ctx bounds the wait and the fetch; status is the enroller
// published as contracts.GatewayStatus; directory and hooks are the gateway
// listing and Sync seams; logger reports and tolerates nil.
// Returns nothing; never panics.
func FetchPepperAfterEnrollment(ctx context.Context, status contracts.GatewayStatus, directory contracts.ListingDirectory, hooks contracts.WebhookClient, logger *zap.Logger) {
	if status == nil {
		return
	}
	ticker := time.NewTicker(pepperFetchPollInterval)
	defer ticker.Stop()
	for !status.Enrolled() {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
	EnsureSealedPepper(ctx, directory, hooks, logger)
}
