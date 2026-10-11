package main

import (
	"strings"

	"konsulin-service/internal/app/config"

	xendit "github.com/xendit/xendit-go/v7"
)

func newXenditClient(cfg config.AppXendit) *xendit.APIClient {
	client := xendit.NewClient(cfg.APIKey)
	if baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"); baseURL != "" {
		client.GetConfig().(*xendit.Configuration).Servers = xendit.ServerConfigurations{{URL: baseURL}}
	}
	return client
}
