# FHIR HTTP Client Pattern

## Rule

**All FHIR HTTP requests MUST go through `fhir_http_client.FHIRHTTPClient.Do()`.**

Do not create raw `http.Client`, do not set `Content-Type` headers inline, do not check status codes manually. The shared client handles all common boilerplate.

## What the shared client handles

- Request creation with context propagation
- `Content-Type: application/fhir+json` header
- HTTP execution
- Status code validation (rejects `<200` and `>=300`)
- `OperationOutcome` parsing on error responses
- Body reading and returning raw `[]byte`

## How to use

```go
import "konsulin-service/internal/pkg/fhir_http_client"

// Instantiate (typically once via constructor injection)
fhirClient := fhir_http_client.New(logger)

// Build the URL: base + resourceType/id (no extra slash before resource type)
url := config.FHIR.BaseUrl + "PractitionerRole" + "/" + id

// Make the request
body, err := fhirClient.Do(ctx, http.MethodGet, url, nil)
if err != nil {
    return nil, fmt.Errorf("fetch practitioner role: %w", err)
}

// Unmarshal into the target FHIR DTO
var pr fhir_dto.PractitionerRole
return &pr, json.Unmarshal(body, &pr)
```

## URL construction — avoid double slashes

The config `FHIR.BaseUrl` has a trailing slash (e.g. `http://localhost:8080/fhir/`).

**Correct** (follows the existing client pattern):
```go
url := baseUrl + resourceType + "/" + resourceID
// → "http://localhost:8080/fhir/PractitionerRole/pr-123"
```

**Wrong** (double slash):
```go
url := fmt.Sprintf("%s/%s/%s", baseUrl, resourceType, id)
// → "http://localhost:8080/fhir//PractitionerRole/pr-123"
```

## Migration

Existing FHIR client files (`internal/app/services/fhir_spark/*/`) still work with their own HTTP logic. Migrate them to use `FHIRHTTPClient.Do()` incrementally when modifying each file — the shared client is intentionally compatible (returns `[]byte`, callers still unmarshal).
