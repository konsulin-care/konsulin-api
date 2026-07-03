# Known Pitfalls

Documented traps and gotchas. Update this file incrementally when discovering new issues.

## FHIR URL Double Slash

`config.FHIR.BaseUrl` has a trailing slash (`http://localhost:8080/fhir/`).

**Wrong**: `fmt.Sprintf("%s/%s/%s", baseUrl, resourceType, id)` → produces `http://localhost:8080/fhir//PractitionerRole/pr-123` (double slash)

**Correct**: `baseUrl + resourceType + "/" + id` → `http://localhost:8080/fhir/PractitionerRole/pr-123`

## Legacy Dependencies Not In Use

- **MongoDB driver** — included in `go.mod` but not actively used. The `models/` package has BSON tags from a past MongoDB era. Do not add new MongoDB code.
- **PostgreSQL references** — several outdated references in the codebase. The remaining PostgreSQL usage is ONLY for SuperTokens internal storage. Do not add new PostgreSQL queries.

## Rate Limiting Gaps

- Rate limiting uses a token bucket algorithm backed by Redis
- It works per-user and per-API-key but **lacks distributed coordination** across instances
- If scaling to multiple API gateway instances, rate limits are per-instance, not global

## SuperTokens Configuration

- `SUPERTOKEN_API_KEY` must be ≥ 20 characters (SuperTokens enforces this server-side)
- Session expiry is configured by `login_session_expired_time_in_hours` — changing it does NOT invalidate existing sessions
- Magic link delivery uses the configured `mailer` service — if mailer is down, auth is broken

## Webhook Failure Policies

Webhook forwarding supports two failure modes (configurable per webhook):
- `return_error` — fail immediately and return error to caller
- `enqueue_request` — retry via RabbitMQ queue

If `enqueue_request` is set without a properly configured RabbitMQ consumer, webhooks are silently lost.

## Casbin Policy CSV Ordering

Casbin evaluates policies in file order. If a deny rule exists but appears after an allow rule, the allow rule wins. Always order policies so that more specific rules come before general ones.

## Slot Generation

Slot generation runs on a cron schedule (daily by default) with a rolling window. On-demand regeneration is triggered via post-FHIR-proxy hooks. If the cron worker fails or the hook is not called after creating a Schedule resource, slots won't be generated until the next cron run.
