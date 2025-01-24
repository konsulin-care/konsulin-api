package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachJournalRouter(router chi.Router, middlewares *middlewares.Middlewares, journalController *controllers.JournalController) {
	router.With(middlewares.Authenticate).Post("/", journalController.CreateJournal)
	router.With(middlewares.Authenticate).Put("/{journal_id}", journalController.UpdateJournalByID)
	router.With(middlewares.Authenticate).Get("/{journal_id}", journalController.FindJournalByID)
	router.With(middlewares.Authenticate).Delete("/{journal_id}", journalController.DeleteJournalByID)
}
