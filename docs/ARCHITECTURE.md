# Architecture

## System Overview

Clients → **API Gateway** (auth → RBAC → routing) → Internal (Blaze FHIR, webhooks) / External (Xendit)

## High-Level Architecture

```
Request → RequestID → Logging → BodyBuffer → CORS
  → SuperTokens (auth) → APIKey (superadmin)
  → Session → RateLimit → ErrorHandler → Router
```

Routes:
- `/api/v1/*` — Business API (auth, payments, schedules, webhooks)
- `/fhir/*` — FHIR proxy to Blaze server with RBAC filtering
- `/api/v1/tx` — Terminology proxy

## Clean Architecture Layers

| Layer | Directory | Responsibility |
|---|---|---|
| Delivery | `internal/app/delivery/http/` | HTTP handlers, controllers, middleware |
| Services | `internal/app/services/` | Business logic (core/fhir_spark/shared) |
| Contracts | `internal/app/contracts/` | Interface definitions |
| Drivers | `internal/app/drivers/` | Infrastructure wrappers (Redis, RabbitMQ, logger) |
| Config | `internal/app/config/` | Environment + YAML config loading |

## Request Flow

### FHIR Proxy Flow
```
/fhir/* → Auth Middleware → RBAC Filter → Proxy Bridge → Blaze Server → Response Filter
```

### Payment Flow
```
/pay/* → Auth Middleware → RBAC Filter → Payment Service → Xendit → Transaction Storage
```

### Auth Flow
```
/auth/* → SuperTokens Middleware → Session Validation → RBAC Check → Response
```

## Authorization (RBAC)

6 roles: Guest, Patient, Practitioner, Clinic Admin, Researcher, Superadmin

Casbin policy model: `sub, method, path` matching with role inheritance.
Policies in [`resources/rbac_policy.csv`](../resources/rbac_policy.csv).
Model in [`resources/rbac_model.conf`](../resources/rbac_model.conf).

Clinic Admin and Researcher are practitioners with a specialized
PractitionerRole coding (`Administrative healthcare staff` = SNOMED 224608005,
`researcher` = HL7 practitioner-role). Sessions holding those roles resolve to
their Practitioner FHIR identity; the ownership engine's code-conditioned rules
do the role gating, so Casbin subjects stay untouched.

## Ownership Model

Ownership is enforced by a single declarative spec in
[`internal/pkg/ownership`](../internal/pkg/ownership): one `ResourceRule` per
FHIR resource type is the single source of truth for read ownership, write
body validation, and search-query scoping. It replaced the legacy
`RequiresPatientOwnership` / `RequiresPractitionerOwnership` / `IsPublicResource`
maps and the scattered per-type switches in the auth/proxy middlewares.

### Scopes

| Scope | Meaning |
|---|---|
| `Public` | Readable by any caller without ownership proof (catalog/system resources) |
| `Patient` | Owned by a patient identity via its reference elements |
| `Practitioner` | Owned by a practitioner identity via its references |
| `Person` | Owned by a Person identity (no session resolves to Person yet; reads fail closed) |
| `Shared` | Owned by any identity its refs name (Communication sender/recipient, Encounter subject/participant) |
| `Internal` | Gateway-only; denied on the FHIR proxy |

### Rules

Each rule declares:

- `Refs` — gjson paths (e.g. `subject.reference`, `participant.#.actor.reference`)
  that confer ownership when they reference an owned Patient / Practitioner /
  PractitionerRole / Person id.
- `WriteRefs` — body references a write must carry (strict on PUT, lenient on
  POST); `WriteBypassCodes` exempt code holders (e.g. clinic admin managing
  PractitionerRole for others).
- `SearchParams` / `SearchAllowances` — query params that scope entry-level
  searches; code-conditioned allowances implement researcher scoped reads
  (Communication by topic/sender/recipient, QuestionnaireResponse by identifier).
- `PractitionerRoleCodings` — restrict the whole rule to callers holding a
  coding; `CodeAllow` permits non-owner reads (researcher Communications,
  clinic-admin Practitioner directory) subject to `RedactKeep` field reduction.
- `WriteCheckerName` — named I/O write strategies (schedule, invoice, slot,
  questionnaire_response) preserved from the legacy per-type logic.
- `Checker` — read-path strategies (invoice public-if-whitelisted-actors).

### Behavior

- **Fail-closed flip**: resource types with no rule deny at runtime. A
  completeness test fails on any type reachable via `rbac_policy.csv` without a
  rule.
- **Reads** are filtered post-response (bundle entries, single resources,
  Communication redaction); GET requests bypass pre-request ownership checks.
- **Writes** (POST/PUT bodies) are validated pre-request against `WriteRefs`.
- **Conformance**: a test asserts every Patient/Practitioner ref exists in the
  vendored FHIR R4 CompartmentDefinitions
  (`resources/fhir/CompartmentDefinition-{patient,practitioner}.json`), with a
  documented `nonCompartmentRefs` allowlist for the few exceptions.

### Adding a new resource type

1. Add one `ResourceRule` entry in `internal/pkg/ownership/rules.go` (derive
   reference paths from the FHIR compartment; see the conformance test).
2. Add the policy row(s) in `resources/rbac_policy.csv`.
3. If the write path needs I/O or legacy quirks, name a `WriteCheckerName` and
   implement it in the middleware's `validateResourceOwnership` dispatch.
4. Add table-driven tests (read/write/search fixtures) in the ownership package
   and run `go test ./...`.

## Tech Stack

| Category | Technology |
|---|---|
| Language | Go 1.25 |
| HTTP Router | Chi v5 |
| Auth | SuperTokens (passwordless magic link) |
| Authorization | Casbin v2 |
| FHIR | Blaze (FHIR R4), proxied internally |
| Session/Cache | Redis |
| Payments | Xendit |
| Messaging | RabbitMQ (mailer, WhatsApp queues) |
| Logging | Zap (structured) |
| Config | godotenv (.env) |
| Container | Docker multi-stage (Alpine) |
| Deployment | Coolify (deploy triggered by .github/workflows) |

## Key Dependencies

- `github.com/go-chi/chi/v5` — HTTP router
- `github.com/supertokens/supertokens-golang` — Auth
- `github.com/casbin/casbin/v2` — RBAC
- `github.com/rabbitmq/amqp091-go` — Messaging
- `github.com/xendit/xendit-go/v7` — Payments
- `go.uber.org/zap` — Logging
- `github.com/joho/godotenv` — Config (.env loading)
- `github.com/robfig/cron/v3` — Scheduled tasks (slot generation)

## Deployment

- **Domain**: api.konsulin.care
- **SSL**: Let's Encrypt (via Nginx reverse proxy)
- **Infra**: Docker Compose services (Redis, Blaze, SuperTokens, RabbitMQ, PostgreSQL)
- **CI/CD**: GitHub Actions (`.github/workflows/`) → Coolify
