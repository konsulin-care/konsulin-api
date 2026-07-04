package contracts

import (
	"testing"
)

func TestSlotSearchParamsToQueryString(t *testing.T) {
	t.Run("returns empty string when no params set", func(t *testing.T) {
		p := SlotSearchParams{}
		result := p.ToQueryString()
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("includes start param when set", func(t *testing.T) {
		p := SlotSearchParams{Start: "lt2026-07-04T10:00:00Z"}
		result := p.ToQueryString()
		if result != "&start=lt2026-07-04T10%3A00%3A00Z" {
			t.Errorf("unexpected query string: %q", result)
		}
	})

	t.Run("includes both start and end as separate &start= params", func(t *testing.T) {
		p := SlotSearchParams{
			Start: "lt2026-07-04T10:00:00Z",
			End:   "gt2026-07-04T09:00:00Z",
		}
		result := p.ToQueryString()
		// End is serialized as a second &start= parameter (BLAZE workaround)
		if result != "&start=lt2026-07-04T10%3A00%3A00Z&start=gt2026-07-04T09%3A00%3A00Z" {
			t.Errorf("unexpected query string: %q", result)
		}
	})

	t.Run("includes status when set", func(t *testing.T) {
		p := SlotSearchParams{
			Start:  "lt2026-07-04T10:00:00Z",
			Status: "busy-unavailable",
		}
		result := p.ToQueryString()
		if result != "&start=lt2026-07-04T10%3A00%3A00Z&status=busy-unavailable" {
			t.Errorf("unexpected query string: %q", result)
		}
	})
}
