package educationLevels

import (
	"context"
	"database/sql"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/queries"
	"sync"

	"go.uber.org/zap"
)

type educationLevelPostgresRepository struct {
	DB  *sql.DB
	Log *zap.Logger
}

var (
	educationLevelPostgresRepositoryInstance contracts.EducationLevelRepository
	onceEducationLevelPostgresRepository     sync.Once
)

func NewEducationLevelPostgresRepository(db *sql.DB, logger *zap.Logger) contracts.EducationLevelRepository {
	onceEducationLevelPostgresRepository.Do(func() {
		instance := &educationLevelPostgresRepository{
			DB:  db,
			Log: logger,
		}
		educationLevelPostgresRepositoryInstance = instance
	})
	return educationLevelPostgresRepositoryInstance
}

func (repo *educationLevelPostgresRepository) FindAll(ctx context.Context) ([]models.EducationLevel, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("educationLevelPostgresRepository.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	query := queries.GetAllEducationLevels
	rows, err := repo.DB.QueryContext(ctx, query)
	if err != nil {
		repo.Log.Error("educationLevelPostgresRepository.FindAll error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	defer rows.Close()

	var educationLevels []models.EducationLevel
	for rows.Next() {
		var model models.EducationLevel
		if err := rows.Scan(&model.ID, &model.Code, &model.Display, &model.CustomDisplay); err != nil {
			repo.Log.Error("educationLevelPostgresRepository.FindAll error scanning row",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrPostgresDBFindData(err)
		}
		educationLevels = append(educationLevels, model)
	}

	if err := rows.Err(); err != nil {
		repo.Log.Error("educationLevelPostgresRepository.FindAll rows iteration error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info("educationLevelPostgresRepository.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingEducationLevelCountKey, len(educationLevels)),
	)
	return educationLevels, nil
}

func (repo *educationLevelPostgresRepository) FindByID(ctx context.Context, educationLevelID string) (*models.EducationLevel, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("educationLevelPostgresRepository.FindByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEducationLevelIDKey, educationLevelID),
	)

	query := queries.GetEducationLevelByID
	var educationLevel models.EducationLevel
	err := repo.DB.QueryRowContext(ctx, query, educationLevelID).Scan(&educationLevel.ID, &educationLevel.Code, &educationLevel.Display, &educationLevel.CustomDisplay)
	if err == sql.ErrNoRows {
		repo.Log.Warn("educationLevelPostgresRepository.FindByID no rows found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEducationLevelIDKey, educationLevelID),
		)
		return nil, nil
	} else if err != nil {
		repo.Log.Error("educationLevelPostgresRepository.FindByID error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEducationLevelIDKey, educationLevelID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info("educationLevelPostgresRepository.FindByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEducationLevelIDKey, educationLevel.ID),
	)
	return &educationLevel, nil
}

func (repo *educationLevelPostgresRepository) FindByCode(ctx context.Context, educationLevelCode string) (*models.EducationLevel, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("educationLevelPostgresRepository.FindByCode called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEducationLevelCodeKey, educationLevelCode),
	)

	query := queries.GetEducationLevelByCode
	var educationLevel models.EducationLevel
	err := repo.DB.QueryRowContext(ctx, query, educationLevelCode).Scan(&educationLevel.ID, &educationLevel.Code, &educationLevel.Display, &educationLevel.CustomDisplay)
	if err == sql.ErrNoRows {
		repo.Log.Warn("educationLevelPostgresRepository.FindByCode no rows found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEducationLevelCodeKey, educationLevelCode),
		)
		return nil, nil
	} else if err != nil {
		repo.Log.Error("educationLevelPostgresRepository.FindByCode error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEducationLevelCodeKey, educationLevelCode),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info("educationLevelPostgresRepository.FindByCode succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEducationLevelIDKey, educationLevel.ID),
	)
	return &educationLevel, nil
}
