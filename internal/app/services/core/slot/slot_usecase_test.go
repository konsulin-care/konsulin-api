package slot

import (
	"context"
	"testing"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

type mockPractitionerFhirClient struct {
	mock.Mock
}

func (m *mockPractitionerFhirClient) CreatePractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	args := m.Called(ctx, req)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) UpdatePractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) { // NOSONAR:go:S4144 testify mock idiom
	args := m.Called(ctx, req)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) PatchPractitioner(ctx context.Context, req *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) { // NOSONAR:go:S4144 testify mock idiom
	args := m.Called(ctx, req)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) FindPractitionerByID(ctx context.Context, practitionerID string) (*fhir_dto.Practitioner, error) {
	args := m.Called(ctx, practitionerID)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) FindPractitionerByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.Practitioner, error) {
	args := m.Called(ctx, system, value)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) FindPractitionerByEmail(ctx context.Context, email string) ([]fhir_dto.Practitioner, error) {
	args := m.Called(ctx, email)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerFhirClient) FindPractitionerByPhone(ctx context.Context, phone string) ([]fhir_dto.Practitioner, error) {
	args := m.Called(ctx, phone)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.Practitioner), args.Error(1)
	}
	return nil, args.Error(1)
}

type mockPractitionerRoleFhirClient struct {
	mock.Mock
}

func (m *mockPractitionerRoleFhirClient) DeletePractitionerRoleByID(ctx context.Context, practitionerRoleID string) error {
	return m.Called(ctx, practitionerRoleID).Error(0)
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByOrganizationID(ctx context.Context, organizationID string) ([]fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, organizationID)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByCustomRequest(ctx context.Context, request *requests.FindAllCliniciansByClinicID) ([]fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, request)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, practitionerID)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx context.Context, practitionerID, organizationID string) ([]fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, practitionerID, organizationID)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) CreatePractitionerRoles(ctx context.Context, request interface{}) error {
	return m.Called(ctx, request).Error(0)
}

func (m *mockPractitionerRoleFhirClient) CreatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, request)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) UpdatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) { // NOSONAR:go:S4144 testify mock idiom
	args := m.Called(ctx, request)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndName(ctx context.Context, request *requests.FindClinicianByClinicianID) ([]fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, request)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) FindPractitionerRoleByID(ctx context.Context, practitionerRoleID string) (*fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, practitionerRoleID)
	if v := args.Get(0); v != nil {
		return v.(*fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPractitionerRoleFhirClient) Search(ctx context.Context, params contracts.PractitionerRoleSearchParams) ([]fhir_dto.PractitionerRole, error) {
	args := m.Called(ctx, params)
	if v := args.Get(0); v != nil {
		return v.([]fhir_dto.PractitionerRole), args.Error(1)
	}
	return nil, args.Error(1)
}

func adminStaffRole(orgRef string) fhir_dto.PractitionerRole {
	return fhir_dto.PractitionerRole{
		Code: []fhir_dto.CodeableConcept{{
			Coding: []fhir_dto.Coding{{Code: constvars.FhirPractitionerRoleCodeAdministrativeStaff}},
		}},
		Organization: fhir_dto.Reference{Reference: orgRef},
	}
}

func targetRole(orgRef string) fhir_dto.PractitionerRole {
	return fhir_dto.PractitionerRole{
		Organization: fhir_dto.Reference{Reference: orgRef},
	}
}

// TestVerifyClinicAdminScope verifies the clinic admin scope check resolves the
// admin's org from their admin-coded PractitionerRole instead of Person.
func TestVerifyClinicAdminScope(t *testing.T) {
	t.Run("non-admin role returns empty without client calls", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		mockPR := new(mockPractitionerRoleFhirClient)
		s := &SlotUsecase{practitioner: mockPrac, practitionerRoles: mockPR}

		ref, err := s.verifyClinicAdminScope(context.Background(), constvars.KonsulinRolePractitioner, "st-123", nil)
		assert.NoError(t, err)
		assert.Equal(t, "", ref)
		mockPrac.AssertNotCalled(t, "FindPractitionerByIdentifier", mock.Anything, mock.Anything, mock.Anything)
		mockPR.AssertNotCalled(t, "FindPractitionerRoleByPractitionerID", mock.Anything, mock.Anything)
	})

	t.Run("admin with matching org passes", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		mockPR := new(mockPractitionerRoleFhirClient)
		s := &SlotUsecase{practitioner: mockPrac, practitionerRoles: mockPR}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{{ID: "prac-1"}}, nil)
		mockPR.On("FindPractitionerRoleByPractitionerID", mock.Anything, "prac-1").
			Return([]fhir_dto.PractitionerRole{adminStaffRole("Organization/org-1")}, nil)

		ref, err := s.verifyClinicAdminScope(context.Background(), constvars.KonsulinRoleClinicAdmin, "st-123",
			[]fhir_dto.PractitionerRole{targetRole("Organization/org-1")})
		assert.NoError(t, err)
		assert.Equal(t, "Practitioner/prac-1", ref)
		mockPrac.AssertExpectations(t)
		mockPR.AssertExpectations(t)
	})

	t.Run("admin modifying other organization rejected", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		mockPR := new(mockPractitionerRoleFhirClient)
		s := &SlotUsecase{practitioner: mockPrac, practitionerRoles: mockPR}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{{ID: "prac-1"}}, nil)
		mockPR.On("FindPractitionerRoleByPractitionerID", mock.Anything, "prac-1").
			Return([]fhir_dto.PractitionerRole{adminStaffRole("Organization/org-1")}, nil)

		_, err := s.verifyClinicAdminScope(context.Background(), constvars.KonsulinRoleClinicAdmin, "st-123",
			[]fhir_dto.PractitionerRole{targetRole("Organization/org-2")})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "other organization")
	})

	t.Run("admin without org-scoped admin role rejected", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		mockPR := new(mockPractitionerRoleFhirClient)
		s := &SlotUsecase{practitioner: mockPrac, practitionerRoles: mockPR}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{{ID: "prac-1"}}, nil)
		mockPR.On("FindPractitionerRoleByPractitionerID", mock.Anything, "prac-1").
			Return([]fhir_dto.PractitionerRole{{Organization: fhir_dto.Reference{Reference: "Organization/org-1"}}}, nil)

		_, err := s.verifyClinicAdminScope(context.Background(), constvars.KonsulinRoleClinicAdmin, "st-123",
			[]fhir_dto.PractitionerRole{targetRole("Organization/org-1")})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no org-scoped admin role")
	})

	t.Run("practitioner not found rejected", func(t *testing.T) {
		mockPrac := new(mockPractitionerFhirClient)
		mockPR := new(mockPractitionerRoleFhirClient)
		s := &SlotUsecase{practitioner: mockPrac, practitionerRoles: mockPR}

		mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
			Return([]fhir_dto.Practitioner{}, nil)

		_, err := s.verifyClinicAdminScope(context.Background(), constvars.KonsulinRoleClinicAdmin, "st-123", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no practitioner found")
	})
}

// TestCheckRoleOwnershipMultiplePractitioners pins the exact error message a
// practitioner gets when the supertoken identifier resolves to zero or several
// Practitioner resources (constantized to avoid string drift).
func TestCheckRoleOwnershipMultiplePractitioners(t *testing.T) {
	mockPrac := new(mockPractitionerFhirClient)
	s := &SlotUsecase{practitioner: mockPrac}

	mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
		Return([]fhir_dto.Practitioner{{ID: "p-1"}, {ID: "p-2"}}, nil)

	_, err := s.checkRoleOwnership(context.Background(), constvars.KonsulinRolePractitioner, "st-123", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "multiple practitioners found on the same identifier or no practitioner found at all")
	mockPrac.AssertExpectations(t)
}

// mockSlotFhirClient mocks contracts.SlotFhirClient with a scripted
// FindSlotsByScheduleWithQuery; all other methods are unused in these tests.
type mockSlotFhirClient struct {
	slotsBySchedule func(scheduleID string, params contracts.SlotSearchParams) ([]fhir_dto.Slot, error)
}

func (*mockSlotFhirClient) FindSlotByScheduleID(_ context.Context, _ string) ([]fhir_dto.Slot, error) {
	return nil, nil
}

func (*mockSlotFhirClient) FindSlotByScheduleAndTimeRange(_ context.Context, _ string, _, _ time.Time) ([]fhir_dto.Slot, error) {
	return nil, nil
}

func (*mockSlotFhirClient) FindSlotByScheduleIDAndStatus(_ context.Context, _, _ string) ([]fhir_dto.Slot, error) {
	return nil, nil
}

func (*mockSlotFhirClient) FindSlotByID(_ context.Context, _ string) (*fhir_dto.Slot, error) {
	return nil, nil
}

func (*mockSlotFhirClient) CreateSlot(_ context.Context, _ *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	return nil, nil
}

func (*mockSlotFhirClient) UpdateSlot(_ context.Context, _ string, _ *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	return nil, nil
}

func (m *mockSlotFhirClient) FindSlotsByScheduleWithQuery(_ context.Context, scheduleID string, params contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
	if m.slotsBySchedule != nil {
		return m.slotsBySchedule(scheduleID, params)
	}
	return nil, nil
}

func (*mockSlotFhirClient) PostTransactionBundle(_ context.Context, _ map[string]any) (*fhir_dto.FHIRBundle, error) {
	return nil, nil
}

// TestSetUnavailabilityReportsConflictForBusySlot verifies that a booked busy
// slot inside the requested unavailability window blocks the request with a
// conflict error: busy counts alongside busy-unavailable and busy-tentative.
func TestSetUnavailabilityReportsConflictForBusySlot(t *testing.T) {
	loc := time.FixedZone("+07:00", 7*3600)
	ctx := context.WithValue(context.Background(), constvars.CONTEXT_FHIR_ROLE, []string{constvars.KonsulinRolePractitioner})
	ctx = context.WithValue(ctx, constvars.CONTEXT_UID, "st-123")

	role := fhir_dto.PractitionerRole{
		ID:           "role-1",
		Practitioner: fhir_dto.Reference{Reference: "Practitioner/prac-1"},
		Period:       fhir_dto.Period{Start: "2026-08-13T00:00:00+07:00"},
	}

	mockPR := new(mockPractitionerRoleFhirClient)
	mockPR.On("FindPractitionerRoleByID", mock.Anything, "role-1").Return(&role, nil)

	mockPrac := new(mockPractitionerFhirClient)
	mockPrac.On("FindPractitionerByIdentifier", mock.Anything, constvars.FhirSupertokenSystemIdentifier, "st-123").
		Return([]fhir_dto.Practitioner{{ID: "prac-1"}}, nil)

	mockSched := new(mockScheduleFhirClient)
	mockSched.On("FindScheduleByPractitionerRoleID", mock.Anything, "role-1").Return([]fhir_dto.Schedule{{ID: "sched-1"}}, nil)

	lockKey := "slotgen:lock:practitioner:prac-1:2026-08-13"
	mockLocker := new(mockLockerService)
	mockLocker.On("TryLock", mock.Anything, lockKey, 30*time.Second).Return(true, "tok-1", nil)
	mockLocker.On("Unlock", mock.Anything, lockKey, "tok-1").Return(nil)

	busyStart := time.Date(2026, time.August, 13, 9, 30, 0, 0, loc)
	busyEnd := time.Date(2026, time.August, 13, 10, 0, 0, 0, loc)
	slotMock := &mockSlotFhirClient{slotsBySchedule: func(scheduleID string, _ contracts.SlotSearchParams) ([]fhir_dto.Slot, error) {
		assert.Equal(t, "sched-1", scheduleID)
		return []fhir_dto.Slot{{ID: "slot-busy", Status: fhir_dto.SlotStatusBusy, Start: busyStart, End: busyEnd}}, nil
	}}

	s := &SlotUsecase{
		schedules:         mockSched,
		locker:            mockLocker,
		slots:             slotMock,
		practitionerRoles: mockPR,
		practitioner:      mockPrac,
		logger:            zap.NewNop(),
	}

	out, err := s.HandleSetUnavailabilityForMultiplePractitionerRoles(ctx, contracts.SetUnavailabilityForMultiplePractitionerRolesInput{
		PractitionerRoleIDs: []string{"role-1"},
		StartTime:           time.Date(2026, time.August, 13, 9, 0, 0, 0, loc),
		EndTime:             time.Date(2026, time.August, 13, 11, 0, 0, 0, loc),
		Reason:              "manual unavailability",
		SlotStatus:          fhir_dto.SlotStatusBusyUnavailable,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conflict detected with existing booked slots")
	assert.Nil(t, out)
	mockPrac.AssertExpectations(t)
	mockPR.AssertExpectations(t)
	mockSched.AssertExpectations(t)
}
