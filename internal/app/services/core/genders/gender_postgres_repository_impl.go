package genders

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
	query := queries.GetAllGenders
	rows, err := repo.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	defer rows.Close()

	var genders []models.Gender
	for rows.Next() {
		var model models.Gender
		if err := rows.Scan(
			&model.ID,
			&model.Code,
			&model.Display,
			&model.CustomDisplay,
		); err != nil {
			return nil, exceptions.ErrPostgresDBFindData(err)
		}
		genders = append(genders, model)
	}

	if err := rows.Err(); err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	return genders, nil
}

func (repo *genderPostgresRepository) FindByID(ctx context.Context, genderID string) (*models.Gender, error) {
	query := queries.GetGenderByID
	var gender models.Gender
	err := repo.DB.
		QueryRowContext(ctx, query, genderID).
		Scan(&gender.ID, &gender.Code, &gender.Display, &gender.CustomDisplay)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	return &gender, nil
}

func (repo *genderPostgresRepository) FindByCode(ctx context.Context, genderCode string) (*models.Gender, error) {
	query := queries.GetGenderByCode
	var gender models.Gender
	err := repo.DB.QueryRowContext(ctx, query, genderCode).Scan(&gender.ID, &gender.Code, &gender.Display, &gender.CustomDisplay)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	return &gender, nil
}
