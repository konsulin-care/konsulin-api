package queries

const (
	GetAllEducationLevels = `
		SELECT id, code, display, custom_display 
		FROM education_levels
	`

	GetEducationLevelByID = `
		SELECT id, code, display, custom_display 
		FROM education_levels 
		WHERE id = $1
	`

	GetEducationLevelByCode = `
		SELECT id, code, display, custom_display 
		FROM education_levels 
		WHERE code = $1
	`

	InsertEducationLevel = `
		INSERT INTO education_levels (code, display, system, definition, internal_id, status, custom_display) 
		VALUES ($1, $2, $3, $4, $5, $6, $7) 
		RETURNING id
	`

	UpdateEducationLevel = `
		UPDATE education_levels 
		SET code = $1, display = $2, system = $3, definition = $4, internal_id = $5, status = $6, custom_display = $7 
		WHERE id = $8
	`

	DeleteEducationLevel = `
		DELETE FROM education_levels 
		WHERE id = $1
	`
)
