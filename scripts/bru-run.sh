#!/usr/bin/env bash
set -euo pipefail

# Runs the Bruno API collection (docs/api) as a commit/push gate.
#
# Usage:
#   bash scripts/bru-run.sh            # pre-commit: skip if API server is down
#   bash scripts/bru-run.sh --required # pre-push: fail if API server is down
#
# Environment:
#   BRU_COLLECTION_DIR  override the collection directory (default: docs/api)

REQUIRED=0
if [[ "${1:-}" == "--required" ]]; then
  REQUIRED=1
fi

COLLECTION_DIR="${BRU_COLLECTION_DIR:-docs/api}"
ENV_FILE="${COLLECTION_DIR}/.env"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "ERROR: ${ENV_FILE} not found — copy docs/api/.env.example to docs/api/.env" >&2
  exit 1
fi

# Load collection env vars (APP_BASE_URL, SUPERADMIN_API_KEY, ORGANIZATION)
set -a
# shellcheck disable=SC1091
source "${ENV_FILE}"
set +a

APP_BASE_URL="${APP_BASE_URL//\"/}" # strip surrounding quotes if present
if [[ -z "${APP_BASE_URL}" ]]; then
  echo "ERROR: APP_BASE_URL not set in ${ENV_FILE}" >&2
  exit 1
fi

HEALTH_URL="${APP_BASE_URL%/}/health"

if RESPONSE="$(curl -sf --max-time 5 "${HEALTH_URL}")"; then
  echo "API healthy at ${HEALTH_URL}: ${RESPONSE}"
  cd "${COLLECTION_DIR}"
  bru run --bail
else
  if [[ "${REQUIRED}" -eq 1 ]]; then
    echo "FAIL: API server not reachable at ${HEALTH_URL} — refusing to push untested API" >&2
    exit 1
  fi
  echo "SKIP: API server not reachable at ${HEALTH_URL} (start the server to run Bruno tests)"
fi
