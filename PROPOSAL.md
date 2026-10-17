# PRD — Backend (open source, "party instance")

## 1. Metadata & conventions

- **Version**: 1.0 (draft) · **Status**: pre-MVP; entry **gated on Gateway MVP (see gateway PRD §16)**.
- **Contract authority**: Bruno collections at `docs/api/gateway/` (normative wire format) + the gateway PRD (normative behavior). Conflicts resolve to the contract.
- **Requirement notation**: IDs `BK-*`, priorities `M/S/C`, SHALL/SHOULD/MAY, terse acceptance criteria.
- This repo remains free and open source; the gateway SDK ships inside it.
- **Module rename (decision)**: the root Go module will be renamed from `konsulin-service` to `github.com/konsulin-care/konsulin-api` so external parties can `go get` the repo and its `gatewayclient` SDK. Execution is a separate backend task (mechanical import-path rewrite); it is a prerequisite for external SDK consumption, not for gateway development.
- **SDK location (decision)**: `gatewayclient` ships in-repo (per §9.5 BK-FED). Import path after the rename: `github.com/konsulin-care/konsulin-api/gatewayclient`.
- **License boundary (decision, AGPL-3.0 evaluation)**: this repo is AGPL-3.0. AGPL is fine for parties self-hosting the instance (copyleft on modifications they distribute to their users — aligned with the federation's open-source model), but the **closed-source gateway must NOT import this SDK**: linking AGPL code would make the gateway a derivative work and §13 would force source disclosure of the gateway to network users. Consequence: shared **contract, not shared code** between gateway and SDK — both derive from the normative Bruno/OpenAPI contracts, and both repos commit identical **golden-vector test fixtures** (fixed JWKS + signed JWT JSON) so CI on either side catches contract drift. If the gateway later needs shared types, the escape hatch is a dual-licensed (MIT) pure-types subpackage with its own LICENSE — a copyright-holder decision, deferred.

## 2. Executive summary

This repo becomes the **self-hostable party instance**. It fully owns: auth (SuperTokens/magic link), RBAC (Casbin) + data-ownership filtering, and the FHIR connection (Blaze) including all identity + clinical resources. It *outsources*: payment processing, webhook delivery, inter-party attesting, identity-linking, and the **service marketplace** (listings, pricing, entitlements) to the gateway — consumed through the in-repo `gatewayclient` SDK. It additionally hosts a **research databank** (derived, de-identified, consent-gated, SQLite-backed) serving publishable endpoints to other parties' researchers. The instance is stateless except Redis (sessions/ratelimiting), Postgres (users), Blaze, and the SQLite research store.

## 3. Background & problem

Today `cmd/http/main.go` wires one binary across auth, RBAC, ownership, FHIR, payment (Xendit/OY), webhooks (RabbitMQ worker), slot generation, and terminology. Target: a lean instance that a clinic can deploy and operate, federated through the gateway. Removed: payment domain (`services/core/payments`, `payment_gateway`, `transactions`, `models/transaction`, payment DTOs, `xendit-go`), webhook domain (`services/core/webhook`, `webhookqueue`, `jwtmanager`, hook rate limiter, `drivers/messaging` RabbitMQ publishers, `HOOK_*` config surface incl. `JWT_HOOK_KEY`, `HOOK_PAID_ONLY_SERVICES`, `HOOK_SYNC/ASYNC_SERVICE_NAMES`, `BASE_PRICE_*`), slot generation (`services/core/slot`, `postfhir/slot_trigger.go`, schedule router/controller), terminology proxy (`/tx`), RabbitMQ (mailer/whatsapp publishers, `drivers/messaging`). Added: `gatewayclient`, identity resolver client, approvals/direct channel, research databank.

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

Components: `auth`, `rbac/ownership`, `fhir proxy (Blaze)`, `gatewayclient` (signing/envelope/payments/webhooks/identity/approvals), `research ingress + databank store`, `privacy/purge`. Context: party ↔ gateway ↔ external services and other parties; party ↔ Blaze local.

Normative flows:

- **First startup**: operator puts keys in env → backend creates `Organization/archetype` → `POST /v1/parties/register` (bootstrap token + public key) → stores returned `party_id` into `Organization/archetype` as `Identifier` (`system=urn:konsulin:party`).
- **Signup/identity**: patient creates account locally (FHIR Patient etc. fully local) → party hashes email/phone (normalized, peppered) → `POST /v1/identities/register`.
- **Magic link**: `passwordless.SendEmail` override forwards delivery to the gateway's webhook API (signed); response semantics unchanged for the user.
- **Payment**: client → party `CreatePayment` (role/service gating local) → gateway invoice → Xendit → gateway settlement notification → party handler → FHIR artifacts.
- **Referral (A→B)**: A `POST /v1/approvals` (scope=referral, purpose, ttl) → ticket → A → B direct with JWE payload + ticket → B decrypts, verifies, soft-matches, calls `link`, human-confirms.
- **Research export (A→B)**: A requests ticket (scope=export:<study>) → A→B direct `POST /research/exports` JWE → B validates publication match + consent, runs async job, mints single-use download URL bound to ticket `jti` → A downloads encrypted tar.gz.
- **Withdrawal**: party purge (existing) + `DELETE FROM research_resources WHERE subject_ref=?` + mark Consent.

## 8. Security model

- **Party keys**: Ed25519/ES256 private key in env (guards: gitignored, 0600, injected by orchestrator, never logged). Public key dispositive at the gateway.
- **Envelope default**: all federation-internal payloads are envelope-encrypted (X25519 + AES-256-GCM; JWE container; streaming via `age`-style chunked AEAD for exports). Routing metadata stays clear-text for dedupe/routing.
- **Outbound hygiene**: emails/phones normalized then `HMAC(…, pepper)`; never plaintext to the gateway or other parties. Pepper in party env, rotated rarely.
- **Callback verification**: gateway-signed callbacks verified against gateway JWKS; `jti` + short `exp`; LRU dedupe; fail closed.
- **RBAC/ownership**: unchanged Casbin + ownership engine for in-party traffic. Research ingress enforces its own layering; no Casbin mutation for ephemeral access.
- **Secrets**: party private key, gateway URL + gateway public key, Blaze/SuperTokens/Redis/Postgres creds, SQLite path — all env/`APP_*`.
- **TLS**: all outbound/inbound HTTPS; refuse cleartext for gateway-facing calls.

## 9. Functional requirements

### 9.1 Config `BK-CONF`

Aim: every deploy-time decision explicit; secrets out of git. Objective: a party deploys from `.env.example` alone. Approach: strict `APP_*` env contract; startup validation fails loudly. Rationale: Coolify operators are not developers.

- `BK-CONF-001 (M)` — SHALL document and require: `APP_GATEWAY_URL`, `APP_PARTY_ID_*`, `APP_PARTY_PRIVATE_KEY`, `APP_PARTY_PUBLIC_KEY`, `APP_DATA_DIR`, `APP_TIMEZONE`, `APP_FHIR_BASE_URL`, `APP_SUPERTOKEN_*`, `APP_REDIS_URL`, `APP_DATABASE_*`, `APP_PEPPER`, `APP_AUTOPUBLISH`.
- `BK-CONF-002 (M)` — SHALL fail startup with an actionable message for missing/unwritable required config (e.g., `APP_DATA_DIR` not writable).
- `BK-CONF-C01`: fresh clone + `.env` + `docker compose up` reaches green health.

### 9.2 Party identity `BK-PARTYID`

Aim: the instance's identity is discoverable and rotatable. Objective: stable identity, self-serve keys. Approach: `Organization/archetype` well-known URL, party ID as Identifier; first-startup registration; key rotation via SDK. Rationale: stable IDs preserve cross-reference integrity; keys churn, identity doesn't.

- `BK-PARTYID-001 (M)` — SHALL serve `GET /fhir/Organization/archetype` publicly, containing the party ID as an `Identifier`; the resource URL never encodes the party ID.
- `BK-PARTYID-002 (M)` — SHALL, on first startup without persisted `party_id`, complete registration with the gateway (bootstrap token + public key) and persist the returned ID at the archetype.
- `BK-PARTYID-003 (M)` — SHALL support self-service key rotation (new `kid` signed by current key) and update the gateway; old party ID is never re-issued.
- `BK-PARTYID-C01`: archetype readable unauthenticated; rotation shuffles JWKS without breaking in-flight ticket verification (grace window respected).

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

Aim: one SDK, no wire-protocol duplication. Objective: every gateway interaction goes through `gatewayclient`. Approach: single package implementing signing, envelope, payments, webhooks, identity, approvals; callbacks verified centrally. Rationale: parties must uniformly implement the contract; auditable open-source client.

- `BK-FED-001 (M)` — SHALL sign every outbound request with the party key (`iss`, `exp` ≤ 5 min) and verify gateway-signed callbacks against JWKS (`aud`, `exp`, `jti` dedupe).
- `BK-FED-002 (M)` — SHALL envelope-encrypt all federation-internal payloads by default (X25519 + AES-256-GCM; streaming for exports) and keep routing metadata clear-text.
- `BK-FED-003 (M)` — SHALL expose typed clients: `payments`, `webhooks`, `identities` (register/link/resolve with hashed fields), `approvals` (ticket mint + direct channel).
- `BK-FED-004 (M)` — SHALL map gateway error envelopes into `exceptions` CustomErrors preserving controller HTTP semantics.
- `BK-FED-005 (S)` — SHALL support `Idempotency-Key` on creates; expose a retry policy for idempotent calls with exponential backoff.
- `BK-FED-C01`: outbound request verifies at the gateway sandbox; callback replay with an old `jti` is rejected; payload bytes never appear in gateway-facing logs.

### 9.6 Referral `BK-REFERRAL`

Aim: clinical hand-off between parties without a gateway data path. Objective: authenticated, consent-scoped, human-confirmed. Approach: approval ticket + direct JWE + soft-match + explicit link. Rationale: puts data custody at the parties and the confirm decision with a human.

- `BK-REFERRAL-001 (M)` — SHALL build and send referrals: request approval (`scope=referral`, `purpose=<study or reason>`, `researcher_hash` where applicable), JWE payload (local patient ref + demographics + ServiceRequest) directly to the recipient's endpoint with the ticket.
- `BK-REFERRAL-002 (M)` — SHALL receive referrals: decrypt, verify ticket (`aud == self`, `exp`, `jti`), then soft-match patient (email/phone/name) and surface a human-confirmation step; SHALL call `identities/link` after confirmation.
- `BK-REFERRAL-003 (M)` — SHALL NOT auto-join clinical data across parties on match; candidates only.
- `BK-REFERRAL-C01`: A→B referral round-trips via ticket without the gateway receiving the payload; B's unsent/cancel state behaves correctly.

### 9.7 Payment client `BK-PAY`

Aim: the instance initiates and records payments but never processes money. Objective: correct UX + FHIR artifacts. Approach: SDK `CreatePayment`/`GetPayment`; settlement notification handler drives FHIR side effects. Rationale: money logic belongs to the gateway; clinical records belong here.

- `BK-PAY-001 (M)` — SHALL implement `CreatePayment` against a **gateway listing**: `CreatePayment({listing_id, customer_ref?})` returns `{payment_id, payment_url, amount, currency, expires_at, status}`. The amount is read by the gateway from the listing row — the buyer never sends a price and this repo holds NO pricing logic (`BASE_PRICE_*` deleted; "no pricing logic anywhere in the repo" now holds by construction). `customer_ref` (peppered end-user hash, never plaintext PII) is REQUIRED when the listing's `price_scope` is `user` (b2c entitlements). Role/service purchase gating stays local (roles are local).
- `BK-PAY-002 (M)` — SHALL serve the settlement-notification handler: verify the **gateway-signed JWT** against the gateway JWKS (`GET /.well-known/jwks`), check `aud` + `exp`, dedupe `webhook-id`, then create/update FHIR artifacts (e.g., PaymentNotice, PaymentReconciliation) per current behavior. The handler receives BOTH audiences — provider-side (fulfillment trigger) and buyer-side (`access_token_url` hint + post-purchase flows). Xendit posts no longer reach this repo (see BK-PAY-005).
- `BK-PAY-003 (M)` — SHALL expose `GetPayment` for status re-check (buyer-polling truth channel; response carries `customer_ref`).
- `BK-PAY-004 (S)` — entitlement for paid webhook services is the **gateway-side `payments` row**: after `CreatePayment` settles (`PAID`), invocation via `gatewayclient`'s webhooks client carries the caller party identity + `user_ref` (b2c); the gateway checks its own row at enqueue (402 `GW-HOOK-006` without it). The legacy `X-Forwarded-From-Payment-Service` JWT mechanism is DELETED — the gateway no longer mints forwarded tokens and this repo no longer verifies them.
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

- `BK-MARKET-001 (M)` — SHALL enlist provided services as gateway listings via `gatewayclient`: `POST /v1/listings {name, description, endpoint_url, price, price_scope, invocation}`. `invocation: webhook` services become invokable through the gateway pipe; `price_scope: party|user` selects b2b vs b2c entitlement granularity; `price: 0` marks free. The legacy `constvars/services.go` registry (`analyze`/`report`/`performance-report`/`access-dataset`), `ServiceToMinQuantity`, `BASE_PRICE_*`, `HOOK_PAID_ONLY_SERVICES`, and `HOOK_SYNC/ASYNC_SERVICE_NAMES` CSVs are DELETED — catalog truth lives in gateway rows, discoverable via `GET /v1/listings`.
- `BK-MARKET-002 (M)` — SHALL invoke provider services through the gateway: async `POST /v1/webhooks {to, service, payload, user_ref?}` (202 + `webhook_id`, status via `GET /v1/webhooks/{id}/status`) or sync `POST /v1/webhooks/sync` (verbatim relay; 504 on timeout — no auto-fallback). The gateway delivers to the listing's `endpoint_url` with a gateway-signed JWT + `Webhook-Id` (dedupe on it); this repo's party endpoint verifies against gateway JWKS.
- `BK-MARKET-003 (M)` — SHALL delete the local webhook forwarding stack: `services/core/webhook` (usecase/worker/auth_helper), `services/shared/webhookqueue` (RabbitMQ), `drivers/messaging`, the hook rate limiter, `JWT_HOOK_KEY`, and every `HOOK_*`/`BASE_PRICE_*` config key. Internal relays (magic-link delivery, omnichannel profile sync) become gateway webhook enqueues to the party's own registered flow endpoint — the flow engine is simply a listing's `endpoint_url`, never a gateway concept.
- `BK-MARKET-004 (S)` — async service results: engines post back to the gateway (`POST /v1/webhooks/{id}/result`, single-use `result_ticket` from the invocation body); the gateway delivers `{webhook_id, result}` to the consuming party's `endpoints.callbacks` — this repo's handler verifies gateway JWKS JWT + dedupes `webhook-id` (same receiver as BK-PAY-005, claim-discriminated).
- `BK-MARKET-C01`: a price change is a `POST /v1/listings` re-enlist, never a redeploy; an invocation of an unpaid priced listing is denied 402 at the gateway; free invocations leave no rows in THIS repo beyond local zap logs.

## 10. Data model

Unchanged locally except by deletion: Postgres (users/roles), Blaze (FHIR), SQLite (research databank, per §9.8). No payment, webhook, or pricing tables exist locally — the gateway owns `listings`, `payments`, `webhook_outbox`, `tickets`, `audit_events`. Federation state (party_id, gateway URL, gateway public key, party keypair) lives in env + `Organization/archetype` (§9.2).

## 11. Security model deltas

- **Callback verification** now targets the gateway JWKS only (BK-PAY-005): EdDSA/ES256 signature, `aud`, `exp`, `webhook-id` dedupe, fail closed. Xendit-token auth retires at the gateway URL cutover.
- **No forwarded payment JWTs**: entitlement checks happen at the gateway against its own rows; this repo never mints or verifies cross-party access tokens except plan-016 tickets presented directly at provider endpoints (verified against gateway JWKS).
- **Outbound hygiene** unchanged: hashes before the wire, pepper local, no plaintext PII to gateway or parties (b2c `customer_ref` is a peppered hash by contract).
- **Payload handling**: invocation payloads crossing the gateway arrive as JWE envelopes (BK-FED-002) — the gateway holds no content keys; this repo decrypts at its endpoint.

## 12. Testing

- Golden-vector lockstep with the gateway (fixed JWKS + signed JWT fixtures committed in both repos) covering: party auth tokens, settlement notifications (both audiences), service-result deliveries, plan-016 access tickets.
  - **Fixture spec (gateway is the reference layout — M2 task copies it).** Directory `testdata/golden/` at the gateway repo root, mirrored verbatim here:
    - `testonly-gateway-signing-private.key` — base64 raw 64-byte Ed25519 private key, generated once and committed **in the gateway repo only**. The `testonly-` prefix plus the directory mark it: the gateway loads its production signing key from env/secret, and a walking CI test fails if any non-test Go file references `testdata/golden`. This repo never receives the private key — it verifies with the public JWKS only.
    - `gateway-jwks.json` — public JWKS for the test key; `kid` is its RFC 7638 thumbprint (the same value the gateway serves at `/.well-known/jwks` in production). Committed in **both** repos.
    - `settlement-*.json` — one file per vector, shape `{name, claims, jwt}`: `claims` is the exact claim set the gateway signs (`iss=gateway, sub=gateway, aud=<recipient>, purpose, webhook_id, iat, exp`), `jwt` is the compact EdDSA token. Coverage: both audiences (`aud=provider`, `aud=buyer`) × both statuses (PAID, EXPIRED); status lives in the notification payload, not the JWT, so each scenario carries a distinct `webhook_id`. Committed in **both** repos.
  - **Lockstep rule**: gateway CI signs with the test key at the fixture-fixed `iat`/`exp` and asserts byte-for-byte equality with the committed `jwt`; this repo's CI loads `gateway-jwks.json` and asserts its verifier accepts the same files. Either side changing claims, signing alg, kid derivation, or payload shape turns one CI red.
  - **Ed25519 only** for golden vectors — RFC 8032 deterministic signatures reproduce byte-for-byte; ES256 uses a random nonce and stays in unit tests.
  - **Regeneration** (deliberate, coordinated — never routine): `go test ./internal/app/services/webhooks -run TestGolden -update` in the gateway repo rewrites the fixtures from the committed test-only key; commit the diff, mirror into this repo, both CIs re-verify.
- `gatewayclient` unit tests: idempotent creates (Idempotency-Key + retry policy), JWKS fetch/cache + rotation-on-unknown-kid, `webhook-id` dedupe hook.
- Marketplace contract tests: listing enlist → invocation → settlement → artifact chain against the gateway sandbox; unpaid invocation → 402; customer_ref required on user-scoped purchases.

## 13. Out of scope (v1 — binding)

- No local pricing, invoicing, entitlement, or webhook-queue code (deleted, §9.7/§9.9).
- No gateway-side quantity/quota enforcement — providers gate access at their own endpoint post-authorization (gateway GW-PAY-006).
- No multi-gateway federation; one gateway URL per deployment.

## 14. Risks

- **Gateway dependency blast radius**: auth-delivery (magic link), payments, and service invocation all gate on gateway availability. Mitigation: gatewayclient retry policy + party-side UX degradation paths (polling status, resend magic link).
- **Xendit URL cutover** (gateway PROPOSAL §17): flip only after the gateway settlement drill; BK-PAY-005 receiver supports a dual-verify transition window.
- **n8n/flow-engine drift**: engines behind listings must honor the behavioral contract (immediate 2xx, JWKS verify, webhook-id dedupe) — see gateway `docs/integrations/n8n.md`.

## 15. Open decisions

- B2C checkout UX: redirect to gateway `payment_url` vs party-hosted checkout calling `CreatePayment` — product sign-off pending.
- Result-ticket TTL (default 24h) vs async services longer than a day (interpret batch jobs) — revisit with real workload data.

## 16. Milestones

- **M1** — Gateway MVP gates this repo's development (gateway PROPOSAL §16): registration/rotation/JWKS, identity plane, payments + listings, webhooks + tiering all green in the gateway before the corresponding backend migration task starts.
- **M2** — `gatewayclient` ships (rename to `konsulin-api` prerequisite); golden vectors committed both sides.
- **M3** — Payment migration (BK-PAY rewrite) against gateway sandbox (`mise run payment`).
- **M4** — Marketplace migration (BK-MARKET): listings enlisted, webhook domain deleted, invocation chain green end-to-end.
- **M5** — Callback re-auth cutover (BK-PAY-005) after the gateway Xendit-URL drill.
