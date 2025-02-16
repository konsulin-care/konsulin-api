package session

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"sync"

	"go.uber.org/zap"
)

type sessionService struct {
	RedisRepository contracts.RedisRepository
	Log             *zap.Logger
}

var (
	sessionServiceInstance contracts.SessionService
	onceSessionService     sync.Once
)

func NewSessionService(redisRepository contracts.RedisRepository, logger *zap.Logger) contracts.SessionService {
	onceSessionService.Do(func() {
		instance := &sessionService{
			RedisRepository: redisRepository,
			Log:             logger,
		}
		sessionServiceInstance = instance
	})
	return sessionServiceInstance
}

func (svc *sessionService) ParseSessionData(ctx context.Context, sessionData string) (*models.Session, error) {
	requestID, ok := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok {
		svc.Log.Error("sessionService.ParseSessionData requestID not found in context")
		return nil, exceptions.ErrMissingRequestID(nil)
	}

	svc.Log.Info("sessionService.ParseSessionData called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRawSessionDataKey, sessionData))

	session := new(models.Session)
	err := json.Unmarshal([]byte(sessionData), session)
	if err != nil {
		svc.Log.Error("sessionService.ParseSessionData error parsing session JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err))
		return nil, exceptions.ErrCannotParseJSON(err)
	}

	svc.Log.Info("sessionService.ParseSessionData succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingSessionDataKey, session))
	return session, nil
}

func (svc *sessionService) GetSessionData(ctx context.Context, sessionID string) (string, error) {
	requestID, ok := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok {
		svc.Log.Error("sessionService.GetSessionData requestID not found in context")
		return "", exceptions.ErrMissingRequestID(nil)
	}

	svc.Log.Info("sessionService.GetSessionData called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSessionIDKey, sessionID))

	sessionData, err := svc.RedisRepository.Get(ctx, sessionID)
	if err != nil {
		svc.Log.Error("sessionService.GetSessionData error fetching session data from Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingSessionIDKey, sessionID),
			zap.Error(err))
		return "", exceptions.ErrTokenInvalidOrExpired(err)
	}

	svc.Log.Info("sessionService.GetSessionData succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSessionDataKey, sessionData))
	return sessionData, nil
}
