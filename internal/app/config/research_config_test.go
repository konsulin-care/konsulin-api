package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLoadInternalConfig_dataDirUnwritableFails asserts BK-CONF-002: an
// APP_DATA_DIR that cannot be created makes config loading fail with an
// actionable error naming APP_DATA_DIR. The blocker is a regular file so
// MkdirAll fails with ENOTDIR regardless of the user running the test.
func TestLoadInternalConfig_dataDirUnwritableFails(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("APP_GATEWAY_URL", "")

	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("writing blocker file: %v", err)
	}
	t.Setenv("APP_DATA_DIR", filepath.Join(blocker, "data"))

	cfg, err := loadInternalConfigWithEnv()

	assert.Error(t, err)
	assert.Nil(t, cfg)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "APP_DATA_DIR")
	}
}

// TestLoadInternalConfig_researchDefaultsDatabankDisabled asserts the research
// settings load with their documented defaults and that the databank reports
// disabled while APP_RESEARCH_PSEUDONYM_KEY is empty.
func TestLoadInternalConfig_researchDefaultsDatabankDisabled(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("APP_GATEWAY_URL", "")
	t.Setenv("APP_DATA_DIR", t.TempDir())
	t.Setenv("APP_AUTOPUBLISH", "")
	t.Setenv("APP_RESEARCH_PSEUDONYM_KEY", "")
	t.Setenv("APP_RESEARCH_EXPORT_MAX_BYTES", "")

	cfg, err := loadInternalConfigWithEnv()

	assert.NoError(t, err)
	if assert.NotNil(t, cfg) {
		assert.False(t, cfg.Research.AutoPublish)
		assert.Equal(t, 52428800, cfg.Research.ExportMaxBytes)
		assert.False(t, cfg.Research.DatabankEnabled())
	}
}

// TestLoadInternalConfig_researchSettingsFromEnv asserts the three research
// env vars bind into the config and that a non-empty pseudonym key enables
// the databank.
func TestLoadInternalConfig_researchSettingsFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("APP_GATEWAY_URL", "")
	t.Setenv("APP_DATA_DIR", t.TempDir())
	t.Setenv("APP_AUTOPUBLISH", "true")
	t.Setenv("APP_RESEARCH_PSEUDONYM_KEY", "research-pepper")
	t.Setenv("APP_RESEARCH_EXPORT_MAX_BYTES", "1048576")

	cfg, err := loadInternalConfigWithEnv()

	assert.NoError(t, err)
	if assert.NotNil(t, cfg) {
		assert.True(t, cfg.Research.AutoPublish)
		assert.Equal(t, "research-pepper", cfg.Research.PseudonymKey)
		assert.Equal(t, 1048576, cfg.Research.ExportMaxBytes)
		assert.True(t, cfg.Research.DatabankEnabled())
	}
}
