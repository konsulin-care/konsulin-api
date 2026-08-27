# internal/app/ — Application Layer

Clean Architecture layers, organized by concern.

## Layers (top to bottom)

### `internal/app/config/`
Configuration structs loaded from `.env` + `config.*.yaml`. Two main structs:
- `InternalConfig` — app, FHIR, JWT, mailer, RabbitMQ, Konsulin, SuperToken, payment gateway, service pricing, webhook, Xendit
- `DriverConfig` — infrastructure components

### `internal/app/contracts/`
Interface definitions (24 files). Key contracts: auth, user, patient, practitioner, slot, payment, webhook, invoice, locker, mailer, redis, storage, whatsapp, session, role, etc. All services depend on these interfaces, not concrete implementations.

### `internal/app/services/`
Three service groups:
- **`core/`** — Auth, organization, payments, roles, session, slot, transactions, users, webhook
- **`fhir_spark/`** — FHIR resource clients: bundle, invoices, observations, organizations, patients, persons, practitioner_role, practitioners, questionnaire_responses, schedules, service_requests, slots
- **`shared/`** — jwtmanager, locker, mailer, payment_gateway, ratelimiter, redis, storage, webhookqueue, whatsapp

### `internal/app/delivery/http/`
- **`controllers/`** — 9 HTTP controllers: auth, clinician, organization, patient, payment, role, schedule, user, webhook
- **`middlewares/`** — 9 middlewares in fixed order (see ARCHITECTURE.md): core, auth, api_key, body_buffer, error_handler, proxy, rate_limit, session, logging
- **`routers/`** — Route setup: CORS, middleware chain, API v1, FHIR bridge, terminology proxy
- **`handlers/`** — HTTP handler utilities

### `internal/app/drivers/`
Infrastructure wrappers:
- **`database/`** — DB connection drivers
- **`logger/`** — Zap logger wrapper
- **`messaging/`** — RabbitMQ wrapper
- **`thirdparty/`** — External service integration

### `internal/app/models/`
Domain entity structs with BSON tags (legacy MongoDB compatibility): User, Patient, Practitioner, etc.

---

## Key Pattern

```
Controller (HTTP) → Service Interface (contracts) → Service Impl (services/) → FHIR Client / Driver
```

All dependencies are injected via constructor functions — no global state, no init() patterns.

## See Also

- [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md) — Middleware chain order, request flow
- [docs/STANDARDS.md](../../docs/STANDARDS.md) — Clean Architecture conventions
