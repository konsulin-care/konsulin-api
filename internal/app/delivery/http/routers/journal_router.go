package routers

import (
	"konsulin-service/internal/app/delivery/http/controllers"
	"konsulin-service/internal/app/delivery/http/middlewares"

	"github.com/go-chi/chi/v5"
)

func attachJournalRouter(router chi.Router, middlewares *middlewares.Middlewares, journalController *controllers.JournalController) {
	router.Get("/", journalController.CreateJournal)
	router.Put("/{journal_id}", journalController.UpdateJournalByID)
	router.Get("/{journal_id}", journalController.FindJournalByID)
	router.Delete("/{journal_id}", journalController.DeleteJournalByID)
}
