package slot

import (
	"encoding/json"
	"errors"
	"fmt"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ConvertAvailableTimeToWeeklyPlan maps FHIR AvailableTime entries to an internal weeklyPlan.
// It performs strict validation and fails fast on the first invalid entry. This ensures
// writer and reader remain consistent: if another feature wrote bad data, slot generation
// will stop early and surface the problem rather than silently diverge.
// This function is one of the must used function by other feature that relates
// to Slot generation to ensure consistency and correctness of the Slot generation.
func ConvertAvailableTimeToWeeklyPlan(avts []fhir_dto.AvailableTime) (weeklyPlan, error) {
	var wp weeklyPlan
	for i, a := range avts {
		if a.AvailableStartTime == "" || a.AvailableEndTime == "" {
			return weeklyPlan{}, fmt.Errorf("availableTime[%d]: missing start/end time", i)
		}
		start, ok1 := parseClockFlex(a.AvailableStartTime)
		if !ok1 {
			return weeklyPlan{}, fmt.Errorf("availableTime[%d]: invalid start time '%s'", i, a.AvailableStartTime)
		}
		end, ok2 := parseClockFlex(a.AvailableEndTime)
		if !ok2 {
			return weeklyPlan{}, fmt.Errorf("availableTime[%d]: invalid end time '%s'", i, a.AvailableEndTime)
		}
		if !validWindow(start, end) {
			return weeklyPlan{}, fmt.Errorf("availableTime[%d]: start >= end (%02d:%02d >= %02d:%02d)", i, start.H, start.M, end.H, end.M)
		}
		if len(a.DaysOfWeek) == 0 {
			return weeklyPlan{}, fmt.Errorf("availableTime[%d]: empty daysOfWeek", i)
		}
		w := dayWindow{Start: start, End: end}
		hadDay := false
		for _, tok := range a.DaysOfWeek {
			mapped := mapDayToken(tok)
			if len(mapped) == 0 {
				return weeklyPlan{}, fmt.Errorf("availableTime[%d]: unknown day token '%s'", i, tok)
			}
			hadDay = true
			for _, wd := range mapped {
				appendWindow(&wp, wd, w)
			}
		}
		if !hadDay {
			return weeklyPlan{}, errors.New("no valid days mapped")
		}
	}
	return wp, nil
}

func parseClockFlex(s string) (clock, bool) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ".", ":")
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return clock{}, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return clock{}, false
	}
	return clock{H: h, M: m}, true
}

func validWindow(a, b clock) bool {
	return a.H*60+a.M < b.H*60+b.M
}

func mapDayToken(s string) []time.Weekday {
	t := strings.ToLower(strings.TrimSpace(s))
	switch t {
	case "mon", "monday":
		return []time.Weekday{time.Monday}
	case "tue", "tues", "tuesday":
		return []time.Weekday{time.Tuesday}
	case "wed", "wednesday":
		return []time.Weekday{time.Wednesday}
	case "thu", "thur", "thurs", "thursday":
		return []time.Weekday{time.Thursday}
	case "fri", "friday":
		return []time.Weekday{time.Friday}
	case "sat", "saturday":
		return []time.Weekday{time.Saturday}
	case "sun", "sunday":
		return []time.Weekday{time.Sunday}
	}
	return nil
}

func appendWindow(wp *weeklyPlan, wd time.Weekday, w dayWindow) {
	switch wd {
	case time.Monday:
		wp.Monday = append(wp.Monday, w)
	case time.Tuesday:
		wp.Tuesday = append(wp.Tuesday, w)
	case time.Wednesday:
		wp.Wednesday = append(wp.Wednesday, w)
	case time.Thursday:
		wp.Thursday = append(wp.Thursday, w)
	case time.Friday:
		wp.Friday = append(wp.Friday, w)
	case time.Saturday:
		wp.Saturday = append(wp.Saturday, w)
	case time.Sunday:
		wp.Sunday = append(wp.Sunday, w)
	}
}

// ScheduleConfig holds per-schedule generation parameters parsed from Schedule.comment JSON.
type ScheduleConfig struct {
	SlotMinutes   int `json:"slotMinutes"`
	BufferMinutes int `json:"bufferMinutes"`
}

// ParseScheduleConfig parses the comment JSON.
func ParseScheduleConfig(comment string) (ScheduleConfig, error) {
	var c ScheduleConfig
	if err := json.Unmarshal([]byte(comment), &c); err != nil {
		return ScheduleConfig{}, err
	}
	if c.SlotMinutes <= 0 {
		return ScheduleConfig{}, fmt.Errorf("invalid slotMinutes: %d", c.SlotMinutes)
	}
	if c.BufferMinutes < 0 {
		return ScheduleConfig{}, fmt.Errorf("invalid bufferMinutes: %d", c.BufferMinutes)
	}
	return c, nil
}

// Coverage classification
type dayCoverage int

const (
	// coverageNone means no slots exist for the schedule within the local day window.
	// Action: generate the full set of slots for this day.
	coverageNone dayCoverage = iota
	// CoverageAllFreeAuto means all existing slots are status=free and tagged as system-generated.
	// Action: already covered; do nothing.
	coverageAllFreeAuto
	// CoverageAllFreeNonAuto means all existing slots are status=free but at least one is not system-generated.
	// Action: delete free non-generated slots and recreate the day deterministically.
	coverageAllFreeNonAuto
	// CoverageConflict means at least one slot is non-free (e.g., busy/unavailable/tentative) or a mixed state.
	// Action: skip mutations for safety and let specialized conflict handling manage it later.
	coverageConflict
)

// classifyDayCoverageFromSlots inspects slots and returns coverage class and slot ids safe to delete / rewritten.
// It compares existing intervals against the expected intervals when all slots are free+auto to determine whether the existing slots is already match with
// the expected slots based on the given intervals
func classifyDayCoverageFromSlots(slots []fhir_dto.Slot, expected []interval) (dayCoverage, []string) {
	if len(slots) == 0 {
		return coverageNone, nil
	}
	allFree := true
	allAuto := true
	var nonAutoDeleteIDs []string
	var autoIDs []string
	for _, s := range slots {
		if s.Status != fhir_dto.SlotStatusFree {
			allFree = false
		}
		auto := false
		for _, t := range s.Meta.Tag {
			if t.Code == SlotTagSystemGenerated {
				auto = true
				break
			}
		}
		if auto {
			if s.ID != "" {
				autoIDs = append(autoIDs, s.ID)
			}
		} else {
			allAuto = false
			if s.Status == fhir_dto.SlotStatusFree && s.ID != "" {
				nonAutoDeleteIDs = append(nonAutoDeleteIDs, s.ID)
			}
		}
	}
	if !allFree {
		return coverageConflict, nil
	}
	if allAuto {
		existing := intervalsFromSlots(slots)
		if isIntervalsMatch(existing, expected) {
			return coverageAllFreeAuto, nil
		}
		return coverageAllFreeNonAuto, autoIDs
	}
	return coverageAllFreeNonAuto, nonAutoDeleteIDs
}

// idsOfAutoGeneratedFreeSlots returns IDs of slots that are free and have the system-generated tag.
// Used when erasing auto-generated free slots on uncovered days.
func idsOfAutoGeneratedFreeSlots(slots []fhir_dto.Slot) []string {
	var ids []string
	for _, s := range slots {
		if s.Status != fhir_dto.SlotStatusFree || s.ID == "" {
			continue
		}
		for _, t := range s.Meta.Tag {
			if t.Code == SlotTagSystemGenerated {
				ids = append(ids, s.ID)
				break
			}
		}
	}
	return ids
}

// intervalsFromSlots extracts intervals from free slots. Caller ensures slots are free when used in comparisons.
func intervalsFromSlots(slots []fhir_dto.Slot) []interval {
	out := make([]interval, 0, len(slots))
	for _, s := range slots {
		out = append(out, interval{Start: s.Start, End: s.End})
	}
	return out
}

// isIntervalsMatch returns true when two interval sets are exactly equal (order-agnostic).
func isIntervalsMatch(a, b []interval) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]struct{}, len(a))
	for _, iv := range a {
		m[intervalKey(iv.Start, iv.End)] = struct{}{}
	}
	for _, iv := range b {
		if _, ok := m[intervalKey(iv.Start, iv.End)]; !ok {
			return false
		}
	}
	return true
}

func intervalKey(a, b time.Time) string {
	return a.UTC().Format(time.RFC3339) + "|" + b.UTC().Format(time.RFC3339)
}

// conflictBlock represents a non-free slot's blocked interval and the buffer to apply after it.
type conflictBlock struct {
	Start         time.Time
	End           time.Time
	PostGapBuffer int // minutes to shift the next free gap's start if it starts exactly at End
}

// adjustIncomingSlotIntervalOnConflict carves the full working windows (baseWindows) by non-free blocks,
// then re-generates intervals per gap with the configured slot and buffer durations.
// Rule: after busy/busy-tentative → 0 post-gap buffer; after busy-unavailable → bufferMinutes.
func adjustIncomingSlotIntervalOnConflict(baseWindows []interval, existingSlots []fhir_dto.Slot, slotMinutes, bufferMinutes int) []interval {
	// 1) Build blocks with post-gap rule
	var blocks []conflictBlock
	for _, s := range existingSlots {
		if s.Status == fhir_dto.SlotStatusFree {
			continue
		}
		post := 0
		if s.Status == fhir_dto.SlotStatusBusyUnavailable {
			post = bufferMinutes
		}
		blocks = append(blocks, conflictBlock{Start: s.Start, End: s.End, PostGapBuffer: post})
	}
	if len(blocks) == 0 {
		// No conflicts; return original windows re-generated as discrete slots
		var noConflictOut []interval
		for _, w := range baseWindows {
			noConflictOut = append(noConflictOut, generateSlotsBetween(w.Start, w.End, slotMinutes, bufferMinutes)...)
		}
		return noConflictOut
	}

	// 2) Merge overlapping/adjacent blocks
	merged := mergeConflictBlocks(blocks)

	// 3) Subtract merged blocks from base windows → free gaps
	var gaps []interval
	for _, iv := range baseWindows {
		gaps = append(gaps, subtractBlocksFromInterval(iv, merged)...)
	}

	// 4) Apply post-gap buffer only when gap starts exactly at a block end
	var adjustedGaps []interval
	for _, g := range gaps {
		buf := bufferAtGapStartIfExactlyAtBlockEnd(g.Start, merged)
		gs := g.Start.Add(time.Duration(buf) * time.Minute)
		if gs.Before(g.End) {
			adjustedGaps = append(adjustedGaps, interval{Start: gs, End: g.End})
		}
	}
	if len(adjustedGaps) == 0 {
		return nil
	}

	// 5) Re-generate fixed-length slots per adjusted gap, re-anchored at gap start
	var out []interval
	for _, g := range adjustedGaps {
		out = append(out, generateSlotsBetween(g.Start, g.End, slotMinutes, bufferMinutes)...)
	}
	return out
}

// dayWorkIntervals converts day windows into continuous intervals on the given day and timezone.
func dayWorkIntervals(day time.Time, tz *time.Location, windows []dayWindow) []interval {
	if tz == nil {
		tz = time.Local
	}
	var out []interval
	for _, w := range windows {
		start := atClock(day, w.Start.H, w.Start.M, tz)
		end := atClock(day, w.End.H, w.End.M, tz)
		if end.After(start) {
			out = append(out, interval{Start: start, End: end})
		}
	}
	return out
}

func mergeConflictBlocks(blocks []conflictBlock) []conflictBlock {
	if len(blocks) <= 1 {
		return blocks
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].Start.Before(blocks[j].Start) })
	out := []conflictBlock{blocks[0]}
	for i := 1; i < len(blocks); i++ {
		last := &out[len(out)-1]
		b := blocks[i]
		// Overlap or touch: b.Start <= last.End
		if !b.Start.After(last.End) {
			if b.End.After(last.End) {
				last.End = b.End
			}
			if b.PostGapBuffer > last.PostGapBuffer {
				last.PostGapBuffer = b.PostGapBuffer
			}
			continue
		}
		out = append(out, b)
	}
	return out
}

// subtractBlocksFromInterval returns the list of free gaps within iv after removing blocks.
func subtractBlocksFromInterval(iv interval, merged []conflictBlock) []interval {
	var gaps []interval
	cur := iv.Start
	for _, b := range merged {
		// Skip blocks completely before/after iv
		if b.End.Before(iv.Start) || b.Start.After(iv.End) {
			continue
		}
		// If there's room before the block, add a gap
		if cur.Before(b.Start) {
			end := b.Start
			if end.After(iv.End) {
				end = iv.End
			}
			if cur.Before(end) {
				gaps = append(gaps, interval{Start: cur, End: end})
			}
		}
		// Advance cursor beyond the block
		if b.End.After(cur) {
			cur = b.End
		}
		if !cur.Before(iv.End) {
			break
		}
	}
	// Tail gap
	if cur.Before(iv.End) {
		gaps = append(gaps, interval{Start: cur, End: iv.End})
	}
	return gaps
}

func bufferAtGapStartIfExactlyAtBlockEnd(start time.Time, merged []conflictBlock) int {
	for _, b := range merged {
		if start.Equal(b.End) {
			return b.PostGapBuffer
		}
	}
	return 0
}

// differenceByIntervalKey returns intervals in a that are not present in b (keyed by Start/End).
func differenceByIntervalKey(a, b []interval) []interval {
	if len(a) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(b))
	for _, iv := range b {
		m[intervalKey(iv.Start, iv.End)] = struct{}{}
	}
	var out []interval
	for _, iv := range a {
		if _, ok := m[intervalKey(iv.Start, iv.End)]; !ok {
			out = append(out, iv)
		}
	}
	return out
}

// generateSlotsBetween produces fixed-length slots within [start,end) with a configurable buffer gap.
// Each slot has duration = slotMinutes; consecutive starts are spaced by slotMinutes + bufferMinutes.
// Any candidate whose end exceeds 'end' is dropped.
func generateSlotsBetween(start, end time.Time, slotMinutes, bufferMinutes int) []interval {
	if slotMinutes <= 0 {
		return nil
	}
	step := time.Duration(slotMinutes+bufferMinutes) * time.Minute
	lenSlot := time.Duration(slotMinutes) * time.Minute
	var out []interval
	for t := start; ; t = t.Add(step) {
		if t.Add(lenSlot).After(end) {
			break
		}
		out = append(out, interval{Start: t, End: t.Add(lenSlot)})
	}
	return out
}

// generateSlotsForDayWindows generates slots for all provided windows on a specific local day.
// Windows are independent; no slot crosses a window boundary.
func generateSlotsForDayWindows(day time.Time, tz *time.Location, windows []dayWindow, slotMinutes, bufferMinutes int) []interval {
	if tz == nil {
		tz = time.Local
	}
	var out []interval
	for _, w := range windows {
		dayStart := atClock(day, w.Start.H, w.Start.M, tz)
		dayEnd := atClock(day, w.End.H, w.End.M, tz)
		out = append(out, generateSlotsBetween(dayStart, dayEnd, slotMinutes, bufferMinutes)...)
	}
	return out
}

// atClock returns the time on 'day' at hour:minute in the given timezone.
func atClock(day time.Time, h, m int, loc *time.Location) time.Time {
	d := day.In(loc)
	y, mo, dd := d.Date()
	return time.Date(y, mo, dd, h, m, 0, 0, loc)
}

// groupSlotsByLocalDay returns a map from local calendar date (YYYY-MM-DD in loc) to slots
// whose Start time falls on that date. Slots spanning midnight are assigned to the day of Start.
func groupSlotsByLocalDay(slots []fhir_dto.Slot, loc *time.Location) map[string][]fhir_dto.Slot {
	if len(slots) == 0 {
		return nil
	}
	out := make(map[string][]fhir_dto.Slot)
	for _, s := range slots {
		dayKey := s.Start.In(loc).Format("2006-01-02")
		out[dayKey] = append(out[dayKey], s)
	}
	return out
}

// buildFHIRSlots maps intervals to FHIR Slots with the given schedule reference and status.
func buildFHIRSlots(scheduleID string, intervals []interval, status fhir_dto.SlotStatus) []fhir_dto.Slot {
	out := make([]fhir_dto.Slot, 0, len(intervals))
	for _, iv := range intervals {
		out = append(out, fhir_dto.Slot{
			ResourceType: "Slot",
			Schedule:     fhir_dto.Reference{Reference: "Schedule/" + scheduleID, Type: "Schedule"},
			Status:       status,
			Start:        iv.Start,
			End:          iv.End,
		})
	}
	return out
}

// SlotTagSystemGenerated marks system-generated slots in meta.tag.code.
// This tag is reserved to be used only when generating slot by current slot
// worker manager.
const SlotTagSystemGenerated = "system-generated"

// slotTagUserGenerated marks user-generated slots in meta.tag.code.
// This should only be used when a Slot is generated by the request of the user or
// is initated by user action, e.g when practitioner request unavailable time.
const slotTagUserGenerated = "user-generated"

// buildCreateSlotsTransactionBundle constructs a FHIR transaction Bundle for creating slots idempotently.
// Each POST entry includes ifNoneExist keyed by schedule+start (ISO8601 with offset).
func buildCreateSlotsTransactionBundle(scheduleID string, slots []fhir_dto.Slot) map[string]any {
	entries := make([]map[string]any, 0, len(slots))
	for _, s := range slots {
		startISO := s.Start.Format(time.RFC3339)
		entry := map[string]any{
			"request": map[string]any{
				"method":      "POST",
				"url":         "Slot",
				"ifNoneExist": "schedule=Schedule/" + scheduleID + "&start=" + url.QueryEscape(startISO),
			},
			"resource": map[string]any{
				"resourceType": "Slot",
				"schedule":     map[string]any{"reference": "Schedule/" + scheduleID},
				"status":       string(s.Status),
				"start":        startISO,
				"end":          s.End.Format(time.RFC3339),
				"meta": map[string]any{
					"tag": []map[string]any{{"code": SlotTagSystemGenerated}},
				},
			},
		}
		entries = append(entries, entry)
	}
	return map[string]any{
		"resourceType": "Bundle",
		"type":         "transaction",
		"entry":        entries,
	}
}

// buildOverrideSlotsTransactionBundle constructs a transaction Bundle that deletes specific
// Slot resources by id, then creates the supplied generated slots idempotently.
// Use this when a day has only free non-generated slots to replace.
func buildOverrideSlotsTransactionBundle(scheduleID string, deleteIDs []string, create []fhir_dto.Slot) map[string]any {
	entries := make([]map[string]any, 0, len(deleteIDs)+len(create))
	// Deletes first
	for _, id := range deleteIDs {
		if id == "" {
			continue
		}
		entries = append(entries, map[string]any{
			"request": map[string]any{
				"method": "DELETE",
				"url":    "Slot/" + id,
			},
		})
	}
	// Creates (conditional)
	for _, s := range create {
		startISO := s.Start.Format(time.RFC3339)
		entries = append(entries, map[string]any{
			"request": map[string]any{
				"method": "POST",
				"url":    "Slot",
				// this parameter ifNoneExist is not working as expected
				// because it will actually collides when there exists a Slot
				// that matches the same schedule and start time
				// for now will be deactivated and if later
				// arise new requirement to have idempotency, we can
				// first fix the usage of this params
				// and then re-enable it
				//"ifNoneExist": "schedule=Schedule/" + scheduleID + "&start=" + url.QueryEscape(startISO),
			},
			"resource": map[string]any{
				"resourceType": "Slot",
				"schedule":     map[string]any{"reference": "Schedule/" + scheduleID},
				"status":       string(s.Status),
				"start":        startISO,
				"end":          s.End.Format(time.RFC3339),
				"meta": map[string]any{
					"tag": []map[string]any{{"code": SlotTagSystemGenerated}},
				},
			},
		})
	}
	return map[string]any{
		"resourceType": "Bundle",
		"type":         "transaction",
		"entry":        entries,
	}
}

// BuildSlotAdjustmentForAppointment generates slot IDs to delete and slots to create
// when a new appointment is booked for a practitioner role. It applies the same
// adjustment rules used in automated slot generation to maintain consistency.
func BuildSlotAdjustmentForAppointment(
	practitionerRole fhir_dto.PractitionerRole,
	schedule fhir_dto.Schedule,
	existingSlots []fhir_dto.Slot,
	appointedStart, appointedEnd time.Time,
	appointedSlotID string,
	slotMinutes, bufferMinutes int,
) (toDelete []string, toCreate []fhir_dto.Slot, err error) {
	loc, tzErr := practitionerRole.GetPreferredTimezone()
	if tzErr != nil {
		return nil, nil, fmt.Errorf("failed to resolve timezone: %w", tzErr)
	}

	plan, planErr := ConvertAvailableTimeToWeeklyPlan(practitionerRole.AvailableTime)
	if planErr != nil {
		return nil, nil, fmt.Errorf("failed to convert available time to weekly plan: %w", planErr)
	}

	day := time.Date(appointedStart.In(loc).Year(), appointedStart.In(loc).Month(), appointedStart.In(loc).Day(), 0, 0, 0, 0, loc)

	windowsForDay := plan.forWeekday(day.Weekday())
	if len(windowsForDay) == 0 {
		// No configured windows for this day, nothing to adjust
		return nil, nil, nil
	}

	baseWindows := dayWorkIntervals(day, loc, windowsForDay)

	// Inject the appointed slot as a busy-unavailable block
	existingWithAppointed := make([]fhir_dto.Slot, len(existingSlots))
	copy(existingWithAppointed, existingSlots)
	existingWithAppointed = append(existingWithAppointed, fhir_dto.Slot{
		Status: fhir_dto.SlotStatusBusyUnavailable,
		Start:  appointedStart,
		End:    appointedEnd,
	})

	adjusted := adjustIncomingSlotIntervalOnConflict(baseWindows, existingWithAppointed, slotMinutes, bufferMinutes)

	var existingFree []fhir_dto.Slot
	for _, slt := range existingSlots {
		if slt.Status == fhir_dto.SlotStatusFree {
			existingFree = append(existingFree, slt)
		}
	}
	existingFreeIntervals := intervalsFromSlots(existingFree)

	// Determine slots to delete (free slots not in adjusted)
	adjustedSet := make(map[string]struct{}, len(adjusted))
	for _, iv := range adjusted {
		adjustedSet[intervalKey(iv.Start, iv.End)] = struct{}{}
	}

	for _, slt := range existingFree {
		if slt.ID == "" {
			continue
		}
		k := intervalKey(slt.Start, slt.End)
		if _, ok := adjustedSet[k]; !ok {
			toDelete = append(toDelete, slt.ID)
		}
	}

	missing := differenceByIntervalKey(adjusted, existingFreeIntervals)
	toCreate = buildFHIRSlots(schedule.ID, missing, fhir_dto.SlotStatusFree)

	// Also create busy-unavailable slot(s) covering the appointment overlap segments
	// clipped to the day's working windows. Avoid duplicating the booked role's slot
	// by checking for an existing slot with the exact interval and matching appointedSlotID.
	if appointedStart.Before(appointedEnd) {
		overlaps := intersectIntervalsWithWindows(appointedStart, appointedEnd, baseWindows)

		existingIntervalToIDs := make(map[string][]string)
		for _, s := range existingSlots {
			k := intervalKey(s.Start, s.End)
			existingIntervalToIDs[k] = append(existingIntervalToIDs[k], s.ID)
		}

		for _, ov := range overlaps {
			k := intervalKey(ov.Start, ov.End)

			// If any existing slot with the same interval is the appointed slot, skip creating busy here
			// because the caller will update that slot via PUT.
			skip := false
			if ids, ok := existingIntervalToIDs[k]; ok {
				for _, id := range ids {
					if id != "" && id == appointedSlotID {
						skip = true
						break
					}
				}
			}
			if skip {
				continue
			}

			// Otherwise, emit a busy-unavailable slot for the overlap interval.
			toCreate = append(toCreate, fhir_dto.Slot{
				ResourceType: "Slot",
				Schedule:     fhir_dto.Reference{Reference: "Schedule/" + schedule.ID, Type: "Schedule"},
				Status:       fhir_dto.SlotStatusBusyUnavailable,
				Start:        ov.Start,
				End:          ov.End,
			})
		}
	}

	return toDelete, toCreate, nil
}

// intersectIntervalsWithWindows returns appointment ∩ union(baseWindows)
func intersectIntervalsWithWindows(appStart, appEnd time.Time, windows []interval) []interval {
	var out []interval
	for _, w := range windows {
		s := w.Start
		if appStart.After(s) {
			s = appStart
		}
		e := w.End
		if appEnd.Before(e) {
			e = appEnd
		}
		if e.After(s) {
			out = append(out, interval{Start: s, End: e})
		}
	}
	return out
}
