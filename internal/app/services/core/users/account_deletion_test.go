package users

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteUserAccount_CallsDeleteWithUserID(t *testing.T) {
	var gotUserID string
	svc := NewAccountDeletionService(zap.NewNop()).(*accountDeletionService)
	svc.deleteFn = func(userID string) error {
		gotUserID = userID
		return nil
	}

	err := svc.DeleteUserAccount(context.Background(), "user-123")
	require.NoError(t, err)
	assert.Equal(t, "user-123", gotUserID)
}

func TestDeleteUserAccount_PropagatesError(t *testing.T) {
	svc := NewAccountDeletionService(zap.NewNop()).(*accountDeletionService)
	svc.deleteFn = func(_ string) error {
		return errors.New("core unreachable")
	}

	err := svc.DeleteUserAccount(context.Background(), "user-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "core unreachable")
}
