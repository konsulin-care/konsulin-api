package contracts

import (
	"context"
	"konsulin-service/internal/app/models"
)

type SessionService interface {
	ParseSessionData(ctx context.Context, sessionData string) (*models.Session, error)
	GetSessionData(ctx context.Context, sessionID string) (sessionData string, err error)
}
