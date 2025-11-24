package organization

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	bundleSvc "konsulin-service/internal/app/services/fhir_spark/bundle"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"strings"
	"time"

	"slices"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Usecase implements contracts.OrganizationUsecase.
type Usecase struct {
	practitionerClient contracts.PractitionerFhirClient
	personClient       contracts.PersonFhirClient
	organizationClient contracts.OrganizationFhirClient
	bundleClient       bundleSvc.BundleFhirClient
	config             *config.InternalConfig
	log                *zap.Logger
	httpClient         *http.Client
}

// NewOrganizationUsecase constructs a new Organization usecase.
func NewOrganizationUsecase(
	practitionerClient contracts.PractitionerFhirClient,
	personClient contracts.PersonFhirClient,
	organizationClient contracts.OrganizationFhirClient,
	bundles bundleSvc.BundleFhirClient,
	cfg *config.InternalConfig,
	log *zap.Logger,
) contracts.OrganizationUsecase {
	return &Usecase{
		practitionerClient: practitionerClient,
		personClient:       personClient,
		organizationClient: organizationClient,
		bundleClient:       bundles,
		config:             cfg,
		log:                log,
		httpClient:         &http.Client{},
	}
}

// RegisterPractitionerRoleAndSchedule implements the flow to:
//   - enforce caller role and organization scope
//   - resolve Practitioner by email (optionally triggering magiclink)
//   - create PractitionerRole and Schedule via FHIR transaction bundle.
func (uc *Usecase) RegisterPractitionerRoleAndSchedule(ctx context.Context, in contracts.RegisterPractitionerRoleInput) (*contracts.RegisterPractitionerRoleOutput, error) {
	if strings.TrimSpace(in.OrganizationID) == "" || strings.TrimSpace(in.Email) == "" {
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusBadRequest,
			constvars.ErrClientCannotProcessRequest,
			"organizationId and email are required",
		)
	}

	role, uid, authErr := uc.whitelistAccessByRoles(
		ctx,
		[]string{
			constvars.KonsulinRoleClinicAdmin,
			constvars.KonsulinRoleSuperadmin,
		},
	)
	if authErr != nil {
		uc.log.With(zap.Error(authErr)).Error("authorization failed for register practitioner role")
		return nil, exceptions.BuildNewCustomError(
			authErr,
			constvars.StatusForbidden,
			constvars.ErrClientNotAuthorized,
			"authorization failed for register practitioner role",
		)
	}

	_, err := uc.organizationClient.FindOrganizationByID(ctx, in.OrganizationID)
	if err != nil {
		notFoundErr := exceptions.BuildNewCustomError(
			errors.New("organization not found"),
			constvars.StatusNotFound,
			constvars.ErrClientCannotProcessRequest,
			fmt.Sprintf("organization %s does not exists", in.OrganizationID),
		)
		return nil, notFoundErr
	}

	// Clinic Admin must manage the target organization.
	if role == constvars.KonsulinRoleClinicAdmin {
		if err := uc.ensureClinicAdminManagesOrganization(ctx, uid, in.OrganizationID); err != nil {
			return nil, err
		}
	}

	practitioner, err := uc.ensurePractitionerExistsByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format(time.RFC3339)
	practitionerRoleID := uuid.New().String()
	scheduleID := uuid.New().String()

	entries := []map[string]any{
		{
			"resource": map[string]any{
				"resourceType": constvars.ResourcePractitionerRole,
				"id":           practitionerRoleID,
				"active":       false,
				"practitioner": map[string]any{"reference": "Practitioner/" + practitioner.ID},
				"organization": map[string]any{"reference": "Organization/" + in.OrganizationID},
				"period":       map[string]any{"start": now},
			},
			"request": map[string]any{
				"method": http.MethodPut,
				"url":    fmt.Sprintf("%s/%s", constvars.ResourcePractitionerRole, practitionerRoleID),
			},
		},
		{
			"resource": map[string]any{
				"resourceType": constvars.ResourceSchedule,
				"id":           scheduleID,
				"actor": []map[string]any{
					{"reference": "Practitioner/" + practitioner.ID},
					{"reference": "PractitionerRole/" + practitionerRoleID},
				},
			},
			"request": map[string]any{
				"method": http.MethodPut,
				"url":    fmt.Sprintf("%s/%s", constvars.ResourceSchedule, scheduleID),
			},
		},
	}

	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "transaction",
		"entry":        entries,
	}

	_, err = uc.bundleClient.PostTransactionBundle(ctx, bundle)
	if err != nil {
		uc.log.With(zap.Error(err)).Error("failed to post PractitionerRole+Schedule transaction bundle")
		// err is already mapped to a CustomError in most cases, so just return it.
		return nil, err
	}

	return &contracts.RegisterPractitionerRoleOutput{
		PractitionerID:     practitioner.ID,
		PractitionerRoleID: practitionerRoleID,
		ScheduleID:         scheduleID,
	}, nil
}

func (uc *Usecase) whitelistAccessByRoles(ctx context.Context, whiteListed []string) (string, string, error) {
	roles, _ := ctx.Value(constvars.CONTEXT_FHIR_ROLE).([]string)
	uid, _ := ctx.Value(constvars.CONTEXT_UID).(string)

	for _, r := range roles {
		if slices.Contains(whiteListed, r) {
			return r, uid, nil
		}
	}

	return "", "", errors.New("current role is not permitted to access")
}

// ensureClinicAdminManagesOrganization enforces that the clinic admin identified
// by uid manages the given organization ID, using Person.ManagingOrganization.
func (uc *Usecase) ensureClinicAdminManagesOrganization(ctx context.Context, uid, organizationID string) error {
	identifierToken := fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, uid)
	people, err := uc.personClient.Search(ctx, contracts.PersonSearchInput{Identifier: identifierToken})
	if err != nil {
		return exceptions.BuildNewCustomError(
			err,
			constvars.StatusInternalServerError,
			err.Error(),
			err.Error(),
		)
	}
	if len(people) != 1 {
		errMultiPersons := errors.New("multiple persons found on the same identifier or no person found at all")
		return exceptions.BuildNewCustomError(
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
		uc.log.With(zap.Error(err)).Error("organization scope check failed: missing managingOrganization on admin")
		return err
	}

	expectedRef := fmt.Sprintf("%s/%s", constvars.ResourceOrganization, organizationID)
	if adminOrgRef != expectedRef {
		msg := fmt.Sprintf("The requesting account does not manage %s", expectedRef)
		err := exceptions.BuildNewCustomError(nil, constvars.StatusForbidden, constvars.ErrClientNotAuthorized, msg)
		uc.log.With(zap.Error(err)).Error("organization scope check failed: admin does not manage organization")
		return err
	}

	return nil
}

// ensurePractitionerExistsByEmail resolves a Practitioner by email. If none exists,
// it triggers the magiclink flow and retries the Practitioner lookup once.
func (uc *Usecase) ensurePractitionerExistsByEmail(ctx context.Context, email string) (*fhir_dto.Practitioner, error) {
	practitioners, err := uc.practitionerClient.FindPractitionerByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if len(practitioners) > 0 {
		return &practitioners[0], nil
	}

	if err := uc.callMagicLink(ctx, email); err != nil {
		return nil, exceptions.ErrServerProcess(err)
	}

	practitioners, err = uc.practitionerClient.FindPractitionerByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if len(practitioners) == 0 {
		return nil, exceptions.BuildNewCustomError(
			nil,
			constvars.StatusNotFound,
			constvars.ErrClientCannotProcessRequest,
			"practitioner not found after magic link trigger",
		)
	}

	return &practitioners[0], nil
}

// callMagicLink calls the local /api/v1/auth/magiclink endpoint using the
// configured App.BaseUrl and Superadmin API key.
func (uc *Usecase) callMagicLink(ctx context.Context, email string) error {
	baseURL := strings.TrimRight(uc.config.App.BaseUrl, "/")
	if baseURL == "" {
		return fmt.Errorf("app base url is not configured")
	}
	url := baseURL + "/api/v1/auth/magiclink"

	body := map[string]any{
		"email": email,
		"roles": []string{
			constvars.KonsulinRolePractitioner,
			constvars.KonsulinRolePatient,
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	req.Header.Set(constvars.HeaderXApiKey, uc.config.App.SuperadminAPIKey)

	resp, err := uc.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("magiclink request failed with status %d", resp.StatusCode)
	}
	return nil
}
