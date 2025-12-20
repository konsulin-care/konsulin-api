package session

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"sync"
	"time"

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
	start := time.Now()
	requestID, ok := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok {
		svc.Log.Error("Request ID missing from context",
			zap.String(constvars.LoggingOperationKey, "parse_session_data"),
		)
		return nil, exceptions.ErrMissingRequestID(nil)
	}

	svc.Log.Debug("Session data parsing started",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOperationKey, "parse_session_data"),
		zap.String(constvars.LoggingRawSessionDataKey, sessionData))

	session := new(models.Session)
	err := json.Unmarshal([]byte(sessionData), session)
	if err != nil {
		svc.Log.Error("Failed to parse session JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingErrorTypeKey, "JSON parsing"),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err))
		return nil, exceptions.ErrCannotParseJSON(err)
	}

	svc.Log.Debug("Session data parsed successfully",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, session.UserID),
		zap.String("role_name", session.RoleName),
		zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
		zap.Bool(constvars.LoggingSuccessKey, true))
	return session, nil
}

func (svc *sessionService) GetSessionData(ctx context.Context, sessionID string) (string, error) {
	start := time.Now()
	requestID, ok := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok {
		svc.Log.Error("Request ID missing from context",
			zap.String(constvars.LoggingOperationKey, "get_session_data"),
		)
		return "", exceptions.ErrMissingRequestID(nil)
	}

	svc.Log.Debug("Session data retrieval started",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSessionIDKey, sessionID),
		zap.String(constvars.LoggingOperationKey, "get_session_data"))

	sessionData, err := svc.RedisRepository.Get(ctx, sessionID)
	if err != nil {
		svc.Log.Error("Failed to fetch session data from Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingSessionIDKey, sessionID),
			zap.String(constvars.LoggingErrorTypeKey, "redis query"),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err))
		return "", exceptions.ErrTokenInvalidOrExpired(err)
	}

	svc.Log.Debug("Session data retrieved successfully",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingSessionIDKey, sessionID),
		zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
		zap.Bool(constvars.LoggingSuccessKey, true))
	return sessionData, nil
}
