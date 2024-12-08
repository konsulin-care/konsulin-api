package queries

const (
	GetAllGenders = `
		SELECT id, code, display, custom_display 
		FROM genders
	`

	GetGenderByID = `
		SELECT id, code, display, custom_display 
		FROM genders 
		WHERE id = $1
	`

	GetGenderByCode = `
		SELECT id, code, display, custom_display 
		FROM genders 
		WHERE code = $1
	`

	InsertGender = `
		INSERT INTO genders (code, display, system, definition, custom_display) 
		VALUES ($1, $2, $3, $4, $5) 
		RETURNING id
	`

	UpdateGender = `
		UPDATE genders 
		SET code = $1, display = $2, system = $3, definition = $4, custom_display = $5 
		WHERE id = $6
	`

	DeleteGender = `
		DELETE FROM genders 
		WHERE id = $1
	`
)
