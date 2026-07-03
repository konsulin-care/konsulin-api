# cmd/ — Entry Points

Three entry points in this directory:

## `cmd/http/` — HTTP API Server (Main)

Primary entry point. Bootstraps config, logger, Redis, RabbitMQ, all services, middleware chain, routes, and starts HTTP server with graceful shutdown.

```bash
go run ./cmd/http           # development
go build -o server ./cmd/http  # production build
```

Configuration via `.env` file + `config.*.yaml`. No CLI flags — all config is env-driven via Viper.

### Startup sequence
1. Load env vars (godotenv) + YAML config (Viper)
2. Initialize logger (Zap)
3. Connect Redis, RabbitMQ, SuperTokens
4. Initialize FHIR HTTP client
5. Build middleware chain
6. Register routes (API v1 under `/api/v1/`, FHIR proxy under `/fhir`, terminology proxy under `/api/v1/tx`)
7. Start HTTP server with graceful shutdown (SIGTERM/SIGINT)

## `cmd/migration/` — Database Migration Tool

Runs SQL migrations for the SuperTokens PostgreSQL database.

```bash
go run ./cmd/migration
```

Uses `sql-migrate` library. Migration files expected in `migrations/` directory.

## `cmd/example/` — Sample / Reference Code

Minimal example demonstrating package usage patterns. Useful reference when adding new features.

```bash
go run ./cmd/example
```

---

## See Also

- [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md) — Full architecture with middleware chain details
- [docs/STRUCTURE.md](../docs/STRUCTURE.md) — Directory layout
