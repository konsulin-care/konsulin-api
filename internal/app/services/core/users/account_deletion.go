package users

import (
	"context"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"

	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

// accountDeletionService deletes SuperTokens accounts after a successful FHIR
// purge. supertokens.DeleteUser also revokes all of the user's sessions.
type accountDeletionService struct {
	log      *zap.Logger
	deleteFn func(userID string) error
}

// NewAccountDeletionService returns an AccountDeletionService backed by the
// SuperTokens Go SDK. deleteFn is injectable for tests.
func NewAccountDeletionService(logger *zap.Logger) contracts.AccountDeletionService {
	return &accountDeletionService{
		log:      logger,
		deleteFn: supertokens.DeleteUser,
	}
}

// DeleteUserAccount permanently deletes the SuperTokens account for userID,
// which also revokes all active sessions for that user.
func (s *accountDeletionService) DeleteUserAccount(_ context.Context, userID string) error {
	s.log.Info("accountDeletionService.DeleteUserAccount called",
		zap.String(constvars.LoggingUserIDKey, userID))
	if err := s.deleteFn(userID); err != nil {
		s.log.Error("accountDeletionService.DeleteUserAccount failed",
			zap.String(constvars.LoggingUserIDKey, userID),
			zap.Error(err))
		return err
	}
	return nil
}
