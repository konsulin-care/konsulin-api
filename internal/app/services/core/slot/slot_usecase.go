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
	"slices"
	"sort"
	"strings"
	"time"

	bundleSvc "konsulin-service/internal/app/services/fhir_spark/bundle"

	"go.uber.org/zap"
)

// errMultiplePractitionersFound is returned when the supertoken identifier
// resolves to zero or several Practitioner resources during role ownership
// checks.
const errMultiplePractitionersFound = "multiple practitioners found on the same identifier or no practitioner found at all"

// practitionerRefPrefix builds FHIR references of the form Practitioner/<id>.
const practitionerRefPrefix = "Practitioner/"

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

	roles, err := s.loadPractitionerRoles(ctx, input.PractitionerRoleIDs)
	if err != nil {
		return nil, err
	}

	roleOwnerRef, err := s.checkRoleOwnership(ctx, role, uid, roles)
	if err != nil {
		return nil, err
	}
	unavailableReason := "Manual unavailability indicated by " + roleOwnerRef

	resolved, err := s.resolveAndLockWindows(ctx, roles, input)
	if err != nil {
		return nil, err
	}
	defer func() { resolved.release(context.Background()) }()

	state := &idempotentState{
		Slots: &[]contracts.CreatedSlotItem{},
		PRIDs: &[]string{},
	}
	out, err := s.processRoleWindows(ctx, roles, resolved, input, unavailableReason, state)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// createItem holds schedule and time window for a slot creation.
type createItem struct {
	scheduleID string
	start, end time.Time
}

// idempotentState tracks created slot items and practitioner role IDs for idempotency.
type idempotentState struct {
	Slots *[]contracts.CreatedSlotItem
	PRIDs *[]string
}

// resolvedWindows holds per-role schedule data after resolution and lock acquisition.
type resolvedWindows struct {
	schedulesByRole map[string]string
	startByRole     map[string]time.Time
	endByRole       map[string]time.Time
	release         func(context.Context)
}

// resolveAndLockWindows resolves schedules for all roles, builds lock targets, and acquires locks.
func (s *SlotUsecase) resolveAndLockWindows(ctx context.Context, roles []fhir_dto.PractitionerRole, input contracts.SetUnavailabilityForMultiplePractitionerRolesInput) (*resolvedWindows, error) {
	res := &resolvedWindows{
		schedulesByRole: make(map[string]string, len(roles)),
		startByRole:     make(map[string]time.Time),
		endByRole:       make(map[string]time.Time),
	}

	// All roles belong to one practitioner (ownership is validated by the caller);
	// derive the practitioner-scoped day lock from the first role.
	var practitionerID string
	if len(roles) > 0 {
		practitionerID = strings.TrimPrefix(roles[0].Practitioner.Reference, practitionerRefPrefix)
	}

	seen := make(map[string]struct{})
	var targets []practitionerDayLockTarget
	for _, pr := range roles {
		scheduleID, winStart, winEnd, loc, err := s.resolveRoleScheduleWindow(ctx, pr, input)
		if err != nil {
			return nil, err
		}

		for _, t := range s.practitionerDayTargetsForWindow(practitionerID, loc, winStart, winEnd) {
			targets = dedupePractitionerDayTargets(seen, targets, t)
		}
		res.schedulesByRole[pr.ID] = scheduleID
		res.startByRole[pr.ID] = winStart
		res.endByRole[pr.ID] = winEnd
	}

	sortPractitionerDayTargets(targets)
	releaseFn, lerr := s.acquirePractitionerDayLocksOrdered(ctx, targets, 30*time.Second)
	if lerr != nil {
		s.logger.With(zap.Error(lerr)).Error("failed to acquire locks")
		return nil, exceptions.BuildNewCustomError(lerr, constvars.StatusConflict, constvars.ErrClientCannotProcessRequest, "failed to acquire locks")
	}
	res.release = releaseFn
	return res, nil
}

// checkRoleOwnership verifies that a Practitioner can only modify their own PractitionerRoles.
func (s *SlotUsecase) checkRoleOwnership(ctx context.Context, role, uid string, roles []fhir_dto.PractitionerRole) (string, error) {
	if role != constvars.KonsulinRolePractitioner {
		return s.verifyClinicAdminScope(ctx, role, uid, roles)
	}

	practitioners, err := s.practitioner.FindPractitionerByIdentifier(
		ctx,
		constvars.FhirSupertokenSystemIdentifier,
		uid,
	)
	if err != nil {
		return "", exceptions.BuildNewCustomError(err, http.StatusInternalServerError, err.Error(), err.Error())
	}
	if len(practitioners) != 1 {
		return "", exceptions.BuildNewCustomError(
			errors.New(errMultiplePractitionersFound),
			http.StatusBadRequest,
			errMultiplePractitionersFound,
			errMultiplePractitionersFound,
		)
	}
	practitioner := practitioners[0]
	for _, pr := range roles {
		if pr.Practitioner.Reference != practitionerRefPrefix+practitioner.ID {
			return "", exceptions.BuildNewCustomError(nil, constvars.StatusForbidden, constvars.ErrClientNotAuthorized, "practitioner cannot modify other practitioner's role")
		}
	}
	return practitionerRefPrefix + practitioner.ID, nil
}

// verifyClinicAdminScope ensures a Clinic Admin only modifies roles from their
// managing organization. The admin's organization set is derived from their own
// PractitionerRole resources carrying the administrative staff code.
func (s *SlotUsecase) verifyClinicAdminScope(ctx context.Context, role, uid string, roles []fhir_dto.PractitionerRole) (string, error) {
	if role != constvars.KonsulinRoleClinicAdmin {
		return "", nil
	}

	practitioners, err := s.practitioner.FindPractitionerByIdentifier(ctx, constvars.FhirSupertokenSystemIdentifier, uid)
	if err != nil {
		return "", exceptions.BuildNewCustomError(err, constvars.StatusInternalServerError, err.Error(), err.Error())
	}
	if len(practitioners) != 1 {
		return "", exceptions.BuildNewCustomError(
			errors.New(errMultiplePractitionersFound),
			constvars.StatusBadRequest,
			errMultiplePractitionersFound,
			errMultiplePractitionersFound,
		)
	}
	practitionerID := practitioners[0].ID

	adminRoles, err := s.practitionerRoles.FindPractitionerRoleByPractitionerID(ctx, practitionerID)
	if err != nil {
		return "", exceptions.BuildNewCustomError(err, constvars.StatusInternalServerError, err.Error(), err.Error())
	}
	adminOrgRefs := adminOrgReferences(adminRoles)
	if len(adminOrgRefs) == 0 {
		return "", exceptions.BuildNewCustomError(nil, constvars.StatusForbidden, constvars.ErrClientNotAuthorized, "clinic admin has no org-scoped admin role configured")
	}
	for _, pr := range roles {
		if !slices.Contains(adminOrgRefs, pr.Organization.Reference) {
			return "", exceptions.BuildNewCustomError(nil, constvars.StatusForbidden, constvars.ErrClientNotAuthorized, "clinic admin cannot modify roles from other organization")
		}
	}
	return practitionerRefPrefix + practitionerID, nil
}

// adminOrgReferences returns the organization references of the given
// PractitionerRole resources that carry the administrative staff code.
func adminOrgReferences(roles []fhir_dto.PractitionerRole) []string {
	var refs []string
	for _, r := range roles {
		if !hasRoleCode(r, constvars.FhirPractitionerRoleCodeAdministrativeStaff) {
			continue
		}
		if r.Organization.Reference != "" {
			refs = append(refs, r.Organization.Reference)
		}
	}
	return refs
}

// hasRoleCode reports whether any coding of the role's code element matches code.
func hasRoleCode(role fhir_dto.PractitionerRole, code string) bool {
	for _, cc := range role.Code {
		for _, c := range cc.Coding {
			if c.Code == code {
				return true
			}
		}
	}
	return false
}

// singleRoleResult holds the outcome of processing one PractitionerRole window.
type singleRoleResult struct {
	Conflicts    []contracts.ConflictingSlotItem
	Deletions    []string
	CreatedItems []createItem
	UpdatedRole  fhir_dto.PractitionerRole
}

// processSingleRoleWindowInput groups parameters for processSingleRoleWindow
type processSingleRoleWindowInput struct {
	pr                fhir_dto.PractitionerRole
	winStart, winEnd  time.Time
	resolved          *resolvedWindows
	input             contracts.SetUnavailabilityForMultiplePractitionerRolesInput
	unavailableReason string
	state             *idempotentState
}

// processSingleRoleWindow processes conflict detection, idempotency, and NA update for one role.
// Returns nil when the role is idempotent (nothing to change).
func (s *SlotUsecase) processSingleRoleWindow(ctx context.Context, in processSingleRoleWindowInput) (*singleRoleResult, error) {
	scheduleID := in.resolved.schedulesByRole[in.pr.ID]

	params := contracts.SlotSearchParams{
		Start:  "le" + in.winEnd.Format(time.RFC3339),
		End:    "ge" + in.winStart.Format(time.RFC3339),
		Status: "",
	}
	slots, err := s.slots.FindSlotsByScheduleWithQuery(ctx, scheduleID, params)
	if err != nil {
		s.logger.With(zap.Error(err)).Error("failed to find slots for conflict detection")
		return nil, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to find slots for conflict detection")
	}

	deletableIDs, isIdempotent, err := detectRoleWindowConflicts(slots, in.pr, in.winStart, in.winEnd, in.input.SlotStatus, in.unavailableReason, in.state)
	if err != nil {
		res := &singleRoleResult{}
		for _, c := range slots {
			if c.Status == fhir_dto.SlotStatusBusyUnavailable || c.Status == fhir_dto.SlotStatusBusyTentative {
				res.Conflicts = append(res.Conflicts, contracts.ConflictingSlotItem{
					PractitionerRoleID: in.pr.ID,
					SlotID:             c.ID,
					Start:              c.Start.Format(time.RFC3339),
					End:                c.End.Format(time.RFC3339),
					Status:             string(c.Status),
				})
			}
		}
		return res, err
	}
	if isIdempotent {
		return nil, nil
	}

	if err := in.pr.RemoveOutdatedNotAvailableReasons(); err != nil {
		return nil, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to prune notAvailable")
	}
	in.pr.AddNotAvailable(in.unavailableReason, in.winStart, in.winEnd)
	in.pr.ResourceType = constvars.ResourcePractitionerRole

	res := &singleRoleResult{
		Deletions:    deletableIDs,
		CreatedItems: []createItem{{scheduleID: scheduleID, start: in.winStart, end: in.winEnd}},
		UpdatedRole:  in.pr,
	}

	return res, nil
}

// processRoleWindows processes all roles: conflict detection, bundle building.
func (s *SlotUsecase) processRoleWindows(
	ctx context.Context,
	roles []fhir_dto.PractitionerRole,
	resolved *resolvedWindows,
	input contracts.SetUnavailabilityForMultiplePractitionerRolesInput,
	unavailableReason string,
	state *idempotentState,
) (*contracts.SetUnavailableOutcome, error) {
	var deletions []string
	var creations []createItem
	var updatedRoleBodies []fhir_dto.PractitionerRole
	out := &contracts.SetUnavailableOutcome{}

	for _, pr := range roles {
		winStart := resolved.startByRole[pr.ID]
		winEnd := resolved.endByRole[pr.ID]

		sr, err := s.processSingleRoleWindow(ctx, processSingleRoleWindowInput{
			pr:       pr,
			winStart: winStart, winEnd: winEnd,
			resolved:          resolved,
			input:             input,
			unavailableReason: unavailableReason,
			state:             state,
		})
		if err != nil {
			return out, err
		}
		if sr == nil {
			continue // idempotent, nothing to do
		}

		out.Conflicts = append(out.Conflicts, sr.Conflicts...)
		deletions = append(deletions, sr.Deletions...)
		creations = append(creations, sr.CreatedItems...)
		updatedRoleBodies = append(updatedRoleBodies, sr.UpdatedRole)
	}

	mutation := &bundleMutation{
		Deletions:    deletions,
		Creations:    creations,
		UpdatedRoles: updatedRoleBodies,
	}
	return s.postUnavailabilityBundle(ctx, out, mutation, state, input)
}

// postUnavailabilityBundle builds and posts the FHIR transaction bundle for unavailability changes.
// buildDeletionEntries builds bundle entries for deleting slots.
func buildDeletionEntries(deletions []string) []map[string]any {
	seen := make(map[string]struct{}, len(deletions))
	var entries []map[string]any
	for _, id := range deletions {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		entries = append(entries, map[string]any{
			constvars.FhirFieldRequest: map[string]any{constvars.FhirFieldMethod: constvars.MethodDelete, constvars.FhirFieldURL: "Slot/" + id},
		})
	}
	return entries
}

// buildBusySlotEntry builds a bundle entry for creating a busy slot.
func buildBusySlotEntry(c createItem, slotStatus fhir_dto.SlotStatus, reason string) map[string]any {
	return map[string]any{
		constvars.FhirFieldRequest: map[string]any{constvars.FhirFieldMethod: constvars.MethodPost, constvars.FhirFieldURL: constvars.ResourceSlot},
		constvars.FhirFieldResource: map[string]any{
			constvars.FhirFieldResourceType: constvars.ResourceSlot,
			constvars.FhirFieldSchedule:     map[string]any{constvars.FhirFieldReference: constvars.FHIRRefPrefixSchedule + c.scheduleID},
			constvars.FhirFieldStatus:       string(slotStatus),
			constvars.FhirFieldMeta: map[string]any{
				constvars.FhirFieldTag: []map[string]any{{constvars.FhirFieldCode: slotTagUserGenerated}},
			},
			"comment":                reason,
			constvars.FhirFieldStart: c.start.Format(time.RFC3339),
			constvars.FhirFieldEnd:   c.end.Format(time.RFC3339),
		},
	}
}

// buildRoleUpdateEntry builds a bundle entry for updating a practitioner role.
func buildRoleUpdateEntry(rb fhir_dto.PractitionerRole) map[string]any {
	return map[string]any{
		constvars.FhirFieldRequest:  map[string]any{constvars.FhirFieldMethod: constvars.MethodPut, constvars.FhirFieldURL: "PractitionerRole/" + rb.ID},
		constvars.FhirFieldResource: rb,
	}
}

// bundleMutation holds the pending mutation state for a FHIR transaction bundle.
type bundleMutation struct {
	Deletions    []string
	Creations    []createItem
	UpdatedRoles []fhir_dto.PractitionerRole
}

// postUnavailabilityBundle builds and posts the FHIR transaction bundle for unavailability changes.
// collectPostBundleResult queries created slots and collects practitioner role IDs after posting the bundle.
func (s *SlotUsecase) collectPostBundleResult(
	ctx context.Context,
	out *contracts.SetUnavailableOutcome,
	mutation *bundleMutation,
	input contracts.SetUnavailabilityForMultiplePractitionerRolesInput,
) {
	for _, c := range mutation.Creations {
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
	for _, rb := range mutation.UpdatedRoles {
		out.UpdatedPractitionerIDs = append(out.UpdatedPractitionerIDs, rb.ID)
	}
	out.Created = len(mutation.Creations) > 0
}

// postUnavailabilityBundle builds and posts the FHIR transaction bundle for unavailability changes.
func (s *SlotUsecase) postUnavailabilityBundle(
	ctx context.Context,
	out *contracts.SetUnavailableOutcome,
	mutation *bundleMutation,
	state *idempotentState,
	input contracts.SetUnavailabilityForMultiplePractitionerRolesInput,
) (*contracts.SetUnavailableOutcome, error) {
	noChanges := len(mutation.Deletions) == 0 && len(mutation.Creations) == 0 && len(mutation.UpdatedRoles) == 0
	if noChanges {
		out.Created = false
		out.CreatedSlots = append(out.CreatedSlots, *state.Slots...)
		out.UpdatedPractitionerIDs = append(out.UpdatedPractitionerIDs, *state.PRIDs...)
		return out, nil
	}

	entries := buildDeletionEntries(mutation.Deletions)
	for _, c := range mutation.Creations {
		entries = append(entries, buildBusySlotEntry(c, input.SlotStatus, input.Reason))
	}
	for _, rb := range mutation.UpdatedRoles {
		entries = append(entries, buildRoleUpdateEntry(rb))
	}

	bundle := map[string]any{constvars.FhirFieldResourceType: constvars.ResourceBundle, constvars.FhirBundleFieldType: constvars.FhirBundleTypeTransaction, constvars.FhirFieldEntry: entries}
	if _, err := s.bundles.PostTransactionBundle(ctx, bundle); err != nil {
		s.logger.With(zap.Error(err)).Error("failed to post transaction bundle")
		return out, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to post transaction bundle")
	}

	s.collectPostBundleResult(ctx, out, mutation, input)
	return out, nil
}

// detectRoleWindowConflicts detects conflicting slots, idempotency, and deletable IDs for one role's window.
// isSlotConflicting checks if a slot status indicates a booked conflict.
func isSlotConflicting(status fhir_dto.SlotStatus) bool {
	return status == fhir_dto.SlotStatusBusyUnavailable || status == fhir_dto.SlotStatusBusyTentative
}

// isSlotDeletable checks if a slot should be deleted during unavailability window adjustment.
func isSlotDeletable(status fhir_dto.SlotStatus) bool {
	return status == fhir_dto.SlotStatusFree || status == fhir_dto.SlotStatusBusyTentative
}

func detectRoleWindowConflicts(
	slots []fhir_dto.Slot,
	pr fhir_dto.PractitionerRole,
	winStart, winEnd time.Time,
	slotStatus fhir_dto.SlotStatus,
	unavailableReason string,
	state *idempotentState,
) (deletableIDs []string, isIdempotent bool, conflictErr error) {
	for _, sl := range slots {
		if isSlotConflicting(sl.Status) {
			conflictErr = exceptions.BuildNewCustomError(nil, constvars.StatusConflict, constvars.ErrClientCannotProcessRequest, "conflict detected with existing booked slots")
		}
		if sl.Start.Equal(winStart) && sl.End.Equal(winEnd) && sl.Status == slotStatus {
			isIdempotent = true
			*state.Slots = append(*state.Slots, contracts.CreatedSlotItem{
				ID:     sl.ID,
				Status: string(sl.Status),
			})
			*state.PRIDs = append(*state.PRIDs, pr.ID)
		}
	}

	if isIdempotent && hasExactNotAvailable(pr.NotAvailable, unavailableReason, winStart, winEnd) {
		isIdempotent = true
		return nil, true, nil
	}

	for _, sl := range slots {
		if isSlotDeletable(sl.Status) && sl.End.After(winStart) && sl.Start.Before(winEnd) {
			deletableIDs = append(deletableIDs, sl.ID)
		}
	}

	return
}

// hasExactNotAvailable checks if the practitioner role has an exact matching NotAvailable entry.
func hasExactNotAvailable(notAvail []fhir_dto.NotAvailable, reason string, winStart, winEnd time.Time) bool {
	for _, na := range notAvail {
		if na.Description == reason && na.During.Start == winStart.Format(time.RFC3339) && na.During.End == winEnd.Format(time.RFC3339) {
			return true
		}
	}
	return false
}

// loadPractitionerRoles loads PractitionerRole resources by their IDs.
func (s *SlotUsecase) loadPractitionerRoles(ctx context.Context, ids []string) ([]fhir_dto.PractitionerRole, error) {
	roles := make([]fhir_dto.PractitionerRole, 0, len(ids))
	for _, id := range ids {
		pr, err := s.practitionerRoles.FindPractitionerRoleByID(ctx, id)
		if err != nil || pr == nil {
			s.logger.With(zap.Error(err), zap.String("practitioner_role_id", id)).Error("failed to load practitioner role")
			return nil, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to load practitioner role")
		}
		roles = append(roles, *pr)
	}
	return roles, nil
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

// resolveRoleScheduleWindow resolves the schedule and unavailability window for
// one practitioner role, returning them along with the role's timezone.
func (s *SlotUsecase) resolveRoleScheduleWindow(ctx context.Context, pr fhir_dto.PractitionerRole, input contracts.SetUnavailabilityForMultiplePractitionerRolesInput) (scheduleID string, winStart, winEnd time.Time, loc *time.Location, err error) {
	loc, tzErr := pr.GetPreferredTimezone()
	if tzErr != nil {
		s.logger.With(zap.Error(tzErr), zap.String("practitioner_role_id", pr.ID)).Error("failed to resolve timezone")
		return "", time.Time{}, time.Time{}, nil, exceptions.BuildNewCustomError(tzErr, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to resolve timezone")
	}

	scheds, err := s.schedules.FindScheduleByPractitionerRoleID(ctx, pr.ID)
	if err != nil || len(scheds) == 0 {
		s.logger.With(zap.Error(err), zap.String("practitioner_role_id", pr.ID)).Error("failed to resolve schedule")
		return "", time.Time{}, time.Time{}, nil, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "failed to resolve schedule")
	}
	if len(scheds) != 1 {
		s.logger.With(zap.Int("count", len(scheds)), zap.String("practitioner_role_id", pr.ID)).Error("unexpected schedules count")
		return "", time.Time{}, time.Time{}, nil, exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "unexpected schedules count")
	}
	scheduleID = scheds[0].ID

	if input.AllDay {
		day, err := time.Parse("2006-01-02", input.AllDayDate)
		if err != nil {
			s.logger.With(zap.Error(err)).Error("invalid allDay date format")
			return "", time.Time{}, time.Time{}, nil, exceptions.BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "invalid allDay date format")
		}
		dayLocal := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
		winStart = dayLocal
		winEnd = dayLocal.Add(24 * time.Hour)
	} else {
		winStart = input.StartTime
		winEnd = input.EndTime
	}
	return scheduleID, winStart, winEnd, loc, nil
}

// practitionerDayLockTarget identifies a practitioner-local day to lock. The
// practitioner-day is the serialization unit: one practitioner can hold at most
// one booking per overlapping window, so every slot mutator contends on the same
// keys regardless of which schedule/role is involved.
type practitionerDayLockTarget struct {
	PractitionerID string
	Day            time.Time
}

// practitionerDayTargetsForWindow computes local days (inclusive) covered by
// [start,end) in the given location. Days are the natural boundaries for
// practitioner-day locks; each caller computes them in its own local context
// (booked slot offset for booking/callback, role Period timezone for the worker).
func (s *SlotUsecase) practitionerDayTargetsForWindow(practitionerID string, loc *time.Location, start, end time.Time) []practitionerDayLockTarget {
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

	var out []practitionerDayLockTarget
	for d := day; !d.After(last); d = d.AddDate(0, 0, 1) {
		out = append(out, practitionerDayLockTarget{PractitionerID: practitionerID, Day: d})
	}
	return out
}

// practitionerDayLockKey builds the practitioner-local-day lock key.
// The key deliberately drops any timezone suffix: each caller computes the local
// day in its own context, so the same practitioner+date always maps to one key.
func (s *SlotUsecase) practitionerDayLockKey(practitionerID string, day time.Time) string {
	y, m, d := day.Date()
	return fmt.Sprintf("slotgen:lock:practitioner:%s:%04d-%02d-%02d", practitionerID, y, int(m), d)
}

// sortPractitionerDayTargets orders targets deterministically by practitioner, then day.
func sortPractitionerDayTargets(targets []practitionerDayLockTarget) {
	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].PractitionerID != targets[j].PractitionerID {
			return targets[i].PractitionerID < targets[j].PractitionerID
		}
		return targets[i].Day.Before(targets[j].Day)
	})
}

// dedupePractitionerDayTargets appends t to out unless a target with the same
// practitioner and local day was already seen.
func dedupePractitionerDayTargets(seen map[string]struct{}, out []practitionerDayLockTarget, t practitionerDayLockTarget) []practitionerDayLockTarget {
	key := fmt.Sprintf("%s|%04d-%02d-%02d", t.PractitionerID, t.Day.Year(), int(t.Day.Month()), t.Day.Day())
	if _, ok := seen[key]; ok {
		return out
	}
	seen[key] = struct{}{}
	return append(out, t)
}

// acquireLocksOrdered acquires the given lock keys in deterministic order,
// releasing all previously acquired locks if any acquisition fails, and returns
// a release closure. keyFor maps a target to its lock key.
func acquireLocksOrdered[T any](ctx context.Context, locker contracts.LockerService, targets []T, ttl time.Duration, keyFor func(T) string) (func(context.Context), error) {
	type acquiredLock struct{ key, tok string }
	acquiredList := make([]acquiredLock, 0, len(targets))
	for _, t := range targets {
		key := keyFor(t)
		ok, tok, err := locker.TryLock(ctx, key, ttl)
		if err != nil || !ok {
			for i := len(acquiredList) - 1; i >= 0; i-- {
				_ = locker.Unlock(ctx, acquiredList[i].key, acquiredList[i].tok)
			}
			if err == nil {
				err = fmt.Errorf("failed to acquire lock: %s", key)
			}
			return nil, err
		}
		acquiredList = append(acquiredList, acquiredLock{key: key, tok: tok})
	}
	release := func(ctx context.Context) {
		for i := len(acquiredList) - 1; i >= 0; i-- {
			_ = locker.Unlock(ctx, acquiredList[i].key, acquiredList[i].tok)
		}
	}
	return release, nil
}

// acquirePractitionerDayLocksOrdered acquires locks in deterministic order and returns a release closure
func (s *SlotUsecase) acquirePractitionerDayLocksOrdered(ctx context.Context, targets []practitionerDayLockTarget, ttl time.Duration) (func(context.Context), error) {
	return acquireLocksOrdered(ctx, s.locker, targets, ttl, func(t practitionerDayLockTarget) string {
		return s.practitionerDayLockKey(t.PractitionerID, t.Day)
	})
}

// AcquireLocksForPractitionerDay acquires day locks for a practitioner across the
// appointment window. Local days are computed from the window's own location (the
// booked slot's offset); sibling role timezones are never resolved, so schedule-less
// or period-less sibling roles cannot break a booking.
func (s *SlotUsecase) AcquireLocksForPractitionerDay(ctx context.Context, practitionerID string, start, end time.Time, ttl time.Duration) (func(context.Context), error) {
	if start.IsZero() || end.IsZero() {
		return nil, fmt.Errorf("appointment window start/end must not be zero")
	}
	loc := start.Location()
	if loc == nil {
		return nil, fmt.Errorf("appointment window start has no location/timezone")
	}
	targets := s.practitionerDayTargetsForWindow(practitionerID, loc, start, end)
	return s.acquirePractitionerDayLocksOrdered(ctx, targets, ttl)
}
