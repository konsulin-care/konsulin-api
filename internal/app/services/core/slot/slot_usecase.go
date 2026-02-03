package slot

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"sort"
	"strings"
	"time"

	bundleSvc "konsulin-service/internal/app/services/fhir_spark/bundle"
	"slices"

	"go.uber.org/zap"
)

type SlotUsecase struct {
	schedules         contracts.ScheduleFhirClient
	locker            contracts.LockerService
	slots             contracts.SlotFhirClient
	practitionerRoles contracts.PractitionerRoleFhirClient
	practitioner      contracts.PractitionerFhirClient
	person            contracts.PersonFhirClient
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
	person contracts.PersonFhirClient,
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
		person:            person,
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
		// Resolve current clinic admin by Person identifier (system|value)
		identifierToken := fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, uid)
		people, perr := s.person.Search(ctx, contracts.PersonSearchInput{Identifier: identifierToken})
		if perr != nil {
			return nil, exceptions.BuildNewCustomError(
				perr,
				constvars.StatusInternalServerError,
				perr.Error(),
				perr.Error(),
			)
		}
		if len(people) != 1 {
			errMultiPersons := errors.New("multiple persons found on the same identifier or no person found at all")
			return nil, exceptions.BuildNewCustomError(
				errMultiPersons,
				constvars.StatusBadRequest,
				errMultiPersons.Error(),
				errMultiPersons.Error(),
			)
		}
		adminPerson := people[0]
		adminOrgRef := ""
		if adminPerson.ManagingOrganization != nil {
			adminOrgRef = adminPerson.ManagingOrganization.Reference
		}
		if adminOrgRef == "" {
			err := exceptions.BuildNewCustomError(nil, constvars.StatusForbidden, constvars.ErrClientNotAuthorized, "clinic admin has no managingOrganization configured")
			s.logger.With(zap.Error(err)).Error("organization scope check failed: missing managingOrganization on admin")
			return nil, err
		}

		// Ensure all target roles belong to the same managing organization as the admin
		for _, pr := range roles {
			if pr.Organization.Reference != adminOrgRef {
				err := exceptions.BuildNewCustomError(nil, constvars.StatusForbidden, constvars.ErrClientNotAuthorized, "clinic admin cannot modify roles from other organization")
				s.logger.With(zap.Error(err)).Error("organization scope check failed")
				return nil, err
			}
		}

		unavailableReason += "Person/" + uid
	}

	// Build per-role windows and lock targets
	windows := make([]lockWindow, 0, len(roles))
	schedulesByRole := make(map[string]string, len(roles))
	// store schedule comment by role for cfg parsing later
	scheduleCommentByRole := make(map[string]string, len(roles))
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
		// keep schedule comment for later per-day adjustment
		scheduleCommentByRole[pr.ID] = scheds[0].Comment
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
	// free slot creations needed after adjustment (system-generated)
	type freeCreateItem struct {
		scheduleID string
		start, end time.Time
	}
	deletions := make([]string, 0)
	creations := make([]createItem, 0)
	createFree := make([]freeCreateItem, 0)
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

		// Additional per-day adjustment to align surrounding free slots using existing rules
		// 1) Parse schedule config and weekly plan
		cfg, cfgErr := ParseScheduleConfig(scheduleCommentByRole[pr.ID])
		if cfgErr != nil {
			s.logger.With(zap.Error(cfgErr)).Error("failed to parse schedule config for adjustment")
			return out, exceptions.BuildNewCustomError(cfgErr, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to parse schedule config")
		}
		plan, planErr := ConvertAvailableTimeToWeeklyPlan(pr.AvailableTime)
		if planErr != nil {
			s.logger.With(zap.Error(planErr)).Error("failed to convert available time to weekly plan for adjustment")
			return out, exceptions.BuildNewCustomError(planErr, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to build weekly plan")
		}

		// 2) Determine affected local day(s) for the requested window
		// the error is not handled here because it is already checked in the previous step
		// thus also no need to have a fallback to time.Local
		roleLoc, _ := pr.GetPreferredTimezone()
		targetDays := s.dayTargetsForWindow(scheduleID, roleLoc, winStart, winEnd)

		for _, td := range targetDays {
			day := td.Day
			loc := td.Location
			windowsForDay := plan.forWeekday(day.Weekday())
			if len(windowsForDay) == 0 {
				// No configured windows for this day; do not adjust unrelated free slots
				continue
			}
			dayStart := atClock(day, 0, 0, loc)
			dayEnd := dayStart.Add(24 * time.Hour)
			params := contracts.SlotSearchParams{
				Start:  "lt" + dayEnd.Format(time.RFC3339),
				End:    "gt" + dayStart.Format(time.RFC3339),
				Status: "",
			}
			daySlots, qErr := s.slots.FindSlotsByScheduleWithQuery(ctx, scheduleID, params)
			if qErr != nil {
				s.logger.With(zap.Error(qErr)).Error("failed to fetch day slots for adjustment")
				return out, exceptions.BuildNewCustomError(qErr, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to fetch day slots for adjustment")
			}

			// Inject the new busy/unavailable block into the day context (clipped to the day)
			clipStart := winStart
			if clipStart.Before(dayStart) {
				clipStart = dayStart
			}
			clipEnd := winEnd
			if clipEnd.After(dayEnd) {
				clipEnd = dayEnd
			}
			var existingWithPseudo []fhir_dto.Slot
			existingWithPseudo = append(existingWithPseudo, daySlots...)
			if clipEnd.After(clipStart) {
				existingWithPseudo = append(existingWithPseudo, fhir_dto.Slot{
					Status: input.SlotStatus,
					Start:  clipStart,
					End:    clipEnd,
				})
			}

			// Build base working windows on this day and compute adjusted free intervals
			base := dayWorkIntervals(day.In(loc), loc, windowsForDay)
			adjusted := adjustIncomingSlotIntervalOnConflict(base, existingWithPseudo, cfg.SlotMinutes, cfg.BufferMinutes)
			if len(adjusted) == 0 {
				continue
			}

			// Compare with existing FREE slots for the day
			var existingFree []fhir_dto.Slot
			for _, slt := range daySlots {
				if slt.Status == fhir_dto.SlotStatusFree {
					existingFree = append(existingFree, slt)
				}
			}
			existingFreeIntervals := intervalsFromSlots(existingFree)

			toDeleteIntervals := differenceByIntervalKey(existingFreeIntervals, adjusted)
			if len(toDeleteIntervals) > 0 {
				// map intervals to IDs
				want := make(map[string]struct{}, len(toDeleteIntervals))
				for _, iv := range toDeleteIntervals {
					want[intervalKey(iv.Start, iv.End)] = struct{}{}
				}
				for _, slt := range existingFree {
					k := intervalKey(slt.Start, slt.End)
					if _, ok := want[k]; ok {
						if slt.ID != "" {
							deletions = append(deletions, slt.ID)
						}
					}
				}
			}

			toCreateIntervals := differenceByIntervalKey(adjusted, existingFreeIntervals)
			for _, iv := range toCreateIntervals {
				createFree = append(createFree, freeCreateItem{scheduleID: scheduleID, start: iv.Start, end: iv.End})
			}
		}

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
	// If after processing nothing to change, return
	if len(deletions) == 0 && len(creations) == 0 && len(updatedRoleBodies) == 0 && len(createFree) == 0 {
		out.Created = false
		out.CreatedSlots = append(out.CreatedSlots, allIdempotentSlots...)
		out.UpdatedPractitionerIDs = append(out.UpdatedPractitionerIDs, allIdempotentPRIDs...)
		return out, nil
	}

	// Build bundle entries
	entries := make([]map[string]any, 0, len(deletions)+len(creations)+len(updatedRoleBodies))
	// de-duplicate deletions
	seenDel := make(map[string]struct{}, len(deletions))
	for _, id := range deletions {
		if id == "" {
			continue
		}
		if _, ok := seenDel[id]; ok {
			continue
		}
		seenDel[id] = struct{}{}
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
	// Add free slot creations after adjustment (system-generated)
	for _, fc := range createFree {
		entries = append(entries, map[string]any{
			"request": map[string]any{"method": "POST", "url": "Slot"},
			"resource": map[string]any{
				"resourceType": "Slot",
				"schedule":     map[string]any{"reference": "Schedule/" + fc.scheduleID},
				"status":       string(fhir_dto.SlotStatusFree),
				"start":        fc.start.Format(time.RFC3339),
				"end":          fc.end.Format(time.RFC3339),
				"meta": map[string]any{
					"tag": []map[string]any{{"code": SlotTagSystemGenerated}},
				},
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

// onDemandLockTTL is the TTL for day locks during on-demand regeneration (single bundle, long run).
const onDemandLockTTL = 5 * time.Minute

// HandleOnDemandSlotRegeneration regenerates slots for one practitioner role from today to end of
// config window: covered days get generated/updated (same rules as automated), uncovered days have
// auto-generated free slots erased. Locks all days in the window upfront, then posts one FHIR transaction.
func (s *SlotUsecase) HandleOnDemandSlotRegeneration(ctx context.Context, practitionerRoleID string) error {
	logger := s.logger.With(
		zap.String("method", "HandleOnDemandSlotRegeneration"),
		zap.String("practitioner_role_id", practitionerRoleID),
	)

	pr, err := s.practitionerRoles.FindPractitionerRoleByID(ctx, practitionerRoleID)
	if err != nil {
		logger.Debug("failed to find practitioner role", zap.Error(err))
		return err
	}
	if pr == nil {
		logger.Debug("practitioner role not found")
		return nil
	}
	if !pr.Active {
		logger.Debug("practitioner role not active, skipping")
		return nil
	}

	scheds, err := s.schedules.FindScheduleByPractitionerRoleID(ctx, pr.ID)
	if err != nil {
		logger.Error("failed to find schedule", zap.Error(err))
		return err
	}
	if len(scheds) != 1 {
		logger.Error("expected 1 schedule", zap.Int("count", len(scheds)))
		return fmt.Errorf("expected 1 schedule for role %s, got %d", practitionerRoleID, len(scheds))
	}

	loc, tzErr := pr.GetPreferredTimezone()
	if tzErr != nil {
		logger.Error("failed to resolve timezone", zap.Error(tzErr))
		return tzErr
	}

	plan, err := ConvertAvailableTimeToWeeklyPlan(pr.AvailableTime)
	if err != nil {
		logger.Error("failed to convert available time", zap.Error(err))
		return err
	}

	schedule := scheds[0]
	cfg, err := ParseScheduleConfig(schedule.Comment)
	if err != nil {
		logger.Error("failed to parse schedule config", zap.Error(err))
		return err
	}

	now := time.Now()
	windowDays := s.config.App.SlotWindowDays
	today := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	end := today.AddDate(0, 0, windowDays-1)

	// Lock all days in [today, end] upfront (inclusive).
	windowEndExclusive := end.AddDate(0, 0, 1)
	targets := s.dayTargetsForWindow(schedule.ID, loc, today, windowEndExclusive)
	release, err := s.acquireDayLocksOrdered(ctx, targets, onDemandLockTTL)
	if err != nil {
		logger.Error("failed to acquire day locks", zap.Error(err))
		return err
	}
	defer release(ctx)

	var allDeleteIDs []string
	var allCreateSlots []fhir_dto.Slot

	for d := today; !d.After(end); d = d.AddDate(0, 0, 1) {
		windows := plan.forWeekday(d.Weekday())
		dayLocal := d.In(loc)
		dayStart := atClock(dayLocal, 0, 0, loc)
		dayEnd := dayStart.AddDate(0, 0, 1)
		params := contracts.SlotSearchParams{
			Start:  "lt" + dayEnd.Format(time.RFC3339),
			End:    "gt" + dayStart.Format(time.RFC3339),
			Status: "",
		}

		existingSlots, err := s.slots.FindSlotsByScheduleWithQuery(ctx, schedule.ID, params)
		if err != nil {
			logger.Error("failed to find slots for day", zap.Time("day", d), zap.Error(err))
			return err
		}

		if len(windows) == 0 {
			// Uncovered: erase auto-generated free slots only.
			ids := idsOfAutoGeneratedFreeSlots(existingSlots)
			allDeleteIDs = append(allDeleteIDs, ids...)
			continue
		}

		incomingSlotsIntervals := generateSlotsForDayWindows(dayLocal, loc, windows, cfg.SlotMinutes, cfg.BufferMinutes)
		cov, toDelete := classifyDayCoverageFromSlots(existingSlots, incomingSlotsIntervals)

		switch cov {
		case coverageNone:
			allCreateSlots = append(allCreateSlots, buildFHIRSlots(schedule.ID, incomingSlotsIntervals, fhir_dto.SlotStatusFree)...)
		case coverageAllFreeNonAuto:
			allDeleteIDs = append(allDeleteIDs, toDelete...)
			allCreateSlots = append(allCreateSlots, buildFHIRSlots(schedule.ID, incomingSlotsIntervals, fhir_dto.SlotStatusFree)...)
		case coverageAllFreeAuto:
			// no change
		case coverageConflict:
			baseWindows := dayWorkIntervals(dayLocal, loc, windows)
			adjusted := adjustIncomingSlotIntervalOnConflict(baseWindows, existingSlots, cfg.SlotMinutes, cfg.BufferMinutes)
			if len(adjusted) == 0 {
				continue
			}
			var existingFree []fhir_dto.Slot
			for _, slt := range existingSlots {
				if slt.Status == fhir_dto.SlotStatusFree {
					existingFree = append(existingFree, slt)
				}
			}
			existingFreeIntervals := intervalsFromSlots(existingFree)
			if isIntervalsMatch(adjusted, existingFreeIntervals) {
				continue
			}
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
			missing := differenceByIntervalKey(adjusted, existingFreeIntervals)
			if len(missing) == 0 && len(deleteIDs) == 0 {
				continue
			}
			createSlots := buildFHIRSlots(schedule.ID, missing, fhir_dto.SlotStatusFree)
			allDeleteIDs = append(allDeleteIDs, deleteIDs...)
			allCreateSlots = append(allCreateSlots, createSlots...)
		default:
			return fmt.Errorf("unknown coverage state")
		}
	}

	if len(allDeleteIDs) == 0 && len(allCreateSlots) == 0 {
		return nil
	}

	var bundle map[string]any
	if len(allDeleteIDs) > 0 {
		bundle = buildOverrideSlotsTransactionBundle(schedule.ID, allDeleteIDs, allCreateSlots)
	} else {
		bundle = buildCreateSlotsTransactionBundle(schedule.ID, allCreateSlots)
	}

	// simple retry logic for post bundle that will
	// retry the process if the error is retryable
	const postBundleMaxAttempts = 3
	const postBundleRetryDelay = 100 * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < postBundleMaxAttempts; attempt++ {
		_, lastErr = s.slots.PostTransactionBundle(ctx, bundle)
		if lastErr == nil {
			return nil
		}
		if !exceptions.IsHTTPErrRetryable(lastErr) || attempt == postBundleMaxAttempts-1 {
			return lastErr
		}
		time.Sleep(postBundleRetryDelay)
	}
	return lastErr
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
	// If the window is empty or inverted, no days are covered.
	if !end.After(start) {
		return nil
	}

	ls := start.In(loc)
	le := end.In(loc)
	day := time.Date(ls.Year(), ls.Month(), ls.Day(), 0, 0, 0, 0, loc)
	// Treat end as exclusive by subtracting a minimal delta before computing the last day.
	// This ensures that an end exactly at midnight does not include the following calendar day.
	leExclusive := le.Add(-time.Nanosecond)
	last := time.Date(leExclusive.Year(), leExclusive.Month(), leExclusive.Day(), 0, 0, 0, 0, loc)

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

// AcquireLocksForAppointment acquires locks for all affected schedule-day pairs
// when booking an appointment. This prevents race conditions across practitioner roles.
func (s *SlotUsecase) AcquireLocksForAppointment(
	ctx context.Context,
	practitionerRoles []fhir_dto.PractitionerRole,
	appointmentStart, appointmentEnd time.Time,
	ttl time.Duration,
) (func(context.Context), error) {
	// Build day lock targets for all roles
	var allTargets []dayLockTarget
	for _, role := range practitionerRoles {
		loc, err := role.GetPreferredTimezone()
		if err != nil {
			return func(context.Context) {}, fmt.Errorf("failed to get timezone for role %s: %w", role.ID, err)
		}

		// Find schedule for this role
		scheds, err := s.schedules.FindScheduleByPractitionerRoleID(ctx, role.ID)
		if err != nil || len(scheds) == 0 {
			return func(context.Context) {}, fmt.Errorf("failed to find schedule for role %s: %w", role.ID, err)
		}
		if len(scheds) != 1 {
			return func(context.Context) {}, fmt.Errorf("unexpected schedule count %d for role %s", len(scheds), role.ID)
		}
		scheduleID := scheds[0].ID

		// Compute affected days
		targets := s.dayTargetsForWindow(scheduleID, loc, appointmentStart, appointmentEnd)
		allTargets = append(allTargets, targets...)
	}

	// Deduplicate and sort targets
	seen := make(map[string]struct{})
	var uniqueTargets []dayLockTarget
	for _, t := range allTargets {
		key := fmt.Sprintf("%s|%04d-%02d-%02d|%s", t.ScheduleID, t.Day.Year(), int(t.Day.Month()), t.Day.Day(), t.Location.String())
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			uniqueTargets = append(uniqueTargets, t)
		}
	}

	// Sort for deterministic ordering
	sort.SliceStable(uniqueTargets, func(i, j int) bool {
		if uniqueTargets[i].ScheduleID != uniqueTargets[j].ScheduleID {
			return uniqueTargets[i].ScheduleID < uniqueTargets[j].ScheduleID
		}
		if !uniqueTargets[i].Day.Equal(uniqueTargets[j].Day) {
			return uniqueTargets[i].Day.Before(uniqueTargets[j].Day)
		}
		return uniqueTargets[i].Location.String() < uniqueTargets[j].Location.String()
	})

	// Acquire locks in order
	return s.acquireDayLocksOrdered(ctx, uniqueTargets, ttl)
}

// AcquireLocksForSlot acquires locks for all affected schedule-day pairs
// for a given slot. This prevents race conditions when mutating slot resources.
// The function extracts the schedule ID and timezone from the slot automatically.
func (s *SlotUsecase) AcquireLocksForSlot(
	ctx context.Context,
	slot *fhir_dto.Slot,
	ttl time.Duration,
) (func(context.Context), error) {
	if slot == nil {
		return func(context.Context) {}, fmt.Errorf("slot cannot be nil")
	}

	// Extract schedule ID from Schedule.Reference (e.g., "Schedule/123" -> "123")
	scheduleRef := slot.Schedule.Reference
	if scheduleRef == "" {
		return func(context.Context) {}, fmt.Errorf("slot has no schedule reference")
	}

	scheduleID := strings.TrimPrefix(scheduleRef, "Schedule/")
	if scheduleID == "" {
		return func(context.Context) {}, fmt.Errorf("invalid schedule reference format: %s", scheduleRef)
	}

	// Extract timezone from slot start time
	if slot.Start.IsZero() {
		return func(context.Context) {}, fmt.Errorf("slot start time is zero")
	}
	loc := slot.Start.Location()
	if loc == nil {
		return func(context.Context) {}, fmt.Errorf("slot start time has no location/timezone")
	}

	// Compute affected days for the slot's time window
	targets := s.dayTargetsForWindow(scheduleID, loc, slot.Start, slot.End)
	if len(targets) == 0 {
		return func(context.Context) {}, fmt.Errorf("no day targets computed for slot time window")
	}

	// Acquire locks in order
	return s.acquireDayLocksOrdered(ctx, targets, ttl)
}
