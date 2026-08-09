# Konsulin Backend — Agent's Guide

## Overview

Konsulin is a digital mental health platform API gateway. It routes requests to FHIR (Blaze), authentication (SuperTokens), payment (Xendit), and webhook services. The backend is a **stateless Go API gateway** — it owns auth, authorization (RBAC), and routing but no domain business logic.

## What This Project Achieves

- Passwordless magic-link authentication via SuperTokens
- Role-based access control (6 roles: Guest, Patient, Practitioner, Clinic Admin, Researcher, Superadmin)
- FHIR R4-compliant health record proxying (Blaze server) with role-based filtering
- Service-based payment processing (Xendit gateway)
- Scheduled slot management with rolling window generation
- Async webhook forwarding with rate limiting and JWT-signed payloads
- Background job processing via RabbitMQ (mailer, WhatsApp)

## Key Commands

```bash
# Run locally (requires Docker services running)
go run ./cmd/http

# Start dependencies
docker compose up -d

# Build
bash build.sh -a 'Your Name'

# DB migrations
go run ./cmd/migration
```

## Quick Navigation

- **[cmd/AGENTS.md](cmd/AGENTS.md)** — Entry points: HTTP server, migration tool, example code
- **[internal/app/AGENTS.md](internal/app/AGENTS.md)** — Application layer: config, contracts, services, delivery, drivers
- **[internal/pkg/AGENTS.md](internal/pkg/AGENTS.md)** — Shared packages: FHIR client, DTOs, constants, utilities
- **[docs/AGENTS.md](docs/AGENTS.md)** — Documentation index: which doc answers what question

## Reference Documents

- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** — System architecture, request flow, tech stack, deployment
- **[docs/STANDARDS.md](docs/STANDARDS.md)** — Coding standards, Clean Architecture conventions, FHIR client rules
- **[docs/KNOWN-PITFALLS.md](docs/KNOWN-PITFALLS.md)** — Common mistakes, legacy debt, configuration gotchas
- **[docs/STRUCTURE.md](docs/STRUCTURE.md)** — Full directory tree with descriptions

## Tech at a Glance

| Layer | Technology |
|---|---|
| Language | Go 1.25 |
| HTTP Router | Chi v5 |
| Auth | SuperTokens (magic link) + JWT |
| Authorization | Casbin RBAC |
| FHIR | Blaze server (FHIR R4) |
| Session/Cache | Redis |
| Payments | Xendit |
| Messaging | RabbitMQ |
| Logging | Zap (structured) |
| Configuration | Viper + godotenv |

## Architecture in One Sentence

Clients → API Gateway (auth → RBAC → routing) → Internal services (FHIR Blaze, webhooks) / External services (Xendit)

## Important Conventions

- **Clean Architecture**: `delivery → services (core / fhir_spark / shared) → contracts → drivers`
- **All FHIR HTTP requests MUST use** `fhir_http_client.FHIRHTTPClient.Do()` — never raw `http.Client`
- **Middleware chain order**: RequestID → Logging → BodyBuffer → CORS → SuperTokens → APIKey → Session → RateLimit → ErrorHandler
- **RBAC model**: Casbin with `sub, method, path` matching; policies in `resources/rbac_policy.csv`
