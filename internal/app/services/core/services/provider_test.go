package services

import (
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/app/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testGateway returns gateway config carrying the automation party id and the
// APP_SERVICE_PROVIDER_ALIASES list under test.
//
// Parameters:
//   - automationPartyID: the APP_AUTOMATION_PARTY_ID value.
//   - aliases: the APP_SERVICE_PROVIDER_ALIASES value.
//
// Returns: gateway config for NewProviderResolver.
func testGateway(automationPartyID, aliases string) *config.AppGateway {
	return &config.AppGateway{
		AutomationPartyID:      automationPartyID,
		ServiceProviderAliases: aliases,
	}
}

// TestResolveProviderAutomationAliasesAndRaw covers assertion 1: automation
// resolves to APP_AUTOMATION_PARTY_ID, configured aliases resolve to their
// party ids, and other values pass through as raw party ids.
func TestResolveProviderAutomationAliasesAndRaw(t *testing.T) {
	r := NewProviderResolver(testGateway(
		"01HZYAUTOMATION00000001",
		"konsulin-calendar=01HZYCALENDAR0000001, interpret = 01HZYINTERPRET000001",
	))

	t.Run("automation resolves to APP_AUTOMATION_PARTY_ID", func(t *testing.T) {
		partyID, err := r.ResolveProvider("automation")
		require.NoError(t, err)
		assert.Equal(t, "01HZYAUTOMATION00000001", partyID)
	})

	t.Run("configured aliases resolve to their party ids", func(t *testing.T) {
		partyID, err := r.ResolveProvider("konsulin-calendar")
		require.NoError(t, err)
		assert.Equal(t, "01HZYCALENDAR0000001", partyID)

		trimmed, err := r.ResolveProvider(" interpret ")
		require.NoError(t, err)
		assert.Equal(t, "01HZYINTERPRET000001", trimmed)
	})

	t.Run("other values pass through as raw party ids", func(t *testing.T) {
		partyID, err := r.ResolveProvider("01HZYRAWPARTY000000001")
		require.NoError(t, err)
		assert.Equal(t, "01HZYRAWPARTY000000001", partyID)
	})
}

// TestResolveProviderReservedInternalListingNames covers assertion 2: the
// reserved internal listing names are never user-callable — they resolve as
// not found even when an operator maps them as aliases.
func TestResolveProviderReservedInternalListingNames(t *testing.T) {
	r := NewProviderResolver(testGateway(
		"01HZYAUTOMATION00000001",
		"send-magiclink=01HZYSHADOW000000001",
	))

	for _, name := range []string{"send-magiclink", "notify-provider", "federation-pepper"} {
		t.Run(name+" resolves as not found", func(t *testing.T) {
			partyID, err := r.ResolveProvider(name)
			require.ErrorIs(t, err, ErrProviderNotFound)
			assert.Empty(t, partyID)
		})
	}
}

// TestResolveProviderAutomationMissingPartyID covers assertion 3: automation
// with an empty APP_AUTOMATION_PARTY_ID resolves as not found.
func TestResolveProviderAutomationMissingPartyID(t *testing.T) {
	r := NewProviderResolver(testGateway("", ""))

	partyID, err := r.ResolveProvider("automation")
	require.ErrorIs(t, err, ErrProviderNotFound)
	assert.Empty(t, partyID)
}

// TestResolveProviderEmptyAliasPartyID pins the fail-closed handling of an
// alias configured without a party id: it resolves as not found instead of
// falling through as a raw party id.
func TestResolveProviderEmptyAliasPartyID(t *testing.T) {
	r := NewProviderResolver(testGateway("", "broken="))

	partyID, err := r.ResolveProvider("broken")
	require.ErrorIs(t, err, ErrProviderNotFound)
	assert.Empty(t, partyID)
}
