package session

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/exceptions"
)

type sessionService struct {
	RedisRepository contracts.RedisRepository
}

func NewSessionService(redisRepository contracts.RedisRepository) contracts.SessionService {
	return &sessionService{
		RedisRepository: redisRepository,
	}
}

func (svc *sessionService) ParseSessionData(ctx context.Context, sessionData string) (*models.Session, error) {
	session := new(models.Session)
	err := json.Unmarshal([]byte(sessionData), session)
	if err != nil {
		return nil, exceptions.ErrCannotParseJSON(err)
	}
	return session, nil
}

func (svc *sessionService) GetSessionData(ctx context.Context, sessionID string) (string, error) {
	sessionData, err := svc.RedisRepository.Get(ctx, sessionID)
	if err != nil {
		return "", exceptions.ErrTokenInvalidOrExpired(err)
	}
	return sessionData, nil
}
