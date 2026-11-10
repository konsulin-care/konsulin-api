# Proposal — Frontend work for `../frontend`

Companion to `PROPOSAL.md` (backend PRD). One section per migration item;
each section names the backend endpoints involved and the frontend files that
touch them. The backend PRD is the contract authority; wire format follows the
gateway Bruno contract (`../gateway/docs/api/`).

## 1. Migrate `/hook/*` and `/service-request/*` calls to `/api/v1/services/*`

The legacy webhook-proxy routes are replaced by the gateway-driven services
surface (PROPOSAL discrepancy 13: `/api/v1/services` is served from live
`GET /v1/listings` data, not a local catalog).

Backend endpoints involved:

- `GET /api/v1/services` — discovery: the frontend loads the invocable service
  catalog (name, description, price, `price_scope`, invocation mode) from here
  instead of hardcoding service names.
- `POST /api/v1/services/{service}` (sync invoke) and
  `POST /api/v1/services/{service}/async` (enqueue, `202 {webhook_id}`) with
  `GET /api/v1/services/{service}/requests/{id}` for async status/result —
  these replace `POST /api/v1/hook/synchronous/{service}`,
  `POST /api/v1/hook/{service}`, and `GET /api/v1/service-request/{id}/result`
  (old routes are removed from `webhook_router.go`).
- `402` handling: an invocation answered `402` (`GW-HOOK-006`, no PAID
  entitlement for a gateway-metered priced/user-scoped listing) triggers the
  pay flow (`POST /api/v1/pay/service` or the listing `payment_url`), then the
  frontend **retries the original 402'd call** with a fresh `Idempotency-Key`
  — no server-side retry queue (PROPOSAL discrepancy 16).

Frontend call sites to migrate:

- `src/services/profile.tsx` — `/api/v1/hook/synchronous/modify-profile`,
  `/api/v1/hook/synchronous/update-avatar`.
- `src/services/api/assessment.tsx` — `/api/v1/hook/interpret`,
  `/api/v1/service-request/{id}/result` polling.
- `src/app/record/hooks/useAssessmentPolling.ts` —
  `/api/v1/service-request/{id}/result`.
- `src/components/general/fhir-forms-renderer.tsx` — `/api/v1/hook/interpret`.

## 2. Remove `/api/v1/tx` from `src/lib/admin/endpoints.ts`

The `/tx` terminology proxy is removed from the party (PROPOSAL discrepancy 11;
terminology becomes a gateway listing invoked through the gateway pipe).

Backend endpoints involved:

- `GET /api/v1/tx` — gone; the route is deleted, so any frontend entry must go.
- Terminology reads move to `GET /api/v1/services/terminology` (the
  terminology listing invoked via the services surface), with `GET
  /api/v1/services` for discovery.

Frontend changes:

- `src/lib/admin/endpoints.ts` — delete the
  `{ path: '/api/v1/tx', methods: ['GET'] }` entry from `SPECIAL_ENDPOINTS`
  and drop the stale `/api/v1/tx` mention in the `getEndpointOptionsGrouped`
  JSDoc.
- `src/lib/admin/__tests__/endpoints.test.ts` — the assertion expecting
  `/api/v1/tx` in the Special group must flip to assert its absence.

## 3. Payments: keep `/pay/appointment`, `payment_url` redirect + return URLs, Slot busy = booked

Backend endpoints involved:

- `POST /api/v1/pay/appointment` — unchanged route. The response now carries
  `paymentUrl` (`payment_url`, gateway invoice checkout), `expiresAt`, plus
  `appointment`/`slot`/`paymentNotice` ids (see
  `internal/pkg/dto/responses/appointment_payment.go`). The invoice is created
  with success/failure **return URLs** pointing at the frontend
  (`SetSuccessRedirectUrl`/`SetFailureRedirectUrl` → `APP_FRONTEND_DOMAIN`),
  so the provider redirects back after payment.
- `GET /api/v1/pay/appointment/{id}` (payment status re-check) — poll after
  the redirect returns to confirm settlement before showing success.
- FHIR `Slot`: a paid appointment slot is marked **busy** (PROPOSAL
  discrepancy 9: paid slot = busy). Slot reads are `GET /fhir/Slot?...` and
  availability updates go through `PUT /fhir/Slot/{id}`.

Frontend changes:

- `src/services/api/appointments.tsx` (`usePayAppointment`) — redirect the
  browser to the returned `paymentUrl` instead of only showing a pending
  modal; on return from the provider, read the return URL, call the status
  re-check endpoint, then refresh `GET /fhir/Slot` for the day.
- `src/app/practitioner/practitioner-availability.tsx` and the payment dialog
  — after settlement, treat the booked slot as busy (remove it from the
  selectable set) rather than optimistically keeping it available; a
  "Slot busy" conflict response is rendered as "slot already booked" and the
  slot list is refetched.

## 4. Email-only login (WhatsApp login stays bot-driven, unavailable standalone)

PROPOSAL discrepancy 3: in standalone mode (no `APP_GATEWAY_URL`) WhatsApp is
disabled — magic-link delivery is email-only.

Backend endpoints involved:

- `POST /api/v1/auth/magiclink` (and the SuperTokens passwordless
  consume/verify flow) — email OTP / magic link is the only login path.
- `GET /api/v1/passwordless/email/exists` — unchanged helper for the email
  form.
- `POST /api/v1/hook/synchronous/send-wa-link` (moves to
  `POST /api/v1/services/send-wa-link` after item 1) — stays a **bot-driven**
  server-side flow; it is never invoked as a standalone login provider from
  the auth screen.

Frontend changes:

- `src/app/auth/page.tsx` — remove the "Continue with WhatsApp" provider
  button and the WhatsApp footer divider from the login screen; email only.
- `config/frontendConfig.ts` — drop the ThirdParty `whatsapp` provider used
  for the standalone provider picker (keep the config that serves the
  bot-driven send-wa-link flow, which does not go through the login picker).

## 5. Handle `503 GATEWAY_NOT_CONFIGURED` responses

PROPOSAL discrepancy 2: with `APP_GATEWAY_URL` unset the backend runs
standalone; gateway-dependent endpoints (payments, service invocation,
federation) answer `503` with code `GATEWAY_NOT_CONFIGURED` instead of failing
startup.

Backend endpoints involved:

- `POST /api/v1/pay/appointment`, `POST /api/v1/pay/service` — payment
  creation when the gateway is unconfigured.
- `POST /api/v1/services/{service}` / `.../async`, `GET /api/v1/services` —
  service invocation and discovery when the gateway is unconfigured.
- Any federation-backed route (approvals/identity) — same 503 envelope.

Frontend changes:

- Central error handling (the shared API client / react-query error handler)
  must recognize `503` + `GATEWAY_NOT_CONFIGURED` and render a degraded-mode
  notice ("this deployment runs standalone — payments and paid services are
  disabled") instead of a generic retry loop; suppress the 402→pay→retry flow
  and the WhatsApp/payment CTAs on those screens.

## 6. Research withdraw + researcher export request/download UI

Backend endpoints involved (per BK-DATABANK):

- `POST /api/v1/research/withdraw` — patient withdraws research consent;
  backend deletes `research_resources` rows for the subject and marks the
  source `Consent` (immediate row removal; ≤24 h external-snapshot SLA).
  Consent reads/writes also go through `GET /fhir/Consent` /
  `PUT /fhir/Consent/{id}`.
- `POST /api/v1/research/exports` — researcher (cross-party) requests a
  consent-scoped export ticket (`scope=export:<study>`); status via
  `GET /api/v1/research/exports/{id}`; the completed job mints a single-use
  download URL bound to the ticket `jti`, downloaded with
  `GET /api/v1/research/exports/{id}/download`.
- `GET /fhir/ResearchStudy` / `GET /fhir/ResearchSubject` — study and
  participation listings that the UIs render.

Frontend changes:

- Patient research/consent screen — a "Withdraw from research" action calling
  the withdraw endpoint with a confirmation step and post-withdraw state
  (withdrawn, no longer in datasets).
- Researcher dashboard (`src/app/researcher-dashboard`,
  `src/services/api/researcher*`) — an "Request export" action per published
  study (request → poll status → "Download" once the single-use URL is
  issued; one-shot download, no URL persistence).

## 7. Referral outgoing form + incoming inbox confirm/reject + partner directory admin

Backend endpoints involved (per BK-REFERRAL; partners are FHIR `Endpoint`
resources — PROPOSAL discrepancy 15):

- `POST /api/v1/referrals` — outgoing send: mints an approval ticket
  (`scope=referral`) and delivers the JWE payload + `ServiceRequest` to the
  recipient party; `GET /api/v1/referrals?dir=outgoing` lists sent referrals
  (with cancel/unsend state).
- `GET /api/v1/referrals?dir=incoming` — inbox of received referrals;
  `POST /api/v1/referrals/{id}/confirm` — human confirmation after soft-match
  (backend then calls `identities/link`); `POST /api/v1/referrals/{id}/reject`
  — decline.
- Partner directory (admin): `GET /fhir/Endpoint`, `POST /fhir/Endpoint`,
  `PUT /fhir/Endpoint/{id}`, `DELETE /fhir/Endpoint/{id}` — counterparties are
  read and maintained as FHIR Endpoint resources.
- Referral artifacts also surface as FHIR `Communication`
  (`GET /fhir/Communication?topic=research-referral`) and
  `PUT /fhir/Communication/{id}` for the existing patient-side referral write.

Frontend changes:

- Outgoing referral form — pick a partner from the directory, pick the
  patient/ServiceRequest, submit via `POST /api/v1/referrals`; outgoing list
  shows pending/sent state.
- Incoming inbox — list, open, and Confirm / Reject each referral; confirm
  requires the explicit human step (no auto-join), reject records the decline.
- Admin partner directory — CRUD screen over `GET/POST/PUT/DELETE
  /fhir/Endpoint` for counterparties (base URL, party identity, status).

## 8. ResearchStudy publish toggle

Backend endpoints involved:

- `PUT /fhir/ResearchStudy/{id}` (Superadmin `GET/POST/PUT` per the RBAC
  policy) — flips publication state (`status` / experimental flag) on the FHIR
  resource; the runtime publication registry derives from published studies and
  gates which derived rows are exportable (BK-DATABANK publication matching).
- `GET /fhir/ResearchStudy` — list read by both the admin toggle screen and
  the researcher export request (item 6, export only against *published*
  studies).

Frontend changes:

- Admin/researcher study detail — a Publish / Unpublish toggle calling `PUT
  /fhir/ResearchStudy/{id}` with optimistic update plus rollback on failure;
  unpublished studies disappear from researcher-facing export requests (the
  backend denies export for a non-published study).
