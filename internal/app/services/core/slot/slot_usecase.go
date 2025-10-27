package slot

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	bundleSvc "konsulin-service/internal/app/services/fhir_spark/bundle"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"sort"
	"time"

	"slices"

	"go.uber.org/zap"
)

type SlotUsecase struct {
	schedules         contracts.ScheduleFhirClient
	locker            contracts.LockerService
	slots             contracts.SlotFhirClient
	practitionerRoles contracts.PractitionerRoleFhirClient
	practitioner      contracts.PractitionerFhirClient
	bundles           bundleSvc.BundleFhirClient
	config            *config.InternalConfig
	logger            *zap.Logger
}

func NewSlotUsecase(
	schedules contracts.ScheduleFhirClient,
	locker contracts.LockerService,
	slots contracts.SlotFhirClient,
	practitionerRoles contracts.PractitionerRoleFhirClient,
	practitioner contracts.PractitionerFhirClient,
	bundles bundleSvc.BundleFhirClient,
	config *config.InternalConfig,
	logger *zap.Logger,
) *SlotUsecase {
	return &SlotUsecase{
		schedules:         schedules,
		locker:            locker,
		slots:             slots,
		practitionerRoles: practitionerRoles,
		practitioner:      practitioner,
		bundles:           bundles,
		config:            config,
		logger:            logger,
	}
}

func (s *SlotUsecase) HandleSetUnavailabilityForMultiplePractitionerRoles(ctx context.Context, input contracts.SetUnavailabilityForMultiplePractitionerRolesInput) (*contracts.SetUnavailableOutcome, error) {
	if err := input.Validate(); err != nil {
		return nil, exceptions.BuildNewCustomError(
			err,
			constvars.StatusBadRequest,
			constvars.ErrDevInvalidRequestPayload,
			"input validation failed for set unavailability",
		)
	}

	role, uid, authErr := s.whitelistAccessByRoles(
		ctx,
		[]string{
			constvars.KonsulinRolePractitioner,
			constvars.KonsulinRoleClinicAdmin,
		},
	)

	if authErr != nil {
		// surface error via logs; controller will handle response mapping when integrated
		s.logger.With(zap.Error(authErr)).Error("authorization failed for set unavailability")
		return nil, exceptions.BuildNewCustomError(
			authErr,
			constvars.StatusForbidden,
			constvars.ErrClientNotAuthorized,
			"authorization failed for set unavailability",
		)
	}

	// Load all requested PractitionerRoles
	roles := make([]fhir_dto.PractitionerRole, 0, len(input.PractitionerRoleIDs))
	for _, id := range input.PractitionerRoleIDs {
		pr, err := s.practitionerRoles.FindPractitionerRoleByID(ctx, id)
		if err != nil || pr == nil {
			s.logger.With(zap.Error(err), zap.String("practitioner_role_id", id)).Error("failed to load practitioner role")
			return nil, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to load practitioner role")
		}
		roles = append(roles, *pr)
	}

	unavailableReason := "Manual unavailability indicated by "

	// Practitioner ownership check
	if role == constvars.KonsulinRolePractitioner {
		practitioners, err := s.practitioner.FindPractitionerByIdentifier(
			ctx,
			constvars.FhirSupertokenSystemIdentifier,
			uid,
		)

		if err != nil {
			return nil, exceptions.BuildNewCustomError(
				err,
				http.StatusInternalServerError,
				err.Error(),
				err.Error(),
			)
		}

		if len(practitioners) != 1 {
			errMultiPracs := errors.New("multiple practitioners found on the same identifier or no practitioner found at all")
			return nil, exceptions.BuildNewCustomError(
				errMultiPracs,
				http.StatusBadRequest,
				errMultiPracs.Error(),
				errMultiPracs.Error(),
			)
		}

		practitioner := practitioners[0]

		for _, pr := range roles {
			if pr.Practitioner.Reference != "Practitioner/"+practitioner.ID {
				err := exceptions.BuildNewCustomError(nil, constvars.StatusForbidden, constvars.ErrClientNotAuthorized, "practitioner cannot modify other practitioner's role")
				s.logger.With(zap.Error(err)).Error("ownership check failed")
				return nil, err
			}
		}

		unavailableReason += "Practitioner/" + practitioner.ID
	}

	// Clinic Admin org-scope check (placeholder: ensure all roles share same org)
	if role == constvars.KonsulinRoleClinicAdmin {
		org := roles[0].Organization.Reference
		for _, pr := range roles[1:] {
			if pr.Organization.Reference != org {
				err := exceptions.BuildNewCustomError(nil, constvars.StatusForbidden, constvars.ErrClientNotAuthorized, "cross-organization modification is not allowed")
				s.logger.With(zap.Error(err)).Error("organization scope check failed")
				return nil, err
			}
		}
		// NOTE: actual verification that admin belongs to org is pending integration with user/org mapping

		unavailableReason += "Person/" + uid
	}

	// Build per-role windows and lock targets
	windows := make([]lockWindow, 0, len(roles))
	schedulesByRole := make(map[string]string, len(roles))
	startByRole := make(map[string]time.Time)
	endByRole := make(map[string]time.Time)

	for _, pr := range roles {
		loc, tzErr := pr.GetPreferredTimezone()
		if tzErr != nil {
			s.logger.With(zap.Error(tzErr), zap.String("practitioner_role_id", pr.ID)).Error("failed to resolve timezone")
			return nil, exceptions.BuildNewCustomError(tzErr, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to resolve timezone")
		}

		// Resolve schedule
		scheds, err := s.schedules.FindScheduleByPractitionerRoleID(ctx, pr.ID)
		if err != nil || len(scheds) == 0 {
			s.logger.With(zap.Error(err), zap.String("practitioner_role_id", pr.ID)).Error("failed to resolve schedule")
			return nil, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to resolve schedule")
		}
		if len(scheds) != 1 {
			s.logger.With(zap.Int("count", len(scheds)), zap.String("practitioner_role_id", pr.ID)).Error("unexpected schedules count")
			return nil, exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "unexpected schedules count")
		}
		scheduleID := scheds[0].ID
		var winStart, winEnd time.Time
		if input.AllDay {
			day, err := time.Parse("2006-01-02", input.AllDayDate)
			if err != nil {
				s.logger.With(zap.Error(err)).Error("invalid allDay date format")
				return nil, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "invalid allDay date format")
			}
			// local midnight to next midnight
			dayLocal := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
			winStart = dayLocal
			winEnd = dayLocal.Add(24 * time.Hour)
		} else {
			winStart = input.StartTime
			winEnd = input.EndTime
		}

		windows = append(windows, lockWindow{ScheduleID: scheduleID, Location: loc, Start: winStart, End: winEnd})
		schedulesByRole[pr.ID] = scheduleID
		startByRole[pr.ID] = winStart
		endByRole[pr.ID] = winEnd
	}

	targets := s.dayTargetsForMultiple(windows)
	release, lerr := s.acquireDayLocksOrdered(ctx, targets, 30*time.Second)
	if lerr != nil {
		s.logger.With(zap.Error(lerr)).Error("failed to acquire locks")
		return nil, exceptions.BuildNewCustomError(lerr, constvars.StatusConflict, constvars.ErrClientCannotProcessRequest, "failed to acquire locks")
	}
	defer release(ctx)

	// Conflict detection and idempotency per role
	type createItem struct {
		scheduleID string
		start, end time.Time
	}
	deletions := make([]string, 0)
	creations := make([]createItem, 0)
	updatedRoleBodies := make([]fhir_dto.PractitionerRole, 0)
	allIdempotentSlots := make([]contracts.CreatedSlotItem, 0)
	allIdempotentPRIDs := make([]string, 0)

	out := &contracts.SetUnavailableOutcome{}

	for _, pr := range roles {
		scheduleID := schedulesByRole[pr.ID]
		winStart := startByRole[pr.ID]
		winEnd := endByRole[pr.ID]

		params := contracts.SlotSearchParams{
			Start:  "le" + winEnd.Format(time.RFC3339),
			End:    "ge" + winStart.Format(time.RFC3339),
			Status: "",
		}
		slots, err := s.slots.FindSlotsByScheduleWithQuery(ctx, scheduleID, params)
		if err != nil {
			s.logger.With(zap.Error(err)).Error("failed to find slots for conflict detection")
			return nil, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to find slots for conflict detection")
		}

		// Conflict detection
		conflicts := make([]fhir_dto.Slot, 0)
		var hasIdempotentSlot bool
		for _, sl := range slots {
			if sl.Status == fhir_dto.SlotStatusBusyUnavailable || sl.Status == fhir_dto.SlotStatusBusyTentative {
				conflicts = append(conflicts, sl)
			}
			if sl.Start.Equal(winStart) && sl.End.Equal(winEnd) && sl.Status == input.SlotStatus {
				hasIdempotentSlot = true
				allIdempotentSlots = append(allIdempotentSlots, contracts.CreatedSlotItem{
					ID:     sl.ID,
					Status: string(sl.Status),
				})
				allIdempotentPRIDs = append(allIdempotentPRIDs, pr.ID)
			}
		}

		// Idempotency: PractitionerRole notAvailable contains exact match AND slot with exact window exists and no overlapping free/tentative
		hasNA := false
		for _, na := range pr.NotAvailable {
			if na.Description == unavailableReason && na.During.Start == winStart.Format(time.RFC3339) && na.During.End == winEnd.Format(time.RFC3339) {
				hasNA = true
				break
			}
		}

		if hasNA && hasIdempotentSlot {
			// nothing to do for this role
			continue
		}

		var deletableIDs []string
		for _, sl := range slots {
			if sl.Status == fhir_dto.SlotStatusFree || sl.Status == fhir_dto.SlotStatusBusyTentative {
				// mark deletable if overlaps window
				if sl.End.After(winStart) && sl.Start.Before(winEnd) {
					deletableIDs = append(deletableIDs, sl.ID)
				}
			}
		}

		if len(conflicts) > 0 {
			s.logger.With(zap.Int("conflict_count", len(conflicts))).Warn("conflict detected")
			// build conflict details
			for _, c := range conflicts {
				out.Conflicts = append(out.Conflicts, contracts.ConflictingSlotItem{
					PractitionerRoleID: pr.ID,
					SlotID:             c.ID,
					Start:              c.Start.Format(time.RFC3339),
					End:                c.End.Format(time.RFC3339),
					Status:             string(c.Status),
				})
			}

			return out, exceptions.BuildNewCustomError(nil, constvars.StatusConflict, constvars.ErrClientCannotProcessRequest, "conflict detected with existing booked slots")
		}

		// Prepare deletes and create for this role
		deletions = append(deletions, deletableIDs...)
		creations = append(creations, createItem{scheduleID: scheduleID, start: winStart, end: winEnd})

		// Update PractitionerRole body: prune outdated + add new NA
		if err := pr.RemoveOutdatedNotAvailableReasons(); err != nil {
			s.logger.With(zap.Error(err)).Error("failed to prune notAvailable")
			return out, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to prune notAvailable")
		}

		pr.AddNotAvailable(unavailableReason, winStart, winEnd)
		pr.ResourceType = constvars.ResourcePractitionerRole
		updatedRoleBodies = append(updatedRoleBodies, pr)
	}

	// If after processing nothing to change, return
	if len(deletions) == 0 && len(creations) == 0 && len(updatedRoleBodies) == 0 {
		out.Created = false
		out.CreatedSlots = append(out.CreatedSlots, allIdempotentSlots...)
		out.UpdatedPractitionerIDs = append(out.UpdatedPractitionerIDs, allIdempotentPRIDs...)
		return out, nil
	}

	// Build bundle entries
	entries := make([]map[string]any, 0, len(deletions)+len(creations)+len(updatedRoleBodies))
	for _, id := range deletions {
		if id == "" {
			continue
		}
		entries = append(entries, map[string]any{
			"request": map[string]any{"method": "DELETE", "url": "Slot/" + id},
		})
	}
	for _, c := range creations {
		entries = append(entries, map[string]any{
			"request": map[string]any{"method": "POST", "url": "Slot"},
			"resource": map[string]any{
				"resourceType": "Slot",
				"schedule":     map[string]any{"reference": "Schedule/" + c.scheduleID},
				"status":       string(input.SlotStatus),
				"meta": map[string]any{
					"tag": []map[string]any{{"code": slotTagUserGenerated}},
				},
				"comment": input.Reason,
				"start":   c.start.Format(time.RFC3339),
				"end":     c.end.Format(time.RFC3339),
			},
		})
	}
	for _, rb := range updatedRoleBodies {
		entries = append(entries, map[string]any{
			"request":  map[string]any{"method": "PUT", "url": "PractitionerRole/" + rb.ID},
			"resource": rb,
		})
	}

	bundle := map[string]any{"resourceType": "Bundle", "type": "transaction", "entry": entries}

	if _, err := s.bundles.PostTransactionBundle(ctx, bundle); err != nil {
		s.logger.With(zap.Error(err)).Error("failed to post transaction bundle")
		return out, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to post transaction bundle")
	}

	// fetch created slot IDs for response
	for _, c := range creations {
		got, gerr := s.slots.FindSlotsByScheduleWithQuery(ctx, c.scheduleID, contracts.SlotSearchParams{
			Start:  "ge" + c.start.Format(time.RFC3339),
			End:    "le" + c.end.Format(time.RFC3339),
			Status: input.SlotStatus,
		})
		if gerr == nil && len(got) > 0 {
			out.CreatedSlots = append(out.CreatedSlots, contracts.CreatedSlotItem{ID: got[0].ID, Status: string(input.SlotStatus)})
		} else {
			out.CreatedSlots = append(out.CreatedSlots, contracts.CreatedSlotItem{ID: "", Status: string(input.SlotStatus)})
		}
	}
	for _, rb := range updatedRoleBodies {
		out.UpdatedPractitionerIDs = append(out.UpdatedPractitionerIDs, rb.ID)
	}
	out.Created = len(creations) > 0

	return out, nil
}

// whitelistAccessByRoles will return non-nil error if the requester's role is not whitelisted.
// the value supplied in whiteListed will be checked against the requester's role and it should be
// the defined enum of known roles as defined in consvars package. for now, the function signature
// will accept string slices, but in the future it should be refactored to used custom typed string
// instead.
func (s *SlotUsecase) whitelistAccessByRoles(ctx context.Context, whiteListed []string) (string, string, error) {
	roles, _ := ctx.Value(constvars.CONTEXT_FHIR_ROLE).([]string)
	uid, _ := ctx.Value(constvars.CONTEXT_UID).(string)

	for _, role := range roles {
		if slices.Contains(whiteListed, role) {
			return role, uid, nil
		}
	}

	return "", "", errors.New("current role is not permitted to access")
}

func (s *SlotUsecase) HandleAutomatedSlotGeneration(ctx context.Context, practitionerRole fhir_dto.PractitionerRole) {
	now := time.Now()
	logger := s.logger.With(
		zap.String("method", "HandleAutomatedSlotGeneration"),
		zap.String("practitioner_role_id", practitionerRole.ID),
		zap.Time("now", now),
	)

	logger.Info("starting automated slot generation")

	scheds, err := s.schedules.FindScheduleByPractitionerRoleID(ctx, practitionerRole.ID)
	if err != nil {
		logger.Error("failed to find schedules", zap.Error(err))
		return
	}

	if len(scheds) != 1 {
		logger.Error("expected 1 schedule, but got", zap.Int("count", len(scheds)))
		return
	}

	loc, tzErr := practitionerRole.GetPreferredTimezone()
	if tzErr != nil {
		logger.Error("failed to resolve role timezone", zap.Error(tzErr))
		return
	}

	logger = logger.With(zap.String("timezone", loc.String()))

	plan, err := ConvertAvailableTimeToWeeklyPlan(practitionerRole.AvailableTime)
	if err != nil {
		logger.Error("failed to convert available time to weekly plan", zap.Error(err))
		return
	}

	schedule := scheds[0]

	logger = logger.With(
		zap.String("schedule_id", schedule.ID),
		zap.String("schedule_comment", schedule.Comment),
		zap.Any("plan", plan),
	)

	cfg, err := ParseScheduleConfig(schedule.Comment)
	if err != nil {
		return
	}

	logger = logger.With(zap.Any("schedule config", cfg))

	windowDays := s.config.App.SlotWindowDays
	today := time.Date(
		now.In(loc).Year(),
		now.In(loc).Month(),
		now.In(loc).Day(),
		0, 0, 0, 0,
		loc,
	)

	end := today.AddDate(0, 0, windowDays-1)

	logger = logger.With(
		zap.Time("today", today),
		zap.Time("end", end),
		zap.Int("window_days", windowDays),
	)

	for d := today; !d.After(end); d = d.AddDate(0, 0, 1) {
		logger.With(zap.String("day", d.String())).Info("processing day")

		windows := plan.forWeekday(d.Weekday())
		if len(windows) == 0 {
			logger.With(zap.String("day", d.String())).Info("no windows for day")
			continue
		}

		acq, key, tok, err := s.tryAcquireDayLock(
			ctx,
			schedule.ID,
			d,
			loc.String(),
			30*time.Second,
		)
		if err != nil || !acq {
			logger.Error("failed to acquire day lock", zap.Error(err))
			continue
		}

		err = s.topUpRoleForDay(
			ctx,
			topUpRoleForDayInput{
				ScheduleID:    schedule.ID,
				Timezone:      loc,
				Day:           d,
				Plan:          windows,
				SlotMinutes:   cfg.SlotMinutes,
				BufferMinutes: cfg.BufferMinutes,
			},
			logger,
		)
		if err != nil {
			logger.With(zap.Error(err)).Error("failed to top up role for day")
			continue
		}

		err = s.releaseLock(ctx, key, tok)
		if err != nil {
			logger.With(zap.Error(err)).Error("failed to release day lock")
			continue
		}
	}
}

// tryAcquireDayLock acquires a per-day lock for a schedule and local day.
// tzName should be the IANA timezone string used when computing the local day boundaries.
func (s *SlotUsecase) tryAcquireDayLock(ctx context.Context, scheduleID string, day time.Time, tzName string, ttl time.Duration) (acquired bool, key string, token string, err error) {
	k := s.dayLockKey(scheduleID, day, tzName)
	ok, tok, err := s.locker.TryLock(ctx, k, ttl)
	if err != nil {
		return false, k, "", err
	}

	return ok, k, tok, nil
}

// ReleaseLock releases a lock by key and token.
func (s *SlotUsecase) releaseLock(ctx context.Context, key, token string) error {
	if key == "" || token == "" {
		return nil
	}
	return s.locker.Unlock(ctx, key, token)
}

// dayLockKey builds the per-(schedule, local-day) lock key. The day is formatted as YYYY-MM-DD in the given timezone name.
func (s *SlotUsecase) dayLockKey(scheduleID string, day time.Time, tzName string) string {
	y, m, d := day.Date()
	prefix := "slotgen:lock:day"
	return fmt.Sprintf("%s:%s:%04d-%02d-%02d:%s", prefix, scheduleID, y, int(m), d, tzName)
}

type topUpRoleForDayInput struct {
	ScheduleID    string
	Timezone      *time.Location
	Day           time.Time
	Plan          []dayWindow
	SlotMinutes   int
	BufferMinutes int
}

// topUpRoleForDay performs per-day generation/override under caller-provided locking.
func (s *SlotUsecase) topUpRoleForDay(ctx context.Context, in topUpRoleForDayInput, log ...*zap.Logger) error {
	logger := zap.L()
	if len(log) > 0 {
		logger = log[0]
	}

	logger = logger.With(
		zap.String("schedule_id", in.ScheduleID),
		zap.String("timezone", in.Timezone.String()),
		zap.Time("day", in.Day),
		zap.Any("plan", in.Plan),
		zap.Int("slot_minutes", in.SlotMinutes),
		zap.Int("buffer_minutes", in.BufferMinutes),
		zap.String("function", "topUpRoleForDay"),
	)

	logger.Info("topping up role for day")

	// Compute local day bounds strings
	dayLocal := in.Day.In(in.Timezone)
	dayStart := atClock(dayLocal, 0, 0, in.Timezone)
	dayEnd := dayStart.AddDate(0, 0, 1)
	params := contracts.SlotSearchParams{
		Start:  "lt" + dayEnd.Format(time.RFC3339),
		End:    "gt" + dayStart.Format(time.RFC3339),
		Status: "", // include all
	}

	logger = logger.With(zap.Any("FindSlotsByScheduleWithQuery params", params))

	existingSlots, err := s.slots.FindSlotsByScheduleWithQuery(ctx, in.ScheduleID, params)
	if err != nil {
		logger.With(zap.Error(err)).Error("failed to find slots by schedule with query")
		return err
	}

	// Compute expected intervals up-front and classify coverage against them
	incomingSlotsIntervals := generateSlotsForDayWindows(dayLocal, in.Timezone, in.Plan, in.SlotMinutes, in.BufferMinutes)

	cov, toDelete := classifyDayCoverageFromSlots(existingSlots, incomingSlotsIntervals)
	switch cov {
	case coverageNone:
		logger.Info("coverage is none, generating new slots")
		fhirSlots := buildFHIRSlots(in.ScheduleID, incomingSlotsIntervals, fhir_dto.SlotStatusFree)
		bundle := buildCreateSlotsTransactionBundle(in.ScheduleID, fhirSlots)
		_, err := s.slots.PostTransactionBundle(ctx, bundle)
		return err
	case coverageAllFreeNonAuto:
		logger.Info("coverage is all free non-auto or mismatch with expected, deleting and generating new slots")
		fhirSlots := buildFHIRSlots(in.ScheduleID, incomingSlotsIntervals, fhir_dto.SlotStatusFree)
		bundle := buildOverrideSlotsTransactionBundle(in.ScheduleID, toDelete, fhirSlots)
		_, err := s.slots.PostTransactionBundle(ctx, bundle)
		return err
	case coverageAllFreeAuto:
		logger.Info("coverage is all free autogenerated and matches. no action needed")
		return nil
	case coverageConflict:
		logger.Info("coverage is conflict, handling slot conflict")
		return s.handleSlotConflict(ctx, in, existingSlots, logger)
	default:
		return fmt.Errorf("unknown coverage state")
	}
}

func (s *SlotUsecase) handleSlotConflict(
	ctx context.Context,
	in topUpRoleForDayInput,
	existingSlots []fhir_dto.Slot,
	log *zap.Logger,
) error {
	// 1) Build base day work windows and adjust by non-free blocks with post-gap buffer rule
	baseWindows := dayWorkIntervals(in.Day.In(in.Timezone), in.Timezone, in.Plan)
	adjusted := adjustIncomingSlotIntervalOnConflict(baseWindows, existingSlots, in.SlotMinutes, in.BufferMinutes)
	if len(adjusted) == 0 {
		log.Info("conflict: no adjusted intervals; nothing to create")
		return nil
	}

	// 2) Gather existing free slots and their intervals
	var existingFree []fhir_dto.Slot
	for _, slt := range existingSlots {
		if slt.Status == fhir_dto.SlotStatusFree {
			existingFree = append(existingFree, slt)
		}
	}
	existingFreeIntervals := intervalsFromSlots(existingFree)

	// 3) If already equal after adjustment, nothing to do
	if isIntervalsMatch(adjusted, existingFreeIntervals) {
		log.Info("conflict: adjusted intervals already satisfied; no action")
		return nil
	}

	// 4) Delete ALL free slots not present in adjusted (auto or non-auto)
	adjustedSet := make(map[string]struct{}, len(adjusted))
	for _, iv := range adjusted {
		adjustedSet[intervalKey(iv.Start, iv.End)] = struct{}{}
	}
	var deleteIDs []string
	for _, slt := range existingFree {
		if slt.ID == "" {
			continue
		}
		k := intervalKey(slt.Start, slt.End)
		if _, ok := adjustedSet[k]; !ok {
			deleteIDs = append(deleteIDs, slt.ID)
		}
	}

	// 5) Missing intervals to create: adjusted \ existingFreeIntervals
	missing := differenceByIntervalKey(adjusted, existingFreeIntervals)
	if len(missing) == 0 && len(deleteIDs) == 0 {
		log.Info("conflict: no missing or deletable slots; no action")
		return nil
	}

	// 6) Build and post bundle: delete extras (any free), create missing adjusted
	createSlots := buildFHIRSlots(in.ScheduleID, missing, fhir_dto.SlotStatusFree)
	if len(deleteIDs) > 0 {
		log.Info("conflict: overriding free slots to match adjusted intervals",
			zap.Int("delete_count", len(deleteIDs)),
			zap.Int("create_count", len(createSlots)),
		)
		bundle := buildOverrideSlotsTransactionBundle(in.ScheduleID, deleteIDs, createSlots)
		_, err := s.slots.PostTransactionBundle(ctx, bundle)
		return err
	}

	log.Info("conflict: creating missing adjusted intervals (no deletes)",
		zap.Int("create_count", len(createSlots)),
	)
	bundle := buildCreateSlotsTransactionBundle(in.ScheduleID, createSlots)
	_, err := s.slots.PostTransactionBundle(ctx, bundle)
	return err
}

// Internal locking helpers (kept unexported in separate file to keep usecase file focused)
type dayLockTarget struct {
	ScheduleID string
	Day        time.Time
	Location   *time.Location
}

type lockWindow struct {
	ScheduleID string
	Location   *time.Location
	Start      time.Time
	End        time.Time
}

// dayTargetsForWindow computes local days (inclusive) covered by [start,end) in the given location
func (s *SlotUsecase) dayTargetsForWindow(scheduleID string, loc *time.Location, start, end time.Time) []dayLockTarget {
	ls := start.In(loc)
	le := end.In(loc)
	day := time.Date(ls.Year(), ls.Month(), ls.Day(), 0, 0, 0, 0, loc)
	last := time.Date(le.Year(), le.Month(), le.Day(), 0, 0, 0, 0, loc)
	var out []dayLockTarget
	for d := day; !d.After(last); d = d.AddDate(0, 0, 1) {
		out = append(out, dayLockTarget{ScheduleID: scheduleID, Day: d, Location: loc})
	}
	return out
}

// dayTargetsForMultiple aggregates and returns sorted unique targets across windows
func (s *SlotUsecase) dayTargetsForMultiple(windows []lockWindow) []dayLockTarget {
	seen := make(map[string]struct{})
	var out []dayLockTarget
	for _, w := range windows {
		ts := s.dayTargetsForWindow(w.ScheduleID, w.Location, w.Start, w.End)
		for _, t := range ts {
			key := fmt.Sprintf("%s|%04d-%02d-%02d|%s", t.ScheduleID, t.Day.Year(), int(t.Day.Month()), t.Day.Day(), t.Location.String())
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ScheduleID != out[j].ScheduleID {
			return out[i].ScheduleID < out[j].ScheduleID
		}
		if !out[i].Day.Equal(out[j].Day) {
			return out[i].Day.Before(out[j].Day)
		}
		return out[i].Location.String() < out[j].Location.String()
	})
	return out
}

// acquireDayLocksOrdered acquires locks in deterministic order and returns a release closure
func (s *SlotUsecase) acquireDayLocksOrdered(ctx context.Context, targets []dayLockTarget, ttl time.Duration) (func(context.Context), error) {
	type acquired struct{ key, tok string }
	acquiredList := make([]acquired, 0, len(targets))
	for _, t := range targets {
		key := s.dayLockKey(t.ScheduleID, t.Day, t.Location.String())
		ok, tok, err := s.locker.TryLock(ctx, key, ttl)
		if err != nil || !ok {
			for i := len(acquiredList) - 1; i >= 0; i-- {
				_ = s.locker.Unlock(ctx, acquiredList[i].key, acquiredList[i].tok)
			}
			if err == nil {
				err = fmt.Errorf("failed to acquire lock: %s", key)
			}
			return func(context.Context) {}, err
		}
		acquiredList = append(acquiredList, acquired{key: key, tok: tok})
	}
	release := func(ctx context.Context) {
		for i := len(acquiredList) - 1; i >= 0; i-- {
			_ = s.locker.Unlock(ctx, acquiredList[i].key, acquiredList[i].tok)
		}
	}
	return release, nil
}
