package queries

const (
	GetAllTransactions = `
		SELECT 
			id, 
			patient_id, 
			practitioner_id, 
			payment_link, 
			status_payment, 
			amount, 
			currency, 
			created_at, 
			updated_at, 
			session_total, 
			length_minutes_per_session, 
			session_type, 
			notes, 
			refund_status, 
			refund_amount, 
			audit_log 
		FROM transactions
	`

	GetTransactionByID = `
		SELECT 
			id, 
			patient_id, 
			practitioner_id, 
			payment_link, 
			status_payment, 
			amount, 
			currency, 
			created_at, 
			updated_at, 
			session_total, 
			length_minutes_per_session, 
			session_type, 
			notes, 
			refund_status, 
			refund_amount, 
			audit_log 
		FROM transactions 
		WHERE id = $1
	`

	GetTransactionByPatientID = `
		SELECT 
			id, 
			patient_id, 
			practitioner_id, 
			payment_link, 
			status_payment, 
			amount, 
			currency, 
			created_at, 
			updated_at, 
			session_total, 
			length_minutes_per_session, 
			session_type, 
			notes, 
			refund_status, 
			refund_amount, 
			audit_log 
		FROM transactions 
		WHERE patient_id = $1
	`

	InsertTransaction = `
		INSERT INTO transactions (
			id,
			patient_id,
			practitioner_id,
			payment_link,
			status_payment,
			amount,
			currency,
			session_total,
			length_minutes_per_session,
			session_type,
			notes,
			refund_status,
			refund_amount,
			audit_log,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), NOW())
		RETURNING 
			id,
			patient_id,
			practitioner_id,
			payment_link,
			status_payment,
			amount,
			currency,
			created_at,
			updated_at,
			session_total,
			length_minutes_per_session,
			session_type,
			notes,
			refund_status,
			refund_amount,
			audit_log
	`

	UpdateTransaction = `
		UPDATE transactions
		SET 
			patient_id = $1,
			practitioner_id = $2,
			payment_link = $3,
			status_payment = $4,
			amount = $5,
			currency = $6,
			session_total = $7,
			length_minutes_per_session = $8,
			session_type = $9,
			notes = $10,
			refund_status = $11,
			refund_amount = $12,
			audit_log = $13,
			updated_at = NOW()
		WHERE id = $14
		RETURNING 
			id,
			patient_id,
			practitioner_id,
			payment_link,
			status_payment,
			amount,
			currency,
			created_at,
			updated_at,
			session_total,
			length_minutes_per_session,
			session_type,
			notes,
			refund_status,
			refund_amount,
			audit_log
	`

	DeleteTransaction = `
		DELETE FROM transactions 
		WHERE id = $1
	`
)
