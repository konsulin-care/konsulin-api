# Self-Hosting Against the Konsulin Gateway

Onboarding is operator-approved. The Konsulin operator creates your party and issues a single-use enrollment token. You generate keys locally; they never go in files or chat.

## Ask the operator for

- `APP_GATEWAY_URL` (https)
- `APP_PARTY_ID`
- `APP_PARTY_ENROLLMENT_TOKEN` (single-use; expiry TBD with the operator)
- `APP_AUTOMATION_PARTY_ID`

## Coolify flow

1. Locally run `mise run gateway --init`. Copy the two private keys (`APP_PARTY_PRIVATE_KEY`, `APP_PARTY_ENC_PRIVATE_KEY`) straight into Coolify. Keys print to stdout only.
2. Request the operator values above.
3. In Coolify set `APP_ENV=production`, an `https://` `APP_GATEWAY_URL`, the party ID, enrollment token, automation party ID and the remaining variables from `.env.example`.
4. Deploy. Then put the Coolify env in a temporary file, run `mise run verify <file> --federation`, and delete the file. Remove `APP_PARTY_ENROLLMENT_TOKEN` from Coolify once enrollment succeeded.

Plain-http gateway URLs fail in any non-dev `APP_ENV`, production included.

## Reading verify output

`mise run verify` checks whether the given `.env` can boot the system. Only `FAIL`/`WARN` lines and the exit status are verification results.

Lines like `[env] X is not set; using its built-in default` are informational: the variable has a default and the env did not override it.

Optional variables and their defaults:

- `APP_RBAC_POLICY_PATH`: `resources/rbac_policy.csv`
- `APP_ORGANIZATION_NAME`: `Konsulin`
- `APP_DATA_DIR`: `./data`
- `APP_AUTOPUBLISH`: `false`
- `APP_RESEARCH_EXPORT_MAX_BYTES`: `52428800`
- `APP_RESEARCH_PSEUDONYM_KEY`: empty, which disables the databank

`APP_JWT_SECRET` should be at least 32 characters.

## Persistent data

`APP_DATA_DIR` (image default `/app/data`) holds:

- `federation-pepper.jwe`: refetched at startup.
- `research.db`: not recoverable if lost. Back it up.

Mount a persistent volume there, owned by uid 1000 (the non-root `app` user). Boot fails naming `APP_DATA_DIR` if it is not writable.

## Commands

See `cmd/party` (`keygen`, `enroll`, `verify`, `rotate`, `enlist`) for the party CLI.
