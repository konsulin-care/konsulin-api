package cities

import (
	"context"
	"database/sql"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/queries"
)

type cityPostgresRepository struct {
	DB *sql.DB
}

func NewCityPostgresRepository(db *sql.DB) contracts.CityRepository {
	return &cityPostgresRepository{
		DB: db,
	}
}

func (r *cityPostgresRepository) FindAll(ctx context.Context) ([]models.City, error) {
	query := queries.GetAllCities
	rows, err := r.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	defer rows.Close()

	var cities []models.City
	for rows.Next() {
		var model models.City
		if err := rows.Scan(&model.ID, &model.Name); err != nil {
			return nil, exceptions.ErrPostgresDBFindData(err)

		}
		cities = append(cities, model)
	}

	if err := rows.Err(); err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)

	}

	return cities, nil
}

func (r *cityPostgresRepository) FindByID(ctx context.Context, cityID string) (*models.City, error) {
	query := queries.GetCityByID
	var city models.City
	err := r.DB.QueryRowContext(ctx, query, cityID).Scan(&city.ID, &city.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	return &city, nil
}
