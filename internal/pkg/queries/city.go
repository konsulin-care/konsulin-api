package queries

const (
	GetAllCities = "SELECT id, name FROM cities"

	GetCityByID = "SELECT id, name FROM cities WHERE id = $1"

	InsertCity = "INSERT INTO cities (name) VALUES ($1) RETURNING id"
	UpdateCity = "UPDATE cities SET name = $1 WHERE id = $2"
	DeleteCity = "DELETE FROM cities WHERE id = $1"
)
