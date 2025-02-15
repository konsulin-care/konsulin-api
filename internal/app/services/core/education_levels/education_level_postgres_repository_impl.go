package educationLevels

import (
	"context"
	"database/sql"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
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
	query := queries.GetAllEducationLevels
	rows, err := repo.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	defer rows.Close()

	var educationLevels []models.EducationLevel
	for rows.Next() {
		var model models.EducationLevel
		if err := rows.Scan(
			&model.ID,
			&model.Code,
			&model.Display,
			&model.CustomDisplay,
		); err != nil {
			return nil, exceptions.ErrPostgresDBFindData(err)
		}
		educationLevels = append(educationLevels, model)
	}

	if err := rows.Err(); err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	return educationLevels, nil
}

func (repo *educationLevelPostgresRepository) FindByID(ctx context.Context, educationLevelID string) (*models.EducationLevel, error) {
	query := queries.GetEducationLevelByID
	var educationLevel models.EducationLevel
	err := repo.DB.QueryRowContext(ctx, query, educationLevelID).Scan(&educationLevel.ID, &educationLevel.Code, &educationLevel.Display, &educationLevel.CustomDisplay)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	return &educationLevel, nil
}

func (repo *educationLevelPostgresRepository) FindByCode(ctx context.Context, educationLevelCode string) (*models.EducationLevel, error) {
	query := queries.GetEducationLevelByCode
	var educationLevel models.EducationLevel
	err := repo.DB.QueryRowContext(ctx, query, educationLevelCode).Scan(&educationLevel.ID, &educationLevel.Code, &educationLevel.Display, &educationLevel.CustomDisplay)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	return &educationLevel, nil
}
