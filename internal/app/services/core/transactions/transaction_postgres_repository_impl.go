package transactions

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

type transactionPostgresRepository struct {
	DB  *sql.DB
	Log *zap.Logger
}

var (
	transactionPostgresRepositoryInstance contracts.TransactionRepository
	onceTransactionPostgresRepository     sync.Once
)

func NewTransactionPostgresRepository(db *sql.DB, logger *zap.Logger) contracts.TransactionRepository {
	onceTransactionPostgresRepository.Do(func() {
		instance := &transactionPostgresRepository{
			DB:  db,
			Log: logger,
		}
		transactionPostgresRepositoryInstance = instance
	})
	return transactionPostgresRepositoryInstance
}

func (repo *transactionPostgresRepository) FindAll(ctx context.Context) ([]models.Transaction, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("transactionPostgresRepository.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	query := queries.GetAllTransactions
	rows, err := repo.DB.QueryContext(ctx, query)
	if err != nil {
		repo.Log.Error("transactionPostgresRepository.FindAll error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
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
			repo.Log.Error("transactionPostgresRepository.FindAll error scanning row",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrPostgresDBFindData(err)
		}
		transactions = append(transactions, model)
	}

	if err := rows.Err(); err != nil {
		repo.Log.Error("transactionPostgresRepository.FindAll rows iteration error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info("transactionPostgresRepository.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingTransactionCountKey, len(transactions)),
	)
	return transactions, nil
}

func (repo *transactionPostgresRepository) FindByID(ctx context.Context, transactionID string) (*models.Transaction, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("transactionPostgresRepository.FindByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingTransactionIDKey, transactionID),
	)

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
		repo.Log.Warn("transactionPostgresRepository.FindByID no rows found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingTransactionIDKey, transactionID),
		)
		return nil, nil
	} else if err != nil {
		repo.Log.Error("transactionPostgresRepository.FindByID error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingTransactionIDKey, transactionID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info("transactionPostgresRepository.FindByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingTransactionIDKey, transaction.ID),
	)
	return &transaction, nil
}

func (repo *transactionPostgresRepository) CreateTransaction(ctx context.Context, transaction *models.Transaction) (*models.Transaction, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("transactionPostgresRepository.CreateTransaction called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

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
		repo.Log.Error("transactionPostgresRepository.CreateTransaction error executing insert",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBInsertData(err)
	}

	repo.Log.Info("transactionPostgresRepository.CreateTransaction succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingTransactionIDKey, insertedTransaction.ID),
	)
	return &insertedTransaction, nil
}

func (repo *transactionPostgresRepository) UpdateTransaction(ctx context.Context, transaction *models.Transaction) (*models.Transaction, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("transactionPostgresRepository.UpdateTransaction called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingTransactionIDKey, transaction.ID),
	)

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
		repo.Log.Error("transactionPostgresRepository.UpdateTransaction error executing update",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingTransactionIDKey, transaction.ID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBUpdateData(err)
	}

	repo.Log.Info("transactionPostgresRepository.UpdateTransaction succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingTransactionIDKey, updatedTransaction.ID),
	)
	return &updatedTransaction, nil
}

func (repo *transactionPostgresRepository) DeleteTransaction(ctx context.Context, transactionID int) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("transactionPostgresRepository.DeleteTransaction called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingTransactionIDKey, transactionID),
	)

	query := queries.DeleteTransaction
	_, err := repo.DB.ExecContext(ctx, query, transactionID)
	if err != nil {
		repo.Log.Error("transactionPostgresRepository.DeleteTransaction error executing delete",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Int(constvars.LoggingTransactionIDKey, transactionID),
			zap.Error(err),
		)
		return exceptions.ErrPostgresDBDeleteData(err)
	}

	repo.Log.Info("transactionPostgresRepository.DeleteTransaction succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingTransactionIDKey, transactionID),
	)
	return nil
}
