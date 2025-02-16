package genders

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

type genderPostgresRepository struct {
	DB  *sql.DB
	Log *zap.Logger
}

var (
	genderPostgresRepositoryInstance contracts.GenderRepository
	onceGenderPostgresRepository     sync.Once
)

func NewGenderPostgresRepository(db *sql.DB, logger *zap.Logger) contracts.GenderRepository {
	onceGenderPostgresRepository.Do(func() {
		instance := &genderPostgresRepository{
			DB:  db,
			Log: logger,
		}
		genderPostgresRepositoryInstance = instance
	})
	return genderPostgresRepositoryInstance
}

func (repo *genderPostgresRepository) FindAll(ctx context.Context) ([]models.Gender, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("genderPostgresRepository.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	query := queries.GetAllGenders
	rows, err := repo.DB.QueryContext(ctx, query)
	if err != nil {
		repo.Log.Error("genderPostgresRepository.FindAll error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	defer rows.Close()

	var genders []models.Gender
	for rows.Next() {
		var model models.Gender
		if err := rows.Scan(&model.ID, &model.Code, &model.Display, &model.CustomDisplay); err != nil {
			repo.Log.Error("genderPostgresRepository.FindAll error scanning row",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrPostgresDBFindData(err)
		}
		genders = append(genders, model)
	}

	if err := rows.Err(); err != nil {
		repo.Log.Error("genderPostgresRepository.FindAll rows iteration error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info("genderPostgresRepository.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingGenderCountKey, len(genders)),
	)
	return genders, nil
}

func (repo *genderPostgresRepository) FindByID(ctx context.Context, genderID string) (*models.Gender, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("genderPostgresRepository.FindByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingGenderIDKey, genderID),
	)

	query := queries.GetGenderByID
	var gender models.Gender
	err := repo.DB.QueryRowContext(ctx, query, genderID).Scan(&gender.ID, &gender.Code, &gender.Display, &gender.CustomDisplay)
	if err == sql.ErrNoRows {
		repo.Log.Warn("genderPostgresRepository.FindByID no rows found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingGenderIDKey, genderID),
		)
		return nil, nil
	} else if err != nil {
		repo.Log.Error("genderPostgresRepository.FindByID error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingGenderIDKey, genderID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info("genderPostgresRepository.FindByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingGenderIDKey, gender.ID),
	)
	return &gender, nil
}

func (repo *genderPostgresRepository) FindByCode(ctx context.Context, genderCode string) (*models.Gender, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("genderPostgresRepository.FindByCode called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingGenderCodeKey, genderCode),
	)

	query := queries.GetGenderByCode
	var gender models.Gender
	err := repo.DB.QueryRowContext(ctx, query, genderCode).Scan(&gender.ID, &gender.Code, &gender.Display, &gender.CustomDisplay)
	if err == sql.ErrNoRows {
		repo.Log.Warn("genderPostgresRepository.FindByCode no rows found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingGenderCodeKey, genderCode),
		)
		return nil, nil
	} else if err != nil {
		repo.Log.Error("genderPostgresRepository.FindByCode error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingGenderCodeKey, genderCode),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info("genderPostgresRepository.FindByCode succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingGenderIDKey, gender.ID),
	)
	return &gender, nil
}
