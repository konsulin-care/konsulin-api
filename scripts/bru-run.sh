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
#   BLAZE_BASE_URL      Blaze base for the post-run litter cleanup (default
#                       http://localhost:8080; the collection .env may set it)

REQUIRED=0
if [[ "${1:-}" == "--required" ]]; then
  REQUIRED=1
fi

# Resolve the script's own directory before any `cd` below, so the decoupled
# cleanup can always be found regardless of the current working directory.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

COLLECTION_DIR="${BRU_COLLECTION_DIR:-docs/api}"
ENV_FILE="${COLLECTION_DIR}/.env"

if [[ ! -f "${ENV_FILE}" ]]; then
  if [[ "${REQUIRED}" -eq 1 ]]; then
    echo "ERROR: ${ENV_FILE} not found — copy docs/api/.env.example to docs/api/.env" >&2
    exit 1
  fi
  echo "SKIP: ${ENV_FILE} not found (copy docs/api/.env.example to docs/api/.env to run Bruno tests)"
  exit 0
fi

# Refuse to load a tracked .env — a committed/force-added env file must never
# be trusted (or executed) by hooks.
if git ls-files --error-unmatch "${ENV_FILE}" >/dev/null 2>&1; then
  echo "ERROR: ${ENV_FILE} is tracked by git — refusing to load a tracked .env" >&2
  exit 1
fi

# Load collection env vars (APP_BASE_URL, SUPERADMIN_API_KEY, ORGANIZATION).
# Non-evaluating parse: only valid KEY=value lines are exported (one layer of
# surrounding quotes stripped); blanks, comments, and any line that is not a
# plain KEY=value assignment are ignored. Nothing is ever executed as shell.
while IFS='=' read -r key value; do
  case "$key" in
    ''|'#'*) continue ;;
    *) : ;; # fall through: validate and export below
  esac
  [[ "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue
  value="${value#\"}"; value="${value%\"}"
  value="${value#\'}"; value="${value%\'}"
  export "$key=$value"
done < "${ENV_FILE}"

APP_BASE_URL="${APP_BASE_URL//\"/}" # strip surrounding quotes if present
if [[ -z "${APP_BASE_URL}" ]]; then
  echo "ERROR: APP_BASE_URL not set in ${ENV_FILE}" >&2
  exit 1
fi

HEALTH_URL="${APP_BASE_URL%/}/health"

if RESPONSE="$(curl -sf --max-time 5 "${HEALTH_URL}")"; then
  echo "API healthy at ${HEALTH_URL}: ${RESPONSE}"
  cd "${COLLECTION_DIR}"
  BRU_RC=0
  if ! bru run --bail; then
    BRU_RC=$?
  fi
  # Decoupled cleanup: runs after the suite whether it passed or failed, so a
  # mid-run failure can never leave the fixed-id seed resources behind. The
  # cleanup is Blaze-direct and non-fatal (see scripts/bru-cleanup.sh).
  "${SCRIPT_DIR}/bru-cleanup.sh" || true
  exit "${BRU_RC}"
else
  if [[ "${REQUIRED}" -eq 1 ]]; then
    echo "FAIL: API server not reachable at ${HEALTH_URL} — refusing to push untested API" >&2
    exit 1
  fi
  echo "SKIP: API server not reachable at ${HEALTH_URL} (start the server to run Bruno tests)"
fi
