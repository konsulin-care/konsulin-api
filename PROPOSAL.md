# PRD — Backend (open source, "party instance")

> Draft living in the backend repository (contract work). Contract paths point
> at the gateway (`../gateway/docs/api/...`) and at
> `../gateway-sdk-go/PROPOSAL.md` for `SDK-*` IDs. The external repos are
> not modified by this draft.

## 1. Metadata & conventions

- **Version**: 1.0 (draft) · **Status**: pre-MVP; entry **gated on Gateway MVP (see gateway PRD §16)**.
- **Contract authority** (order, conflicts resolve upward): (1) the gateway contract — Bruno collections at `../gateway/docs/api/` (normative wire format) plus the gateway PRD (`../gateway/PROPOSAL.md`, normative behavior); (2) the SDK requirements by `SDK-*` ID (`../gateway-sdk-go/PROPOSAL.md`, implemented by the external SDK); (3) this PRD. This PRD never redefines wire format.
- **Requirement notation**: IDs `BK-*`, priorities `M/S/C`, SHALL/SHOULD/MAY, terse acceptance criteria.
- **License (decision)**: this repo remains free and open source and is **fully AGPL-3.0 — no MIT carve-out** anywhere in it (there is no in-repo SDK left to dual-license).
- **SDK consumption (decision)**: the gateway client is an **external module** — `github.com/konsulin-care/gateway-sdk-go`, MIT, its own repository, its own `go.mod`, semver tags — consumed **pinned to a semver tag** behind one adapter package (§7). The gateway does not import the SDK.
- **Module rename (dropped)**: the former module-rename prerequisite is dropped — with the SDK external, the root Go module keeps its name and nothing blocks SDK consumption.
- **License boundary (consequence)**: the closed-source gateway still must not import AGPL code from this repo. The shared artifact between gateway, SDK, and this repo is the **contract, not shared code**: all three derive from the normative Bruno contract, and golden-vector fixtures are mirrored gateway → SDK repo (§12) so CI on either side catches contract drift.

## 2. Executive summary

This repo becomes the **self-hostable party instance**. It fully owns: auth (SuperTokens/magic link), RBAC (Casbin) + data-ownership filtering, and the FHIR connection (Blaze) including all identity + clinical resources. It *outsources*: payment processing, webhook delivery, inter-party attesting, identity-linking, and the **service marketplace** (listings, pricing, entitlements) to the gateway — consumed through the external gateway SDK (`github.com/konsulin-care/gateway-sdk-go`), pinned to a semver tag behind a single adapter package (§7). It additionally hosts a **research databank** (derived, de-identified, consent-gated, SQLite-backed) serving publishable endpoints to other parties' researchers. The instance is stateless except Redis (sessions/ratelimiting), Postgres (users), Blaze, and the SQLite research store.

## 3. Background & problem

Today `cmd/http/main.go` wires one binary across auth, RBAC, ownership, FHIR, payment (Xendit/OY), webhooks (RabbitMQ worker), slot generation, and terminology. Target: a lean instance that a clinic can deploy and operate, federated through the gateway. Removed: payment domain (`services/core/payments`, `payment_gateway`, `transactions`, `models/transaction`, payment DTOs, `xendit-go`), webhook domain (`services/core/webhook`, `webhookqueue`, `jwtmanager`, hook rate limiter, `drivers/messaging` RabbitMQ publishers, `HOOK_*` config surface incl. `JWT_HOOK_KEY`, `HOOK_PAID_ONLY_SERVICES`, `HOOK_SYNC/ASYNC_SERVICE_NAMES`, `BASE_PRICE_*`), slot generation (`services/core/slot`, `postfhir/slot_trigger.go`, schedule router/controller), terminology proxy (`/tx`), RabbitMQ (mailer/whatsapp publishers, `drivers/messaging`). Added: the gateway SDK adapter (sole importer of `github.com/konsulin-care/gateway-sdk-go`), identity resolver client, approvals/direct channel, research databank.

## 4. Goals & objectives

| Objective | Target |
|---|---|
| Outsourced domains removed | Zero references to payment/webhook/slot/RabbitMQ packages and deps |
| Single-instance deploy | Docker image + documented Coolify runbook (§17) |
| No plaintext PII leaves the party | Standing test on outbound gateway/party calls |
| Research databank freshness | ≤ 24 h; nightly extraction at 00:00 local |
| Withdrawal propagation | Store row deleted immediately; ≤24 h for any external snapshot exposure (documented SLA) |
| Gateway dependency | Party functions fully standalone except auth-delivery/payment/webhook/attestation calls |

## 5. Non-goals (v1 — binding)

- No payment processing logic, Xendit calls, pricing, or settlement — client-side only.
- No webhook worker/queue/retry — the gateway delivers.
- No slot generation, no `POST /tx` terminology proxy, no RabbitMQ at all.
- No master data index, no gateway-owned identity, no cross-party session trust.
- No local key custody for the gateway (gateway public keys are config only).

## 6. Personas & user stories

- **Party user (patient/practitioner)** — "Sign in, view my records, consent to research, withdraw consent anytime."
- **Party operator** — "Deploy on Coolify, put my key in env, register with the gateway, rotate my key myself."
- **Researcher (cross-party)** — "With my party's attestation, obtain a consent-scoped dataset from another party, never exposing my identity."
- **External service** — "Receive signed webhooks via the gateway; never reach into party infrastructure."
- **Gateway** — the external actor this repo integrates with.

## 7. Architecture & flows

Components: `auth`, `rbac/ownership`, `fhir proxy (Blaze)`, gateway SDK adapter (`internal/app/drivers/gateway` — signing/envelope/payments/webhooks/identity/approvals via the external SDK), `research ingress + databank store`, `privacy/purge`. Context: party ↔ gateway ↔ external services and other parties; party ↔ Blaze local.

**One adapter package (normative).** Exactly one package — e.g. `internal/app/drivers/gateway` — is the **sole importer** of `github.com/konsulin-care/gateway-sdk-go`. The adapter owns: config/key/pepper loading for gateway calls, the Redis-backed `ReplayCache` plugged into the SDK's interface, the `slog` bridge into the SDK's logger hooks, and `*wire.Error` → backend `exceptions` mapping (BK-FED-004). Everything else in the backend depends only on backend interfaces (`RegistryClient`, `PaymentClient`, `WebhookClient`, `IdentityClient`, `ApprovalClient`, verification middleware as an `http.Handler`) and never imports the SDK directly (BK-FED-006). **FHIR mapping and consent are backend-local: neither is ever sent to the gateway, and no gateway call is involved in either.** Enrollment, request signing, and callback verification are prerequisites for every gateway feature — not webhook-specific.

**Integration map (backend requirement → SDK surface):**

| Backend requirement | SDK surface |
|---|---|
| `BK-PARTYID-002` (first-startup enrollment) | `parties.Enroll` |
| `BK-PARTYID-003` (key rotation) | `parties.Rotate` |
| `BK-PAY-001`, `BK-PAY-003` (create/get payment) | `payments` (`Create`, `Get`, `AccessToken`) |
| `BK-PAY-002`, `BK-PAY-005` (settlement/callback receiver) | `verify` middleware |
| `BK-REFERRAL` | `approvals` (`Mint`) + `envelope` + recipient verification (six rules) |
| `BK-DATABANK` (cross-party export) | `approvals` with `scope=export:<study>` |
| `BK-MARKET` | `listings`, `webhooks`, `verify` |
| identity plane (register/resolve/link) | `identity` (hashing) + `identities` (routes) |

Normative flows:

- **First startup**: the gateway admin creates the party and hands the operator `APP_PARTY_ID` + a single-use `APP_PARTY_ENROLLMENT_TOKEN` out of band (admin API; `party keygen` prints what to give the admin) → operator puts them and the keypair in env → backend creates `Organization/archetype`, derives its `kid` (RFC 7638 thumbprint), and checks enrollment against `GET /v1/parties/{id}/jwks` — **the gateway is the source of truth: enrolled means the backend's own `kid` appears in the party JWKS** (no local enrolled flag) → if absent, `parties.Enroll` via the SDK (`enrollment_token`, `public_key`, `kid`, `enc_public_key` + PoP proof) → persist `party_id` at `Organization/archetype` as `Identifier` (`system=urn:konsulin:party`) → after enrollment succeeds, fetch the sealed federation pepper through the gateway (`federation-pepper` listing, BK-PEPPER-001) and keep the sealed JWE for startup. If enrollment fails (gateway down, expired token, rejected proof) the backend keeps running with federation features disabled — every gateway call fails with a "not enrolled" error — and retries in the background; an expired token needs an admin reissue.
- **Signup/identity**: patient creates account locally (FHIR Patient etc. fully local) → party hashes email/phone (normalized, peppered) → `POST /v1/identities/register`.
- **Magic link**: `passwordless.SendEmail` override forwards delivery to the gateway's webhook API (signed); response semantics unchanged for the user.
- **Payment**: client → party `CreatePayment` (role/service gating local) → gateway invoice → Xendit → gateway settlement notification → party handler → FHIR artifacts.
- **Referral (A→B)**: A `POST /v1/approvals` (scope=referral, purpose, ttl) → ticket → A → B direct with JWE payload + ticket → B decrypts, verifies, soft-matches, calls `link`, human-confirms.
- **Research export (A→B)**: A requests ticket (scope=export:<study>) → A→B direct `POST /research/exports` JWE → B validates publication match + consent, runs async job, mints single-use download URL bound to ticket `jti` → A downloads encrypted tar.gz.
- **Withdrawal**: party purge (existing) + `DELETE FROM research_resources WHERE subject_ref=?` + mark Consent.

## 8. Security model

- **Party keys**: Ed25519/ES256 private key in env (guards: gitignored, 0600, injected by orchestrator, never logged). Public key dispositive at the gateway.
- **Envelope default**: all federation-internal payloads are envelope-encrypted (X25519 + AES-256-GCM; JWE container; streaming via `age`-style chunked AEAD for exports). Routing metadata stays clear-text for dedupe/routing.
- **Outbound hygiene**: emails/phones normalized then `HMAC(…, pepper)`; never plaintext to the gateway or other parties. **Federation pepper (decision, option B)**: one pepper for the whole federation, generated once by a Konsulin operator — 32 random bytes, standard base64 (e.g. `openssl rand -base64 32`); the master copy lives in Konsulin's party instance env (`APP_PEPPER`) plus an offline backup in Konsulin's secret store; it is **never in the gateway** — not in gateway code, config, `.env`, DB, or logs (gateway ADR-005 is the record). Konsulin's party instance SHALL run in a **separate deployment environment from the gateway**. Parties obtain the pepper sealed to their `use:enc` key via the `federation-pepper` listing (BK-PEPPER); `APP_PEPPER` on a non-Konsulin party is only the manual-delivery fallback (option A). Accepted limit: the plaintext pepper lives in every federation party's process memory (BK-PEPPER).
- **Callback verification**: gateway-signed callbacks verified against gateway JWKS; `jti` + short `exp`; LRU dedupe; fail closed.
- **RBAC/ownership**: unchanged Casbin + ownership engine for in-party traffic. Research ingress enforces its own layering; no Casbin mutation for ephemeral access.
- **Secrets**: party private key, gateway URL + gateway public key, Blaze/SuperTokens/Redis/Postgres creds, SQLite path — all env/`APP_*`. The federation pepper rests only as the sealed JWE (BK-PEPPER-001) — plaintext exists in process memory, never on disk or in env on a non-Konsulin party.
- **TLS**: all outbound/inbound HTTPS; refuse cleartext for gateway-facing calls.

## 9. Functional requirements

### 9.1 Config `BK-CONF`

Aim: every deploy-time decision explicit; secrets out of git. Objective: a party deploys from `.env.example` alone. Approach: strict `APP_*` env contract; startup validation fails loudly. Rationale: Coolify operators are not developers.

- `BK-CONF-001 (M)` — SHALL document and require the strict `APP_*` env contract. Key, enrollment, and pepper rows (decided):

| Variable | Status | Meaning |
|---|---|---|
| `APP_GATEWAY_URL` | required (kept) | gateway base URL; HTTPS only |
| `APP_PARTY_ID` | required (replaces `APP_PARTY_ID_*`) | the party ID, known **before** enrollment — issued out of band by the gateway admin (`POST /v1/admin/parties`); never self-registered |
| `APP_PARTY_PRIVATE_KEY` | required (kept) | Ed25519 (or ES256) private key; signs `ks-party+jwt` requests and enrollment proofs |
| `APP_PARTY_ENC_PRIVATE_KEY` | required (new) | X25519 private key; opens JWE envelopes at startup into memory only. Its public half is sent as `enc_public_key` at enrollment/rotation and published in the party JWKS as `use:enc` |
| `APP_PARTY_ENROLLMENT_TOKEN` | new | single-use enrollment token, read **only while the party is not enrolled**; after enrollment it is ignored and the backend logs a reminder to remove it from env; an expired token needs an admin reissue |
| `APP_PARTY_NEXT_PRIVATE_KEY` | optional (new) | next signing key — rotation path (a), BK-PARTYID-003 |
| `APP_PARTY_NEXT_ENC_PRIVATE_KEY` | optional (new) | next X25519 key — rotation path (a), BK-PARTYID-003 |
| `APP_PEPPER` | optional fallback | manual pepper delivery (option A) — the fallback when the sealed post-enrollment fetch (BK-PEPPER-001) is not used; on Konsulin's own instance this var holds the federation master copy (§8), never on the gateway |

Dropped from the contract: `APP_PARTY_PUBLIC_KEY` — the public key is derived from `APP_PARTY_PRIVATE_KEY` at load, never configured; `APP_PARTY_KID` — there is no kid config: the `kid` is always the RFC 7638 thumbprint of the public key, derived. Rest unchanged: `APP_DATA_DIR`, `APP_TIMEZONE`, `APP_FHIR_BASE_URL`, `APP_SUPERTOKEN_*`, `APP_REDIS_URL`, `APP_DATABASE_*`, `APP_AUTOPUBLISH`.

- `BK-CONF-002 (M)` — SHALL fail startup with an actionable message for missing/unwritable required config (e.g., `APP_DATA_DIR` not writable).
- `BK-CONF-C01`: fresh clone + `.env` + `docker compose up` reaches green health.

### 9.2 Party identity `BK-PARTYID`

Aim: the instance's identity is discoverable and rotatable. Objective: stable identity, self-serve keys. Approach: `Organization/archetype` well-known URL, party ID as Identifier; first-startup enrollment gated on the gateway's party JWKS; key rotation via the SDK adapter or the `party rotate` CLI. Rationale: stable IDs preserve cross-reference integrity; keys churn, identity doesn't.

- `BK-PARTYID-001 (M)` — SHALL serve `GET /fhir/Organization/archetype` publicly, containing the party ID as an `Identifier`; the resource URL never encodes the party ID.
- `BK-PARTYID-002 (M)` — SHALL complete first-startup enrollment: `party_id` and the single-use enrollment token come from the gateway admin out of band (no self-registration). On startup the backend derives its `kid` (RFC 7638 thumbprint) and treats **`GET /v1/parties/{id}/jwks` containing that `kid` as the only enrolled check** — the gateway is the source of truth, no local enrolled flag. If absent, it SHALL enroll through the SDK (`parties.Enroll`) with `enc_public_key` (the public half of `APP_PARTY_ENC_PRIVATE_KEY`) and persist the party ID at the archetype. On any failure the backend SHALL keep running with federation features disabled — gateway calls fail with a "not enrolled" error — and retry enrollment in the background; an expired token requires an admin reissue.
- `BK-PARTYID-003 (M)` — SHALL support self-service key rotation with two paths: **(a)** with `APP_PARTY_NEXT_PRIVATE_KEY` / `APP_PARTY_NEXT_ENC_PRIVATE_KEY` set, on startup the backend signs the rotation request with the **current** key (`POST /v1/parties/{id}/keys` via the SDK), switches to the next key(s), and logs a promote reminder (operator moves `NEXT_*` into the main vars and clears `NEXT_*`); **(b)** the one-shot `party rotate` CLI (BK-PARTYID-005). The gateway's 10-minute grace window (`APP_KEY_GRACE_WINDOW`, default 10m) covers the switch: the superseded key stays in the JWKS and auth lookups, so in-flight requests signed with the old key keep verifying. Every `kid` is the key's RFC 7638 thumbprint; the old party ID is never re-issued.
- `BK-PARTYID-004 (M)` — SHALL ship a `party keygen` CLI that generates an Ed25519 + X25519 keypair and prints paste-ready env values (`APP_PARTY_PRIVATE_KEY`, `APP_PARTY_ENC_PRIVATE_KEY`) plus the derived public key, `kid` (RFC 7638 thumbprint), and `enc_public_key` that the operator gives the gateway admin to obtain `APP_PARTY_ID` and the enrollment token. It never writes key material to disk or into logs.
- `BK-PARTYID-005 (M)` — SHALL ship a `party rotate` CLI: one shot — generate the next keypair, sign `POST /v1/parties/{id}/keys` with the current key through the adapter (publishing the new `enc_public_key`), then print the new paste-ready env values and the promote reminder; the gateway's grace window covers the switch.
- `BK-PARTYID-C01`: archetype readable unauthenticated; the enrolled check resolves from the party JWKS (own `kid` present) with no local flag; a failed enrollment leaves the backend serving with gateway calls failing "not enrolled" and a background retry pending; rotation shuffles JWKS without breaking in-flight ticket verification (grace window respected).

### 9.3 Auth `BK-AUTH`

Aim: keep the full auth UX while delivering magic links through the gateway. Objective: auth flow untouched for users. Approach: re-point the SuperTokens email-delivery override from local forwarder to gateway webhook API. Rationale: user-approved — auth is gateway-dependent for delivery only; sessions/roles stay local.

- `BK-AUTH-001 (M)` — SHALL deliver magic links via the gateway's signed webhook API (replacing the in-process forwarder); failure surfaces to `passwordless.SendEmail` semantics (retryable).
- `BK-AUTH-002 (M)` — SHALL parameterize all hardcoded Konsulin identity (tenant ID, org resource ID, dashboard admin email, bank display name) through config.
- `BK-AUTH-003 (M)` — SHALL retain the existing SuperTokens session, roles, and RBAC flows unchanged.
- `BK-AUTH-C01`: magic-link e2e through gateway sandbox; non-Konsulin party boots with its own tenant/org names.

### 9.4 FHIR `BK-FHIR`

Aim: the instance keeps full ownership of its FHIR data. Objective: no behavioral change to proxy/RBAC/ownership. Approach: leave `fhir_http_client`, `fhir_spark`, proxy, Casbin, ownership intact; touch nothing except removing consumers of removed domains. Rationale: zero regressions in clinical paths is the top priority of this migration.

- `BK-FHIR-001 (M)` — SHALL preserve the `/fhir/*` bridge, Casbin policies, ownership filters, and identity resource reads/writes exactly as today (changes only where a removed domain was the only consumer).
- `BK-FHIR-002 (M)` — SHALL preserve Researcher-role behavior inside the party per existing ownership rules.

### 9.5 Federation client `BK-FED`

Aim: one external SDK behind one adapter, no wire-protocol duplication. Objective: every gateway interaction goes through `github.com/konsulin-care/gateway-sdk-go`, pinned to a semver tag and imported only by the adapter package (§7). Approach: the SDK provides signing (`SDK-AUTH`), inbound verification (`SDK-VERIFY`), envelope (`SDK-ENV`), identity hashing (`SDK-ID`), and typed route clients (`SDK-API`); the adapter wires them to backend config, the Redis `ReplayCache`, `slog`, and backend exceptions. Rationale: parties must uniformly implement the contract; the SDK is the auditable, conformance-tested client — this repo keeps backend logic only. Enrollment, request signing, and callback verification are prerequisites for every gateway feature, not webhook-specific.

- `BK-FED-001 (M)` — SHALL use the SDK for outbound request signing and inbound callback verification (`SDK-AUTH`, `SDK-VERIFY`): `ks-party+jwt` claims per Token Kinds, header `kid` selection with a bounded JWKS refetch, per-kind checks, `body_sha256` over the exact request bytes, exact `typ`, and `webhook_id`/`jti` dedupe.
- `BK-FED-002 (M)` — SHALL use the SDK envelope (`SDK-ENV`) to seal/open all federation-internal payloads by default (X25519 + AES-256-GCM JWE) and keep routing metadata clear-text; streaming export envelopes remain out of SDK v1 and are revisited with the SDK's open decision.
- `BK-FED-003 (M)` — SHALL use the SDK typed clients (`SDK-API`): `payments`, `webhooks`, `identities` (register/link/resolve with hashed fields), `approvals` (ticket mint + direct channel).
- `BK-FED-004 (M)` — SHALL map the SDK's `*wire.Error` (`SDK-WIRE-003`) into `exceptions` CustomErrors preserving controller HTTP semantics (backend duty, implemented in the adapter).
- `BK-FED-005 (S)` — SHALL rely on the SDK retry policy (`SDK-RETRY`): `Idempotency-Key` on creates, a fresh `jti` per attempt, retryable vs terminal status classes as specified in the SDK PRD.
- `BK-FED-006 (M)` — SHALL confine all SDK use to the single adapter package (§7), which owns config/key/pepper loading, the Redis `ReplayCache` behind the SDK interface, the `slog` bridge, and error mapping (BK-FED-004); no other package imports `gateway-sdk-go`.
- `BK-FED-007 (M)` — SHALL consume the SDK only as a pinned semver tag: no committed `replace` directive; upgrades are deliberate and Renovate-driven, checked against the SDK's `COMPATIBILITY.md` before merging.
- `BK-FED-C01`: outbound request verifies at the gateway sandbox; callback replay with an old `jti` is rejected; payload bytes never appear in gateway-facing logs; adapter tests pass offline against `gatewaytest` (`SDK-TEST-002`).

### 9.6 Referral `BK-REFERRAL`

Aim: clinical hand-off between parties without a gateway data path. Objective: authenticated, consent-scoped, human-confirmed. Approach: approval ticket + direct JWE + soft-match + explicit link. Rationale: puts data custody at the parties and the confirm decision with a human.

- `BK-REFERRAL-001 (M)` — SHALL build and send referrals: request approval (`scope=referral`, `purpose=<study or reason>`, `researcher_hash` where applicable), JWE payload (local patient ref + demographics + ServiceRequest) directly to the recipient's endpoint with the ticket.
- `BK-REFERRAL-002 (M)` — SHALL receive referrals: decrypt, verify ticket (`aud == self`, `exp`, `jti`), then soft-match patient (email/phone/name) and surface a human-confirmation step; SHALL call `identities/link` after confirmation.
- `BK-REFERRAL-003 (M)` — SHALL NOT auto-join clinical data across parties on match; candidates only.
- `BK-REFERRAL-C01`: A→B referral round-trips via ticket without the gateway receiving the payload; B's unsent/cancel state behaves correctly.

### 9.7 Payment client `BK-PAY`

Aim: the instance initiates and records payments but never processes money. Objective: correct UX + FHIR artifacts. Approach: SDK `CreatePayment`/`GetPayment` (via the adapter); settlement notification handler drives FHIR side effects. Rationale: money logic belongs to the gateway; clinical records belong here.

- `BK-PAY-001 (M)` — SHALL implement `CreatePayment` against a **gateway listing**: `CreatePayment({listing_id, customer_ref?})` returns `{payment_id, payment_url, amount, currency, expires_at, status}`. The amount is read by the gateway from the listing row — the buyer never sends a price and this repo holds NO pricing logic (`BASE_PRICE_*` deleted; "no pricing logic anywhere in the repo" now holds by construction). `customer_ref` (peppered end-user hash, never plaintext PII) is REQUIRED when the listing's `price_scope` is `user` (b2c entitlements). Role/service purchase gating stays local (roles are local).
- `BK-PAY-002 (M)` — SHALL serve the settlement-notification handler: verify the **gateway-signed JWT** against the gateway JWKS (`GET /.well-known/jwks`), check `aud` + `exp`, dedupe `webhook-id`, then create/update FHIR artifacts (e.g., PaymentNotice, PaymentReconciliation) per current behavior. The handler receives BOTH audiences — provider-side (fulfillment trigger) and buyer-side (`access_token_url` hint + post-purchase flows). Xendit posts no longer reach this repo (see BK-PAY-005).
- `BK-PAY-003 (M)` — SHALL expose `GetPayment` for status re-check (buyer-polling truth channel; response carries `customer_ref`).
- `BK-PAY-004 (S)` — entitlement for paid webhook services is the **gateway-side `payments` row**: after `CreatePayment` settles (`PAID`), invocation via the gateway SDK's webhooks client (through the adapter) carries the caller party identity + `user_ref` (b2c); the gateway checks its own row at enqueue (402 `GW-HOOK-006` without it). The legacy `X-Forwarded-From-Payment-Service` JWT mechanism is DELETED — the gateway no longer mints forwarded tokens and this repo no longer verifies them.
- `BK-PAY-005 (M)` — callback receiver re-auth: the settlement-notification endpoint re-authenticates from the legacy Xendit static-token/HMAC schemes to the **gateway JWKS JWT** (signature + `aud` + `exp` + `webhook-id` dedupe, fail closed). One receiver, claim-discriminated events (`purpose= settlement-notification | service-result`); after the gateway Xendit-URL cutover (gateway PROPOSAL §13/§17) no Xendit traffic targets this repo at all.
- **Decision point (flagged)**: appointment side effects touching `Slot` availability. Provisional: BFF owns slot lifecycle; the backend records Appointment + payment artifacts only. Needs product sign-off before implementation.
- `BK-PAY-C01`: create→pay→settlement→artifact chain green against gateway sandbox; expired payment surfaces correctly; no pricing logic anywhere in the repo.

### 9.8 Research databank `BK-DATABANK`

Aim: an ethical, consent-gated research databank per party. Objective: ≤24 h freshness, immediate withdrawal, export-only research access. Approach: SQLite derived store; nightly `_since` ingestion; runtime publication registry (no Casbin mutation); on-demand tar.gz exports. Rationale: separates decision support from clinical OLTP; append-only file artifacts were rejected because they conflict with withdrawal compliance.

- `BK-DATABANK-001 (M)` — SHALL maintain `research_resources(id, resource_type, source_id, party_id, subject_ref, consent_state, created_at, purged_at)` — the derived, de-identified resource index.
- `BK-DATABANK-002 (M)` — SHALL ingest from Blaze via nightly `_since` extraction (00:00 local, ≤24 h freshness SLA) and derive de-identified publishable resources only.
- `BK-DATABANK-003 (M)` — SHALL enforce withdrawal immediately: `DELETE FROM research_resources WHERE subject_ref=?` + Consent mark on the source patient; documented SLA ≤24 h for any external snapshot exposure.
- `BK-DATABANK-004 (S)` — SHALL gate cross-party access on attestation tickets (`scope=export:<study>`) minted via the gateway approvals plane; exports as single-use download URLs bound to the ticket `jti`.
- `BK-DATABANK-C01`: withdrawal removes every derived row; a ticketless researcher request is denied fail-closed.

### 9.9 Service marketplace `BK-MARKET` *(new — gateway extraction)*

Aim: the service catalog, pricing, and access control move from this repo's `.env`/constants into the gateway marketplace. Objective: parties PUBLISH services self-serve and get paid through the gateway; the gateway owns price truth and entitlement checks. Approach: gateway listings replace the hardcoded service registry; invocation rides the gateway webhook pipe (sync + async). Rationale: the legacy design required a redeploy to change a price or open a service (the `BASE_PRICE_*` / `HOOK_PAID_ONLY_SERVICES` pain), had no discoverability, and no cross-party access model.

- `BK-MARKET-001 (M)` — SHALL enlist provided services as gateway listings via the gateway SDK (through the adapter): `POST /v1/listings {name, description, endpoint_url, price, price_scope, invocation}`. `invocation: webhook` services become invokable through the gateway pipe; `price_scope: party|user` selects b2b vs b2c entitlement granularity; `price: 0` marks free. The legacy `constvars/services.go` registry (`analyze`/`report`/`performance-report`/`access-dataset`), `ServiceToMinQuantity`, `BASE_PRICE_*`, `HOOK_PAID_ONLY_SERVICES`, and `HOOK_SYNC/ASYNC_SERVICE_NAMES` CSVs are DELETED — catalog truth lives in gateway rows, discoverable via `GET /v1/listings`.
- `BK-MARKET-002 (M)` — SHALL invoke provider services through the gateway: async `POST /v1/webhooks {to, service, payload, user_ref?}` (202 + `webhook_id`, status via `GET /v1/webhooks/{id}/status`) or sync `POST /v1/webhooks/sync` (verbatim relay; 504 on timeout — no auto-fallback). The gateway delivers to the listing's `endpoint_url` with a gateway-signed JWT + `Webhook-Id` (dedupe on it); this repo's party endpoint verifies against gateway JWKS.
- `BK-MARKET-003 (M)` — SHALL delete the local webhook forwarding stack: `services/core/webhook` (usecase/worker/auth_helper), `services/shared/webhookqueue` (RabbitMQ), `drivers/messaging`, the hook rate limiter, `JWT_HOOK_KEY`, and every `HOOK_*`/`BASE_PRICE_*` config key. Internal relays (magic-link delivery, omnichannel profile sync) become gateway webhook enqueues to the party's own registered flow endpoint — the flow engine is simply a listing's `endpoint_url`, never a gateway concept.
- `BK-MARKET-004 (S)` — async service results: engines post back to the gateway (`POST /v1/webhooks/{id}/result`, single-use `result_ticket` from the invocation body); the gateway delivers `{webhook_id, result}` to the consuming party's `endpoints.callbacks` — this repo's handler verifies gateway JWKS JWT + dedupes `webhook-id` (same receiver as BK-PAY-005, claim-discriminated).
- `BK-MARKET-C01`: a price change is a `POST /v1/listings` re-enlist, never a redeploy; an invocation of an unpaid priced listing is denied 402 at the gateway; free invocations leave no rows in THIS repo beyond local zap logs.

### 9.10 Federation pepper `BK-PEPPER`

Aim: one federation-wide HMAC pepper, delivered to parties sealed to their published `use:enc` key — the gateway never holds it. Objective: a party obtains the pepper without any out-of-band secret exchange beyond its own keys. Approach: option B — after enrollment, the party fetches the sealed pepper through the gateway's sync relay against the free `federation-pepper` webhook listing served by Konsulin's own instance; option A (manual out-of-band delivery into `APP_PEPPER`) stays the fallback. Rationale: keeps the pepper out of the gateway entirely (gateway ADR-005) while removing manual distribution from the happy path.

- `BK-PEPPER-001 (M)` — every party SHALL fetch the sealed pepper after enrollment: invoke the free `federation-pepper` listing through the gateway sync relay (`POST /v1/webhooks/sync` via the SDK `webhooks` client, `SDK-API`); Konsulin's instance returns the pepper sealed as a JWE (ECDH-ES+A256GCM) to the party's `use:enc` key; the gateway relays ciphertext only. The party SHALL store the sealed JWE exactly as received — ciphertext at rest, never rewritten — open it at startup with `APP_PARTY_ENC_PRIVATE_KEY` into memory only, and never write plaintext pepper to disk or env. `APP_PEPPER` remains the fallback for manual delivery (option A), used only when the sealed fetch is not taken up; the sealed JWE takes precedence when present.
- `BK-PEPPER-002 (M)` — Konsulin's own party instance SHALL enlist and serve the free `federation-pepper` webhook listing (`price: 0`): on invocation it verifies the gateway-signed delivery JWT (`ks-delivery+jwt` against the gateway JWKS, `purpose=webhook-delivery`, fail closed), takes the calling party from `sub`, looks up that party's `use:enc` key in the party's JWKS (`GET /v1/parties/{id}/jwks`), seals the pepper as a JWE to it, and fails closed if the `use:enc` key is absent. Konsulin's instance SHALL run in a separate deployment environment from the gateway (§8).
- `BK-PEPPER-C01`: after enrollment the party serves peppered hashes from a pepper opened at startup into memory only, with the sealed JWE as the only pepper artifact at rest; a `federation-pepper` invocation for a party without a `use:enc` key is denied fail-closed; the `APP_PEPPER` fallback path produces identical hashes.

**Accepted limit (decision)**: the plaintext pepper exists in every federation party's process memory — including Konsulin's own instance. This is accepted: memory-only exposure matches the single-node, operator-trusted deployment model, and the pepper never rests on disk or in env on a non-Konsulin party and never crosses the gateway.

## 10. Data model

Unchanged locally except by deletion: Postgres (users/roles), Blaze (FHIR), SQLite (research databank, per §9.8). No payment, webhook, or pricing tables exist locally — the gateway owns `listings`, `payments`, `webhook_outbox`, `tickets`, `audit_events`. Federation state (party_id, gateway URL, gateway public key, party keypair) lives in env + `Organization/archetype` (§9.2). The sealed federation pepper JWE is stored verbatim as received — ciphertext at rest, opened into memory only at startup (BK-PEPPER-001).

## 11. Security model deltas

- **Callback verification** now targets the gateway JWKS only (BK-PAY-005): EdDSA/ES256 signature, `aud`, `exp`, `webhook-id` dedupe, fail closed. Xendit-token auth retires at the gateway URL cutover.
- **No forwarded payment JWTs**: entitlement checks happen at the gateway against its own rows; this repo never mints or verifies cross-party access tokens except plan-016 tickets presented directly at provider endpoints (verified against gateway JWKS).
- **Outbound hygiene** unchanged: hashes before the wire, no plaintext PII to gateway or parties (b2c `customer_ref` is a peppered hash by contract). The pepper is fetched sealed after enrollment and exists in plaintext only in process memory (BK-PEPPER-001); `APP_PEPPER` is the manual-delivery fallback.
- **Payload handling**: invocation payloads crossing the gateway arrive as JWE envelopes (BK-FED-002) — the gateway holds no content keys; this repo decrypts at its endpoint.

## 12. Testing

- **Golden vectors live in the SDK repo**: fixture lockstep (fixed JWKS + signed JWT fixtures) is mirrored gateway → SDK (`gateway-sdk-go/testdata/golden/`, per SDK PRD §11) and proven in the SDK's own CI. This repo holds **no golden vectors** — the backend verifies tokens through the SDK, which is already proven against the mirrored vectors (party auth tokens, settlement notifications for both audiences, service-result deliveries, plan-016 access tickets, `ks-party` byte-exact signing).
- **Adapter tests**: the adapter (§7) and each integration point SHALL be tested against the SDK's `gatewaytest` kit, offline (`SDK-TEST-002`): idempotent creates (`Idempotency-Key` + `SDK-RETRY`), JWKS fetch/cache + rotation-on-unknown-kid (`SDK-VERIFY`), `webhook-id` dedupe hook, identity-hash vectors (`SDK-ID-C01`), envelope seal/open round trips (`SDK-ENV`), and `wire.Error` → exceptions mapping (BK-FED-004).
- **End-to-end**: full party-surface conformance is `cmd/conformance` from the SDK, pinned and run by the gateway's `qc --flow` (`SDK-CONF`); this repo's sandbox tests cover only its own handlers' behavior.
- **Marketplace contract tests**: listing enlist → invocation → settlement → artifact chain against the gateway sandbox; unpaid invocation → 402; customer_ref required on user-scoped purchases.

## 13. Out of scope (v1 — binding)

- No local pricing, invoicing, entitlement, or webhook-queue code (deleted, §9.7/§9.9).
- No gateway-side quantity/quota enforcement — providers gate access at their own endpoint post-authorization (gateway GW-PAY-006).
- No multi-gateway federation; one gateway URL per deployment.

## 14. Risks

- **Gateway dependency blast radius**: auth-delivery (magic link), payments, and service invocation all gate on gateway availability. Mitigation: the gateway SDK's retry policy (`SDK-RETRY`, through the adapter) + party-side UX degradation paths (polling status, resend magic link).
- **SDK/gateway version skew**: the backend pins an SDK tag while the gateway deploys independently, so a contract change can land on one side first. Mitigation: pin by semver tag with no committed `replace` (BK-FED-007), read the SDK's `COMPATIBILITY.md` before every upgrade, keep upgrades Renovate-driven and deliberate, and rely on golden-vector lockstep + pinned `cmd/conformance` in gateway QC to catch drift before it reaches production.
- **Xendit URL cutover** (gateway PROPOSAL §17): flip only after the gateway settlement drill; BK-PAY-005 receiver supports a dual-verify transition window.
- **n8n/flow-engine drift**: engines behind listings must honor the behavioral contract (immediate 2xx, JWKS verify, webhook-id dedupe) — see gateway `docs/integrations/n8n.md`.

## 15. Open decisions

- B2C checkout UX: redirect to gateway `payment_url` vs party-hosted checkout calling `CreatePayment` — product sign-off pending.
- Result-ticket TTL (default 24h) vs async services longer than a day (interpret batch jobs) — revisit with real workload data.
- Federation pepper versioning: v1 ships one unversioned pepper (BK-PEPPER); how a future rotation is versioned and rolled out — e.g. tagging the sealed JWE with the `enc` key `kid`, a dual-pepper verify window, re-sealing to every party — is an open decision, not built in v1.

## 16. Milestones

- **M1** — Gateway MVP gates this repo's development (gateway PROPOSAL §16): registration/rotation/JWKS, identity plane, payments + listings, webhooks + tiering all green in the gateway before the corresponding backend migration task starts.
- **M2** — the backend consumes `github.com/konsulin-care/gateway-sdk-go` **v0.1.1** behind the adapter package (§7) — external pinned module, **no module-rename prerequisite**; adapter + integration-point tests green against `gatewaytest`.
- **M3** — Payment migration (BK-PAY rewrite) against gateway sandbox (`mise run payment`).
- **M4** — Marketplace migration (BK-MARKET): listings enlisted, webhook domain deleted, invocation chain green end-to-end.
- **M5** — Callback re-auth cutover (BK-PAY-005) after the gateway Xendit-URL drill.

## Discrepancies and resolutions

Deviations found while verifying this draft against the codebase and the
gateway/SDK contracts. Where a section above disagrees, the resolution here
wins.

| # | Discrepancy | Resolution |
|---|---|---|
| 1 | Docs and draft assume an older Go toolchain (Go 1.25). | The toolchain is **Go 1.27.3** (`.mise.toml`; the SDK module targets `go 1.27`). `docs/ARCHITECTURE.md` states Go 1.27.3. |
| 2 | BK-CONF-001 lists `APP_GATEWAY_URL` as required. | `APP_GATEWAY_URL` is **optional**: without it the backend runs **standalone**, and gateway-dependent endpoints (payments, webhooks, federation) answer **503 `GATEWAY_NOT_CONFIGURED`** instead of failing startup. |
| 3 | BK-AUTH-001 makes gateway webhook delivery the only magic-link path. | **Magic-link fallback to SuperTokens**: SuperTokens' built-in email delivery is used both when the gateway is unconfigured and when a configured gateway call fails. In standalone mode **WhatsApp is disabled** — delivery is **email-only**. |
| 4 | Two pepper-delivery mechanisms compete (sealed fetch vs env var). | **Sealed pepper vs `APP_PEPPER`**: the sealed JWE from the `federation-pepper` listing (BK-PEPPER-001) takes precedence; `APP_PEPPER` is the manual-delivery fallback (option A) only, producing identical hashes. |
| 5 | BK-PAY describes only listing-priced payments. | FHIR-priced appointments use **seller mode** (`{domain, opaque, amount}` — the selling party declares the amount); ordinary catalog services use **listing mode** (`listing_id`, price pinned at enlistment). |
| 6 | The gateway contract allows `customer_email`/`customer_name` transit to the payment provider (invariant-2 exception). | The backend **never sends `customer_email` or `customer_name`** — only the peppered `customer_ref`; plaintext PII never leaves the party. |
| 7 | Draft says "LRU dedupe" and lists a gateway public key among env secrets. | Dedupe/replay is a **Redis-backed `ReplayCache`** (atomic `SET NX PX`, fail closed) — **not an in-process LRU** — and there is **no static gateway public key** in config: gateway signatures verify against the fetched gateway JWKS (`GET /.well-known/jwks`). |
| 8 | BK-FED-002 leaves the invocation transport shape open. | The **bound envelope payload travels as a JSON string**: on `POST /v1/webhooks` and `/v1/webhooks/sync`, `payload` is the compact JWE serialized as a JSON string, with the `v`/`from`/`to`/`service`/`iat`/`nonce` checks run after opening it. |
| 9 | Draft lists slot generation (`services/core/slot`, `postfhir/slot_trigger.go`, schedule router) as removed. | **No slot generation exists** in the tree to remove; **`services/core/slot` is kept** (local slot reads/writes stay), and a **paid appointment slot is marked busy** (paid slot = busy). |
| 10 | Contract wording says IDR amounts are "minor units"; code passes whole rupiah. | Amounts are **whole rupiah pass-through** — IDR has no minor unit and no conversion happens; the "minor units" wording is stale. |
| 11 | Draft removes the `/tx` proxy but leaves terminology serving unspecified. | **`/tx` is removed** from the party; terminology becomes a **gateway listing** invoked through the gateway pipe. |
| 12 | Config sections imply env renames (`APP_PARTY_ID_*` → `APP_PARTY_ID`, dropped `APP_PARTY_PUBLIC_KEY`/`APP_PARTY_KID`). | **Env names are kept** — the existing `APP_*` names stay stable for operators; no renaming beyond what §9.1 already documents. |
| 13 | Draft implies a local service catalog mirror. | **Gateway-driven `/api/v1/services`** — served from live `GET /v1/listings` data through the adapter, **not** a local catalog or registry. |
| 14 | Draft treats the n8n hook services as this party's own listings. | The **automation party owns the n8n listings** — a separate n8n provider party enlists them (GWP-006); this party invokes them, it does not own them. |
| 15 | Referral directory models counterparties ad hoc. | **Partners are FHIR `Endpoint` resources** — the referral directory reads counterparties as FHIR Endpoint resources. |
| 16 | Settlement handling implies the listing-domain settlement drives side effects, and unpaid calls need server-side handling. | Listing-domain **settlement is ack-only** (the receiver acknowledges; FHIR side effects follow the payments-row flow), and on a **402** the **frontend retries the 402'd call** after payment — no server-side retry queue. |
