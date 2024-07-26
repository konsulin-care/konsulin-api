package session

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/pkg/exceptions"
)

type sessionService struct {
	RedisRepository redis.RedisRepository
}

func NewSessionService(redisRepository redis.RedisRepository) SessionService {
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
		return "", exceptions.ErrTokenInvalid(err)
	}
	return sessionData, nil
}
