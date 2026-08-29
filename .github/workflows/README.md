# CI/CD Pipelines

Deployment and PR gating are fully automated via GitHub Actions.

## Flow

```
push to develop/main
      │
      ▼
main.yml ──► docker-build.yml ──► trigger-coolify.yml ──► deploy.yml
 (orchestrator)  (build & push image)   (Coolify webhook)   (SSH status check)

pull request to develop/main
      │
      ▼
pr.yml ──► quality / static-security / codeql / integration / build
 (authoritative PR gate; all jobs are required checks)

weekly (cron Mon 03:00 UTC)
      │
      ▼
security-weekly.yml ──► zap-and-regression + fuzz jobs
```

## Workflows

### PR gate — `pr.yml` (authoritative, cannot be bypassed with `--no-verify`)

Every pull request to `develop`/`main` runs five independent jobs. **All five
are configured as required status checks** in branch protection for
`develop`/`main`, so a PR cannot merge unless every gate is green.

### Fork pull requests

The `integration` job references the `Pull Request Screening` environment to
receive its CI credentials, and GitHub does **not** expose any secrets
(repository or environment) to workflows triggered by `pull_request` events
from forks. The job therefore **skips itself on fork PRs**
(`github.event.pull_request.head.repo.full_name == github.repository`):

- Same-repo PRs (feature branches pushed to `konsulin-care/konsulin-api`): the
  full suite runs with the environment secrets resolved.
- Fork PRs (external contributors): the job shows as skipped instead of
  failing on missing secrets. The suite still runs for those commits after
  merge, on `develop`, where the environment is available.

Keep secret-using jobs behind this guard; never switch them to
`pull_request_target` to "fix" fork coverage — that runs base-branch workflow
code with fork-influenced inputs and is a known secret-leak vector.

| Job | Checks |
|---|---|
| `quality` | gofumpt (changed files), `go mod tidy`, golangci-lint (new issues), `go vet`, `go test`, `go test -race` |
| `static-security` | govulncheck (blocking), Trivy vuln scan (blocking, HIGH/CRITICAL), Trivy secrets/config (SARIF, non-blocking) |
| `codeql` | CodeQL Go analysis (SARIF into Code Scanning) |
| `integration` | disposable Docker env (`ci-env` action) + full Bruno suite (auth, RBAC, ownership-violation). An expected-4xx ownership test that starts returning 2xx fails the PR. **Skipped on fork PRs** (see above). |
| `build` | vendor + app image build, then a blocking Trivy **image** scan (OS-level CVEs) with SARIF |

### Weekly security — `security-weekly.yml` (scheduled, non-blocking for PRs)

Runs Mondays 03:00 UTC on the default branch (also `workflow_dispatch`):

- **`zap-and-regression` job**: disposable env → full Bruno regression suite →
  OWASP ZAP active API scan (informational, SARIF) → govulncheck + Trivy
  freshness scans (SARIF). ZAP *complements* the Bruno authorization tests; it
  never replaces them.
- **`fuzz` job**: coverage-guided Go fuzzing over the service-layer parse
  entry points (FHIR bundle/invoice responses, Xendit callbacks, webhook
  bodies), 2m budget per package. A crasher fails the run — triage the saved
  `testdata/fuzz` corpus and fix the bug.

Active DAST and fuzzing stay off the blocking PR path: the PR gate must remain
fast and deterministic.

### Deployment — `main.yml`, `docker-build.yml`, `trigger-coolify.yml`, `deploy.yml`

- `main.yml` — orchestrator on push to `develop`/`main`: builds, deploys to
  dev (`develop`) or prod (`main`), then verifies the container is running.
- `docker-build.yml` — reusable build: vendor image, then app image (dev:
  `nightly`/`dev-$SHA`; prod: `latest`/git tag), pushes to Docker Hub.
- `trigger-coolify.yml` — reusable deploy trigger (Coolify restart webhook).
- `deploy.yml` — reusable SSH container status check.

## Required repository secrets

| Secret | Used by | Scope |
|---|---|---|
| `DOCKER_USERNAME`, `DOCKER_PASSWORD` | `docker-build.yml` | repo |
| `COOLIFY_URL`, `COOLIFY_SERVICE_DEV`, `COOLIFY_SERVICE_PROD`, `COOLIFY_TOKEN` | `main.yml` → `trigger-coolify.yml` | repo |
| `SSH_HOST`, `SSH_USERNAME`, `SSH_KEY` | `main.yml` → `deploy.yml` | repo |
| `XENDIT_SANDBOX_API_KEY` | `pr.yml` / `security-weekly.yml` (integration env) | repo |
| `CI_POSTGRES_PASSWORD` | `ci-env` action → `.env.ci` (postgres + SuperTokens URI) | env `Pull Request Screening` |
| `CI_REDIS_PASSWORD` | `ci-env` action → `.env.ci` (redis + gateway sessions) | env `Pull Request Screening` |
| `CI_RABBITMQ_PASSWORD` | `ci-env` action → `.env.ci` (rabbitmq) | env `Pull Request Screening` |
| `CI_SUPERTOKEN_API_KEY` | `ci-env` action → `.env.ci` (SuperTokens core + SDK) | env `Pull Request Screening` |
| `CI_SUPERADMIN_API_KEY` | `ci-env` action → `.env.ci` + `docs/api/.env` (Bruno admin) | env `Pull Request Screening` |
| `CI_XENDIT_CALLBACK_TOKEN` | `ci-env` action → `.env.ci` + `docs/api/.env` (Bruno callbacks) | env `Pull Request Screening` |
| `CI_JWT_HOOK_KEY` | `ci-env` action → job env (webhook JWT signing) | env `Pull Request Screening` |

Environment secrets live under **Settings → Environments → "Pull Request
Screening"**; only jobs that declare `environment: "Pull Request Screening"`
(`integration`, `zap-and-regression`) can read them.

## Tooling decisions and boundaries

- **Semgrep runs via the GitHub App, not CI.** The Semgrep GitHub App posts
  findings on every open PR through its own check run, so a CI step would be
  redundant. Configured app behavior lives in the Semgrep dashboard.
- **DeepSource is retained deliberately.** It previously found issues that
  DeepScan, Codacy, and SonarCloud all missed; this demonstrated value is the
  topic-level exception to tool consolidation (config: `.deepsource.toml`).
  Revisit only if it starts producing duplicate/low-value findings.
- **CodeRabbit** remains the PR review assistant (`.coderabbit.yaml`), not a
  blocking gate.

### Pinning policy

Security-relevant tool versions are pinned, not floating:

| Tool | Pin | Kept fresh by |
|---|---|---|
| `gofumpt` | `v0.11.0` | dependabot (`go.mod` via `tools/tools.go`) |
| `govulncheck` | `v1.7.0` | dependabot (`go.mod` via `tools/tools.go`) |
| `trivy-action` | commit SHA `ed142fd…` `# v0.36.0` | dependabot (github-actions) |
| Trivy binary version | `v0.74.0` (action input) | manual |
| `actionlint` (pre-commit) | `v1.7.12` | dependabot updates the rev via pre-commit autoupdate manually — treat as manual |
| ZAP image | `ghcr.io/zaproxy/zaproxy:v2.17.0` | manual |

**SHA + tag comment pattern.** `trivy-action` is pinned to a full commit SHA
with a `# v0.36.0` comment because the March 2026 supply-chain incident
published a malicious Trivy `v0.69.4` release and re-tagged action refs. The
SHA makes the pin immutable; the tag comment lets Dependabot (github-actions
ecosystem) bump it to the next release. Follow this pattern for any action
that pulls a binary or image.

**Dependabot boundaries.** Dependabot updates: GitHub Actions refs (including
SHA-pinned-with-comment), Docker images inside `Dockerfile`s, and `go.mod`
modules. CI tools (`gofumpt`, `govulncheck`) are pinned in `go.mod`
(`tools/tools.go` + `tool` directives) and invoked via `go tool`, so dependabot
keeps them fresh as regular modules. Image tags inside `docker run` inputs
(e.g. the ZAP image) are bumped manually.

## Validation strategy

- `actionlint` (pre-commit hook) validates every workflow on each commit.
- `act` is a manual smoke test for workflow edits — **not** wired into hooks,
  because it cannot fully emulate GitHub and would gate pushes on false
  negatives.
- The authoritative check is the PR itself: for `pull_request` events GitHub
  executes the head-branch version of the workflow.
