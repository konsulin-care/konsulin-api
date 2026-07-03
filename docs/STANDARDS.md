# Coding Standards

## Clean Architecture

- **Dependency direction**: delivery → services → contracts → drivers (inward)
- **No cyclic dependencies** between layers
- **Constructor injection** for all dependencies — no `init()`, no global state
- **Interface segregation**: one interface per concern (see `contracts/` — 24 files)

## Middleware Chain (Fixed Order)

```go
1. RequestID     // Trace context
2. Logging       // Zap structured logs
3. BodyBuffer    // Read body once
4. CORS          // Chi CORS middleware
5. SuperTokens   // Session verification
6. APIKey        // Superadmin key validation
7. Session       // User context enrichment
8. RateLimit     // Token bucket (Redis)
9. ErrorHandler  // Consistent error responses
```

Do NOT reorder or insert new middlewares without updating the chain.

## FHIR HTTP Client

**All FHIR HTTP requests MUST use `fhir_http_client.FHIRHTTPClient.Do()`.**

Rules:
- Never create raw `http.Client` for FHIR calls
- Never set `Content-Type` headers inline (client sets `application/fhir+json`)
- Never check status codes manually (client validates and parses `OperationOutcome`)
- URL construction: `baseUrl + resourceType + "/" + id` (baseUrl has trailing slash)

## Error Handling

- Controllers return typed errors from `internal/pkg/exceptions/`
- Each exception type maps to an HTTP status code
- FHIR errors returned as `OperationOutcome` resources
- Never expose internal error details to clients

## RBAC Policy Conventions

- Policies in `resources/rbac_policy.csv` — one rule per line
- Format: `p, role, method, path`
- Role hierarchy via `g, child_role, parent_role` (e.g. `g, Patient, Guest`)
- Use Casbin `pathMatch()` for wildcard routes in the model
- API keys bypass RBAC (superadmin only)

## Logging

- Use `go.uber.org/zap` — structured, never `fmt.Println` or `log.Println`
- Every request gets a unique `request_id` via middleware
- Log context: request ID, method, path, status code, duration, user agent, remote addr

## Repository Pattern

- Every data access goes through an interface in `contracts/`
- Interface implementations are injected at service construction time
- FHIR resources accessed via `fhir_spark/` service wrappers
- Mock-friendly interfaces for testing

## Testing

- Standard `testing` package (no third-party test frameworks)
- Interface-based mocking for dependencies
- Test files co-located with source files (`*_test.go`)
- Focus on service-layer tests with mocked contracts
- RBAC policy tests via Casbin enforcer unit tests
