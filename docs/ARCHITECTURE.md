# Architecture

## System Overview

Clients → **API Gateway** (auth → RBAC → routing) → Internal (Blaze FHIR, webhooks) / External (OY! Indonesia, Xendit)

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
/pay/* → Auth Middleware → RBAC Filter → Payment Service → OY! / Xendit → Transaction Storage
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

## Tech Stack

| Category | Technology |
|---|---|
| Language | Go 1.25 |
| HTTP Router | Chi v5 |
| Auth | SuperTokens (passwordless magic link) |
| Authorization | Casbin v2 |
| FHIR | Blaze (FHIR R4), proxied internally |
| Session/Cache | Redis |
| Payments | OY! Indonesia + Xendit |
| Messaging | RabbitMQ (mailer, WhatsApp queues) |
| Logging | Zap (structured) |
| Config | Viper + godotenv |
| Container | Docker multi-stage (Alpine) |
| Deployment | Coolify (deploy triggered by .github/workflows) |

## Key Dependencies

- `github.com/go-chi/chi/v5` — HTTP router
- `github.com/supertokens/supertokens-golang` — Auth
- `github.com/casbin/casbin/v2` — RBAC
- `github.com/rabbitmq/amqp091-go` — Messaging
- `github.com/xendit/xendit-go/v7` — Payments
- `go.uber.org/zap` — Logging
- `github.com/spf13/viper` — Config
- `github.com/robfig/cron/v3` — Scheduled tasks (slot generation)

## Deployment

- **Domain**: api.konsulin.care
- **SSL**: Let's Encrypt (via Nginx reverse proxy)
- **Infra**: Docker Compose services (Redis, Blaze, SuperTokens, RabbitMQ, PostgreSQL)
- **CI/CD**: GitHub Actions (`.github/workflows/`) → Coolify
