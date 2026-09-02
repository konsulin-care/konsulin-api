package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// magiclinkStore holds in-memory state for magic link deliveries.
// Each entry maps an org prefix to a list of messages (keyed by email prefix).
type magiclinkStore struct {
	mu        sync.RWMutex
	nextMsgID int
	inboxes   map[string][]magiclinkMessage // org -> messages
}

type magiclinkMessage struct {
	ID         string
	SecondsAgo int
	Links      []string
}

var store = &magiclinkStore{
	nextMsgID: 1,
	inboxes:   make(map[string][]magiclinkMessage),
}

// handleSendMagiclink receives the app's webhook payload {url, exp, email}
// and files the link in the mailbox of the org derived from the email prefix.
func handleSendMagiclink(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		URL   string `json:"url"`
		Exp   int    `json:"exp"`
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}

	org := extractOrg(payload.Email)
	msgID := fmt.Sprintf("msg_%d", store.nextMsgID)
	store.nextMsgID++

	store.mu.Lock()
	store.inboxes[org] = append(store.inboxes[org], magiclinkMessage{
		ID:         msgID,
		SecondsAgo: 0,
		Links:      []string{payload.URL},
	})
	store.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "magic link stubbed",
	})
}

// handleInboxListing returns all messages for the given org, shaped like the
// Mailinator public inbox API: {msgs: [{id, seconds_ago}]}.
func handleInboxListing(w http.ResponseWriter, r *http.Request) {
	org := r.PathValue("org")

	store.mu.RLock()
	msgs := store.inboxes[org]
	store.mu.RUnlock()

	type msgJSON struct {
		ID         string `json:"id"`
		SecondsAgo int    `json:"seconds_ago"`
	}
	out := make([]msgJSON, len(msgs))
	for i, m := range msgs {
		out[i] = msgJSON{ID: m.ID, SecondsAgo: m.SecondsAgo}
	}

	writeJSON(w, http.StatusOK, map[string]any{"msgs": out})
}

// handleMessageLinks returns the magic-link URLs for a given message ID.
func handleMessageLinks(w http.ResponseWriter, r *http.Request) {
	msgID := r.PathValue("id")

	store.mu.RLock()
	var links []string
	for _, msgs := range store.inboxes {
		for _, m := range msgs {
			if m.ID == msgID {
				links = m.Links
			}
		}
	}
	store.mu.RUnlock()

	if links == nil {
		links = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"links": links})
}

// extractOrg pulls the local-part (before @) from an email address to use as
// the org key in the mailbox.
func extractOrg(email string) string {
	for i, c := range email {
		if c == '@' {
			return email[:i]
		}
	}
	return email
}

// writeJSON encodes payload as JSON and writes it to w.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
