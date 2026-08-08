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

// logPrefix namespaces every log entry emitted by this repository.
const logPrefix = "transactionPostgresRepository."

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
	repo.Log.Info(logPrefix+"FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	query := queries.GetAllTransactions
	rows, err := repo.DB.QueryContext(ctx, query)
	if err != nil {
		repo.Log.Error(logPrefix+"FindAll error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	defer func() { _ = rows.Close() }()

	var transactions []models.Transaction
	for rows.Next() {
		model, err := scanTransaction(rows)
		if err != nil {
			repo.Log.Error(logPrefix+"FindAll error scanning row",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrPostgresDBFindData(err)
		}
		transactions = append(transactions, *model)
	}

	if err := rows.Err(); err != nil {
		repo.Log.Error(logPrefix+"FindAll rows iteration error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info(logPrefix+"FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingTransactionCountKey, len(transactions)),
	)
	return transactions, nil
}

func (repo *transactionPostgresRepository) FindByID(ctx context.Context, transactionID string) (*models.Transaction, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info(logPrefix+"FindByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingTransactionIDKey, transactionID),
	)

	query := queries.GetTransactionByID
	transaction, err := scanTransaction(repo.DB.QueryRowContext(ctx, query, transactionID))
	if err == sql.ErrNoRows {
		repo.Log.Warn(logPrefix+"FindByID no rows found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingTransactionIDKey, transactionID),
		)
		return nil, nil
	} else if err != nil {
		repo.Log.Error(logPrefix+"FindByID error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingTransactionIDKey, transactionID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info(logPrefix+"FindByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingTransactionIDKey, transaction.ID),
	)
	return transaction, nil
}

func (repo *transactionPostgresRepository) CreateTransaction(ctx context.Context, transaction *models.Transaction) (*models.Transaction, error) {
	return repo.scanTransactionRow(ctx, queries.InsertTransaction, "CreateTransaction", "error executing insert", exceptions.ErrPostgresDBInsertData, nil, nil, transactionArgs(transaction)...)
}

func (repo *transactionPostgresRepository) UpdateTransaction(ctx context.Context, transaction *models.Transaction) (*models.Transaction, error) {
	return repo.scanTransactionRow(ctx, queries.UpdateTransaction, "UpdateTransaction", "error executing update", exceptions.ErrPostgresDBUpdateData,
		[]zap.Field{zap.String(constvars.LoggingTransactionIDKey, transaction.ID)},
		[]zap.Field{zap.String(constvars.LoggingTransactionIDKey, transaction.ID)},
		updateTransactionArgs(transaction)...)
}

// transactionArgs returns the transaction fields in INSERT placeholder order.
func transactionArgs(t *models.Transaction) []any {
	return []any{t.ID, t.PatientID, t.PractitionerID, t.PaymentLink, t.StatusPayment, t.Amount, t.Currency, t.SessionTotal, t.LengthMinutesPerSession, t.SessionType, t.Notes, t.RefundStatus, t.RefundAmount, t.AuditLog}
}

// updateTransactionArgs returns the transaction fields in UPDATE placeholder
// order (all columns then the ID used in the WHERE clause).
func updateTransactionArgs(t *models.Transaction) []any {
	args := transactionArgs(t)
	return append(args[1:], args[0])
}

// scanTransactionRow runs query with args, scanning the single result row into
// a Transaction and logging success/failure with the operation name. calledFields
// and errFields carry per-operation log fields.
func (repo *transactionPostgresRepository) scanTransactionRow(ctx context.Context, query string, opName, errMsg string, errFactory func(error) *exceptions.CustomError, calledFields, errFields []zap.Field, args ...any) (*models.Transaction, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	called := append([]zap.Field{zap.String(constvars.LoggingRequestIDKey, requestID)}, calledFields...)
	repo.Log.Info(logPrefix+opName+" called", called...)

	tx, err := scanTransaction(repo.DB.QueryRowContext(ctx, query, args...))
	if err != nil {
		errLog := append([]zap.Field{zap.String(constvars.LoggingRequestIDKey, requestID)}, errFields...)
		errLog = append(errLog, zap.Error(err))
		repo.Log.Error(logPrefix+opName+" "+errMsg, errLog...)
		return nil, errFactory(err)
	}
	repo.Log.Info(logPrefix+opName+" succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingTransactionIDKey, tx.ID),
	)
	return tx, nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanTransaction scans a single transaction row into a Transaction.
func scanTransaction(row rowScanner) (*models.Transaction, error) {
	var tx models.Transaction
	err := row.Scan(
		&tx.ID, &tx.PatientID, &tx.PractitionerID, &tx.PaymentLink, &tx.StatusPayment,
		&tx.Amount, &tx.Currency, &tx.CreatedAt, &tx.UpdatedAt, &tx.SessionTotal,
		&tx.LengthMinutesPerSession, &tx.SessionType, &tx.Notes, &tx.RefundStatus,
		&tx.RefundAmount, &tx.AuditLog,
	)
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (repo *transactionPostgresRepository) DeleteTransaction(ctx context.Context, transactionID int) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info(logPrefix+"DeleteTransaction called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingTransactionIDKey, transactionID),
	)

	query := queries.DeleteTransaction
	_, err := repo.DB.ExecContext(ctx, query, transactionID)
	if err != nil {
		repo.Log.Error(logPrefix+"DeleteTransaction error executing delete",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Int(constvars.LoggingTransactionIDKey, transactionID),
			zap.Error(err),
		)
		return exceptions.ErrPostgresDBDeleteData(err)
	}

	repo.Log.Info(logPrefix+"DeleteTransaction succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingTransactionIDKey, transactionID),
	)
	return nil
}
