package contracts

import (
	"context"
	"konsulin-service/internal/app/models"
)

type TransactionRepository interface {
	FindAll(ctx context.Context) ([]models.Transaction, error)
	FindByID(ctx context.Context, transactionID string) (*models.Transaction, error)
	CreateTransaction(ctx context.Context, transaction *models.Transaction) (*models.Transaction, error)
	UpdateTransaction(ctx context.Context, transaction *models.Transaction) (*models.Transaction, error)
	DeleteTransaction(ctx context.Context, transactionID int) error
}
