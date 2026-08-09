# CI/CD Pipeline

Deployment is fully automated via GitHub Actions and Coolify.

## Flow

```
push to develop/main
      │
      ▼
main.yml ──► docker-build.yml ──► trigger-coolify.yml ──► deploy.yml
 (orchestrator)  (build & push image)   (Coolify webhook)   (SSH status check)
```

## Workflows

- **`main.yml`** — Entry point. Triggered on push to `develop` or `main`.
  - `docker` job: calls `docker-build.yml` with version/build inputs.
  - `deploy-dev` job: calls `trigger-coolify.yml` on `develop` pushes.
  - `deploy-prod` job: calls `trigger-coolify.yml` on `main` pushes.
  - `deploy-check` job: calls `deploy.yml` to verify the deployed container is running.

- **`docker-build.yml`** — Reusable build job. Builds the vendor image, rewrites the
  Dockerfile `FROM` to it, builds the app image, and pushes to Docker Hub
  (`nightly`/dev-sha for develop, `latest`/git-tag for main).

- **`trigger-coolify.yml`** — Reusable deploy job. Calls the Coolify deploy webhook
  (`COOLIFY_TRIGGER_URL`) with a bearer token (`COOLIFY_TOKEN`).

- **`deploy.yml`** — Reusable verification job. SSHes into the host and runs
  `docker ps | grep konsulin-api` to confirm the container is up.

- **`pr-build-check.yml`** — PR gate for pull requests to `develop`: golangci-lint,
  `go test -race ./...`, and a Docker build smoke check.

## Required repository secrets

- `DOCKER_USERNAME`, `DOCKER_PASSWORD` — Docker Hub credentials (used by `docker-build.yml`).
- `COOLIFY_TRIGGER_URL_DEV`, `COOLIFY_TOKEN_DEV` — Coolify dev deploy (used by `main.yml`).
- `COOLIFY_TRIGGER_URL_PROD`, `COOLIFY_TOKEN_PROD` — Coolify prod deploy (used by `main.yml`).
- `SSH_HOST`, `SSH_USERNAME`, `SSH_KEY` — host access for the container status check
  (used by `main.yml` via `deploy.yml`).
