package slot

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/fhir_dto"
	"time"

	"go.uber.org/zap"
)

type SlotUsecase struct {
	schedules contracts.ScheduleFhirClient
	locker    contracts.LockerService
	slots     contracts.SlotFhirClient
	config    *config.InternalConfig
	logger    *zap.Logger
}

func NewSlotUsecase(
	schedules contracts.ScheduleFhirClient,
	locker contracts.LockerService,
	slots contracts.SlotFhirClient,
	config *config.InternalConfig,
	logger *zap.Logger,
) *SlotUsecase {
	return &SlotUsecase{
		schedules: schedules,
		locker:    locker,
		slots:     slots,
		config:    config,
		logger:    logger,
	}
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

	loc, tzErr := s.resolveRoleTimezone(practitionerRole)
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
