package queries

const (
	// Insert Queries
	CreateUserQuery = `
		INSERT INTO users (
			email, gender, role_id, address, fullname, username, password, birth_date,
			patient_id, reset_token, whatsapp_otp, whatsapp_number, practitioner_id,
			profile_picture_name, educations, reset_token_expiry, whatsapp_otp_expiry, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, NOW(), NOW()
		) RETURNING id
	`

	// Select Queries
	FindByFieldQueryTemplate = `
		SELECT id, email, gender, role_id, address, fullname, username, password, birth_date,
			patient_id, reset_token, whatsapp_otp, whatsapp_number, practitioner_id,
			profile_picture_name, educations, reset_token_expiry, whatsapp_otp_expiry,
			created_at, updated_at, deleted_at
		FROM users
		WHERE %s = $1 AND deleted_at IS NULL
	`

	FindByEmailOrUsernameQuery = `
		SELECT id, email, gender, role_id, address, fullname, username, password, birth_date,
			patient_id, reset_token, whatsapp_otp, whatsapp_number, practitioner_id,
			profile_picture_name, educations, reset_token_expiry, whatsapp_otp_expiry,
			created_at, updated_at, deleted_at
		FROM users
		WHERE (email = $1 OR username = $2) AND deleted_at IS NULL
	`

	// Update Queries
	UpdateUserQuery = `
		UPDATE users
		SET email = $1, gender = $2, role_id = $3, address = $4, fullname = $5,
			username = $6, password = $7, birth_date = $8, patient_id = $9,
			reset_token = $10, whatsapp_otp = $11, whatsapp_number = $12,
			practitioner_id = $13, profile_picture_name = $14, educations = $15,
			reset_token_expiry = $16, whatsapp_otp_expiry = $17, updated_at = NOW()
		WHERE id = $18 AND deleted_at IS NULL
	`

	// Delete Queries
	DeleteByIDQuery = `
		UPDATE users
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
)
