package transactions

import (
	"context"
	"database/sql"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/queries"
)

type transactionPostgresRepository struct {
	DB *sql.DB
}

func NewTransactionPostgresRepository(db *sql.DB) contracts.TransactionRepository {
	return &transactionPostgresRepository{
		DB: db,
	}
}
func (repo *transactionPostgresRepository) FindAll(ctx context.Context) ([]models.Transaction, error) {
	query := queries.GetAllTransactions
	rows, err := repo.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		var model models.Transaction
		if err := rows.Scan(
			&model.ID,
			&model.PatientID,
			&model.PractitionerID,
			&model.PaymentLink,
			&model.StatusPayment,
			&model.Amount,
			&model.Currency,
			&model.CreatedAt,
			&model.UpdatedAt,
			&model.SessionTotal,
			&model.LengthMinutesPerSession,
			&model.SessionType,
			&model.Notes,
			&model.RefundStatus,
			&model.RefundAmount,
			&model.AuditLog,
		); err != nil {
			return nil, exceptions.ErrPostgresDBFindData(err)
		}
		transactions = append(transactions, model)
	}

	if err := rows.Err(); err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	return transactions, nil
}

func (repo *transactionPostgresRepository) FindByID(ctx context.Context, transactionID string) (*models.Transaction, error) {
	query := queries.GetTransactionByID
	var transaction models.Transaction
	err := repo.DB.QueryRowContext(ctx, query, transactionID).Scan(
		&transaction.ID,
		&transaction.PatientID,
		&transaction.PractitionerID,
		&transaction.PaymentLink,
		&transaction.StatusPayment,
		&transaction.Amount,
		&transaction.Currency,
		&transaction.CreatedAt,
		&transaction.UpdatedAt,
		&transaction.SessionTotal,
		&transaction.LengthMinutesPerSession,
		&transaction.SessionType,
		&transaction.Notes,
		&transaction.RefundStatus,
		&transaction.RefundAmount,
		&transaction.AuditLog,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	return &transaction, nil
}

func (repo *transactionPostgresRepository) CreateTransaction(ctx context.Context, transaction *models.Transaction) (*models.Transaction, error) {
	query := queries.InsertTransaction
	var insertedTransaction models.Transaction
	err := repo.DB.QueryRowContext(ctx, query,
		transaction.ID,
		transaction.PatientID,
		transaction.PractitionerID,
		transaction.PaymentLink,
		transaction.StatusPayment,
		transaction.Amount,
		transaction.Currency,
		transaction.SessionTotal,
		transaction.LengthMinutesPerSession,
		transaction.SessionType,
		transaction.Notes,
		transaction.RefundStatus,
		transaction.RefundAmount,
		transaction.AuditLog,
	).Scan(
		&insertedTransaction.ID,
		&insertedTransaction.PatientID,
		&insertedTransaction.PractitionerID,
		&insertedTransaction.PaymentLink,
		&insertedTransaction.StatusPayment,
		&insertedTransaction.Amount,
		&insertedTransaction.Currency,
		&insertedTransaction.CreatedAt,
		&insertedTransaction.UpdatedAt,
		&insertedTransaction.SessionTotal,
		&insertedTransaction.LengthMinutesPerSession,
		&insertedTransaction.SessionType,
		&insertedTransaction.Notes,
		&insertedTransaction.RefundStatus,
		&insertedTransaction.RefundAmount,
		&insertedTransaction.AuditLog,
	)
	if err != nil {
		return nil, exceptions.ErrPostgresDBInsertData(err)
	}
	return &insertedTransaction, nil
}

func (repo *transactionPostgresRepository) UpdateTransaction(ctx context.Context, transaction *models.Transaction) (*models.Transaction, error) {
	query := queries.UpdateTransaction
	var updatedTransaction models.Transaction

	err := repo.DB.QueryRowContext(ctx, query,
		transaction.PatientID,
		transaction.PractitionerID,
		transaction.PaymentLink,
		transaction.StatusPayment,
		transaction.Amount,
		transaction.Currency,
		transaction.SessionTotal,
		transaction.LengthMinutesPerSession,
		transaction.SessionType,
		transaction.Notes,
		transaction.RefundStatus,
		transaction.RefundAmount,
		transaction.AuditLog,
		transaction.ID,
	).Scan(
		&updatedTransaction.ID,
		&updatedTransaction.PatientID,
		&updatedTransaction.PractitionerID,
		&updatedTransaction.PaymentLink,
		&updatedTransaction.StatusPayment,
		&updatedTransaction.Amount,
		&updatedTransaction.Currency,
		&updatedTransaction.CreatedAt,
		&updatedTransaction.UpdatedAt,
		&updatedTransaction.SessionTotal,
		&updatedTransaction.LengthMinutesPerSession,
		&updatedTransaction.SessionType,
		&updatedTransaction.Notes,
		&updatedTransaction.RefundStatus,
		&updatedTransaction.RefundAmount,
		&updatedTransaction.AuditLog,
	)
	if err != nil {
		return nil, exceptions.ErrPostgresDBUpdateData(err)
	}
	return &updatedTransaction, nil
}

func (repo *transactionPostgresRepository) DeleteTransaction(ctx context.Context, transactionID int) error {
	query := queries.DeleteTransaction
	_, err := repo.DB.ExecContext(ctx, query, transactionID)
	if err != nil {
		return exceptions.ErrPostgresDBDeleteData(err)
	}
	return nil
}
