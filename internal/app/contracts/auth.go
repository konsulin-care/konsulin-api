package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"

	"github.com/supertokens/supertokens-golang/recipe/passwordless/plessmodels"
)

type CheckUserExistsOutput struct {
	SupertokenUser  *plessmodels.User
	PatientIds      []string
	PractitionerIds []string
}

type AnonymousSessionResult struct {
	Token   string
	GuestID string
	IsNew   bool
}

type ClaimAnonymousResourcesOutput struct {
	Count         int
	ReferenceList []string
}

type AuthUsecase interface {
	InitializeSupertoken() error
	LogoutUser(ctx context.Context, sessionData string) error
	CreateMagicLink(ctx context.Context, request *requests.SupertokenPasswordlessCreateMagicLink) error
	CreateAnonymousSession(ctx context.Context, existingToken string) (*AnonymousSessionResult, error)
	ClaimAnonymousResources(ctx context.Context, supertokensUserID string, roles []string, anonToken string) (*ClaimAnonymousResourcesOutput, error)
	CheckUserExists(ctx context.Context, email string) (*CheckUserExistsOutput, error)
	CheckUserExistsByPhone(ctx context.Context, phone string) (*CheckUserExistsOutput, error)
}

type AuthRepository interface{}
