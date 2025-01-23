package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type JournalUsecase interface {
	CreateJournal(ctx context.Context, request *requests.CreateJournal) (*responses.Journal, error)
	UpdateJournal(ctx context.Context, request *requests.UpdateJournal) (*responses.Journal, error)
	FindJournalByID(ctx context.Context, request *requests.FindJournalByID) (*responses.Journal, error)
	DeleteJournalByID(ctx context.Context, request *requests.DeleteJournalByID) error
}
