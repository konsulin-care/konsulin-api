# internal/pkg/ — Shared Packages

Reusable packages shared across the application. No application-layer logic here — these are pure utility and infrastructure wrappers.

## Packages

### `fhir_http_client/` — FHIR HTTP Client

**All FHIR HTTP requests MUST go through `FHIRHTTPClient.Do()`** — never raw `http.Client`.

The shared client handles:
- Context propagation
- `Content-Type: application/fhir+json` header
- HTTP execution
- Status code validation (rejects `<200` and `>=300`)
- `OperationOutcome` parsing on error responses
- Body reading, returning raw `[]byte`

Usage:
```go
fhirClient := fhir_http_client.New(logger)
url := config.FHIR.BaseUrl + "PractitionerRole" + "/" + id
body, err := fhirClient.Do(ctx, http.MethodGet, url, nil)
```

**⚠ URL construction — avoid double slashes**: `BaseUrl` has a trailing slash (e.g. `http://localhost:8080/fhir/`). Concatenate directly: `baseUrl + resourceType + "/" + resourceID`. Do NOT use `fmt.Sprintf("%s/%s/%s", ...)` — that produces `/fhir//PractitionerRole`.

Migration: existing `fhir_spark/*/` files still use their own HTTP logic. Migrate incrementally when modifying each file.

### `fhir_dto/` — FHIR DTOs

Typed structs mirroring FHIR R4 resources: Patient, Practitioner, PractitionerRole, Schedule, Slot, Organization, Person, Observation, QuestionnaireResponse, Invoice, ServiceRequest, Bundle, etc.

### `dto/` — Application DTOs

Request/response data transfer objects for API endpoints. Subdirectories for specific use cases.

### `constvars/` — Constants & Context Keys

Shared constants: context keys (`CONTEXT_REQUEST_ID_KEY`), logging field names, HTTP header names, role strings, timeouts. Single source of truth to avoid magic strings.

### `exceptions/` — Error Types

Custom error types with HTTP status mapping. Centralizes error handling so controllers return consistent error responses.

### `queries/` — SQL Query Constants

Raw SQL query strings (for SuperTokens PostgreSQL / legacy DB access). Keeps SQL out of service files.

### `utils/` — Utility Functions

General helpers: string manipulation, slice operations, time formatting, pointer helpers, cryptographic utilities.

---

## See Also

- [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md) — How FHIR client fits in the proxy flow
- [docs/STANDARDS.md](../../docs/STANDARDS.md) — FHIR client usage rules, error handling conventions
