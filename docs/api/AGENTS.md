# docs/api/ — Bruno Collection Conventions

The Bruno collection (`docs/api/`) is the API test suite for the Konsulin
gateway. This guide explains how the collection is structured, how requests
reach their targets, and how to run it.

## Two Access Modes

The collection talks to two different servers:

| Mode | Base URL | Used by |
|---|---|---|
| Gateway (API) | `APP_BASE_URL` (default `http://localhost:3200`) | Everything except `fhir/seed/*` — auth, patient/practitioner journeys, payment, schedules, webhooks, ownership-violation, `fhir/admin/*`, `fhir/cleanup/delete-organization` |
| Direct Blaze (FHIR) | `BLAZE_BASE_URL` (default `http://localhost:8080`) | All `fhir/seed/*` requests — utility seeding only |

Seeds PUT directly to Blaze and carry **no auth headers**: Blaze is a
development FHIR server with no authentication, so nothing to send. The
gateway is not involved for these requests.

## Why Seeds Bypass the Gateway

Seeds are environment setup, not gateway behavior under test. Writing them
through the gateway would require a Superadmin RBAC entry (GET/POST/PUT) for
every seeded resource type — a policy line per seed. Seeding directly to Blaze
keeps `resources/rbac_policy.csv` about real API capabilities and lets the seed
chain mirror the checkout data without policy inflation.

This also means gateways business rules are **deliberately bypassed** for
seeds: e.g. `seed-invoice.yml` writes an Invoice without the gateway's
`ensurePreconditionsValid` precondition check. The journeys that exercise the
payment flow test those rules for real.

## Environment Variables

| Variable | Meaning | Example |
|---|---|---|
| `APP_BASE_URL` | API gateway base (health-checked before the run) | `http://localhost:3200` |
| `BLAZE_BASE_URL` | Blaze FHIR base for direct seeds and cleanup | `http://localhost:8080` |
| `SUPERADMIN_API_KEY` | Gateway API key for `fhir/admin/*`, `cleanup/delete-organization` | set in `docs/api/.env` (CI: `CI_SUPERADMIN_API_KEY` environment secret) |
| `ORGANIZATION` | Boot organization name | `organization-name` |

`docs/api/.env` holds these (untracked; `.env.example` is the template).
`scripts/bru-run.sh` loads it without executing shell and exports the vars —
`{{process.env.X}}` interpolation in requests resolves from that export.
`BLAZE_BASE_URL` falls back to `http://localhost:8080` in the script even when
the `.env` lacks it.

## Running the Suite

The canonical entrypoint is the repo script, not `bru run` alone:

```bash
bash scripts/bru-run.sh            # pre-commit: SKIPs if the API is down
bash scripts/bru-run.sh --required # pre-push: FAILS if the API is down
```

Flow per run:

1. Refuse tracked `.env`; load/export collection env.
2. Health-check `${APP_BASE_URL%/}/health` (HTTP 200, build info).
3. `cd docs/api && bru run --bail` — collection runs its `setNextRequest`
   chain linearly; `--bail` stops at the first failed assertion.
4. Always run `scripts/bru-cleanup.sh` (via `bash`, non-fatal) — a
   Blaze-direct sweep that deletes the fixed-id seeds and anything referencing
   them, so a failed run never leaves seed litter.

Run from anywhere — `bru-run.sh` resolves its own dir before any `cd`.

## The Seed Chain

The suite entry is the Organization seed, not the auth chain: the fhir folder
carries `seq: 1` (auth is `seq: 2`), so the runner's first request is
`fhir/seed/seed-organization.yml` (`Organization/seed-clinic`). It captures
`organizationId` and chains into "Send Magic Link"; after the auth flow
finishes, "Set Active Role" resumes the seed chain at `seed-location`.

`fhir/seed/folder.yml` chains 10 fixed-id, idempotent PUTs (Blaze ignores
client-supplied ids on POST, so PUT is required for deterministic ids), split
by the auth flow in between:

`seed-clinic` (Organization, entry) → [auth chain] → `seed-location` → `seed-hs`
(HealthcareService) → `seed-role` (PractitionerRole) → `seed-schedule` →
`seed-wellbeing` (Questionnaire) → `seed-soap` → `seed-protocol`
(PlanDefinition) → `seed-study` (ResearchStudy) → `seed-invoice`.

The SOAP questionnaire is deliberately `seed-soap`, never `soap`: a dev Blaze
may hold the real SOAP questionnaire and the suite must not overwrite it.
Slots are not seeded — free time is computed dynamically. The Practitioner and
Patient identities come from the auth magic-link flow, not from seeds. The
magic-link request values its `organizationId` from the seeded org var
(`{{organizationId}}` = `seed-clinic`), so a fresh Blaze always has the
Organization before any PractitionerRole is created.

## Content-Type

The gateway rewrites POST/PUT `Content-Type` to `application/fhir+json`
internally, so gateway requests need no explicit header. Direct-Blaze seeds
inherit the folder-level `Content-Type: application/fhir+json` declared in
`docs/api/fhir/folder.yml`. Blaze accepts plain `application/json` writes too,
but the folder-level header keeps direct writes byte-identical to
gateway-proxied ones and feeds the fhir folder's "responses are well-formed"
test — keep it when editing seed requests.

## Related Files

- `docs/api/fhir/folder.yml` — fhir folder auth modes and chain overview
- `scripts/bru-run.sh`, `scripts/bru-cleanup.sh` — run gate and litter sweep
- Root `AGENTS.md` — project orientation; `docs/AGENTS.md` — doc index
