package payments

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testAppointmentIDs are the six canonical ids used across this suite.
var testAppointmentIDs = []string{"slot-1", "pr-2", "pat-3", "inv-4", "hs-5", "appt-6"}

func TestBuildAppointmentOpaque(t *testing.T) {
	t.Run("joins the six ids in order with '.'", func(t *testing.T) {
		opaque, err := BuildAppointmentOpaque(testAppointmentIDs)
		require.NoError(t, err)
		assert.Equal(t, "slot-1.pr-2.pat-3.inv-4.hs-5.appt-6", opaque)
	})

	t.Run("rejects an id containing '.'", func(t *testing.T) {
		ids := append([]string{"slot.1"}, testAppointmentIDs[1:]...)
		_, err := BuildAppointmentOpaque(ids)
		require.Error(t, err)
	})

	t.Run("rejects an opaque longer than 512 characters", func(t *testing.T) {
		ids := append([]string{strings.Repeat("a", 513)}, testAppointmentIDs[1:]...)
		_, err := BuildAppointmentOpaque(ids)
		require.Error(t, err)
	})

	t.Run("rejects a count other than six ids", func(t *testing.T) {
		_, err := BuildAppointmentOpaque(testAppointmentIDs[:5])
		require.Error(t, err)
	})
}

func TestParseAppointmentExternalIDForParty(t *testing.T) {
	const ownParty = "party-own"
	valid := ownParty + ":appointment:slot-1.pr-2.pat-3.inv-4.hs-5.appt-6"

	t.Run("returns the six ids for the own party", func(t *testing.T) {
		ids, err := ParseAppointmentExternalID(valid, ownParty)
		require.NoError(t, err)
		assert.Equal(t, testAppointmentIDs, ids)
	})

	t.Run("round-trips BuildAppointmentOpaque output", func(t *testing.T) {
		opaque, err := BuildAppointmentOpaque(testAppointmentIDs)
		require.NoError(t, err)
		ids, err := ParseAppointmentExternalID(ownParty+":appointment:"+opaque, ownParty)
		require.NoError(t, err)
		assert.Equal(t, testAppointmentIDs, ids)
	})

	t.Run("rejects a foreign party", func(t *testing.T) {
		_, err := ParseAppointmentExternalID(valid, "party-other")
		require.Error(t, err)
	})

	t.Run("rejects a non-appointment domain", func(t *testing.T) {
		foreign := ownParty + ":webhook:slot-1.pr-2.pat-3.inv-4.hs-5.appt-6"
		_, err := ParseAppointmentExternalID(foreign, ownParty)
		require.Error(t, err)
	})

	t.Run("rejects an opaque longer than 512 characters", func(t *testing.T) {
		long := ownParty + ":appointment:" + strings.Repeat("a", 513)
		_, err := ParseAppointmentExternalID(long, ownParty)
		require.Error(t, err)
	})

	t.Run("rejects a malformed external id", func(t *testing.T) {
		_, err := ParseAppointmentExternalID("only-two-segments", ownParty)
		require.Error(t, err)
	})
}
