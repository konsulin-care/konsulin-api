package base

import (
	"konsulin-service/internal/pkg/fhir_http_client"
	"sync"

	"go.uber.org/zap"
)

// ResourceClient is a base struct embedded by all FHIR resource client implementations.
// It provides the shared fields (BaseUrl, Log, Client) and a common constructor.
type ResourceClient struct {
	BaseUrl string
	Log     *zap.Logger
	Client  *fhir_http_client.FHIRHTTPClient
}

// New creates a ResourceClient with the given base FHIR URL, resource path suffix,
// and logger. The BaseUrl is set to baseUrl + resourcePath.
func New(baseUrl, resourcePath string, logger *zap.Logger) *ResourceClient {
	return &ResourceClient{
		BaseUrl: baseUrl + resourcePath,
		Log:     logger,
		Client:  fhir_http_client.New(logger),
	}
}

// Singleton is a generic helper for sync.Once-based singleton access.
// Usage: var (instance T; once sync.Once)
func Singleton[T any](once *sync.Once, instance *T, factory func() T) T {
	once.Do(func() {
		*instance = factory()
	})
	return *instance
}
