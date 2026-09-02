#!/usr/bin/env bash
set -euo pipefail

# Runs the Bruno API collection (docs/api) as a commit/push gate.
#
# Usage:
#   bash scripts/bru-run.sh                 # run all; skip if API server is down
#   bash scripts/bru-run.sh --required      # fail if API server is down
#   bash scripts/bru-run.sh --required --skip-cleanup
#                                           # full run without the chained teardown
#   bash scripts/bru-run.sh --required --tag PR
#                                           # run only requests tagged `PR`
#
# --tag <T> narrows the run to requests carrying tag <T> (a single Bruno
# `--tags=<T>` flag). Omit it to run the whole collection. Teardown of seeded
# FHIR resources is handled by scripts/bru-cleanup.sh after the suite.
#
# --skip-cleanup (and any --tag) pass --env-var skipCleanup=true to Bruno, so
# the journey's final request (`Admin: Update Questionnaire`) terminates the
# run instead of chaining into the teardown requests — which a tag-filtered
# run does not contain and would otherwise dangle on (Bruno prints "Could not
# find request" and advances to the next run-set request, looping the
# ownership→patient→practitioner→admin sub-chain forever). Untagged runs
# without --skip-cleanup keep the full chained teardown + "Sign Out".
#
# Environment:
#   BRU_COLLECTION_DIR  override the collection directory (default: docs/api)
#   BLAZE_BASE_URL      Blaze base for the post-run litter cleanup (default
#                       http://localhost:8080; the collection .env may set it)

REQUIRED=0
TAG=""
SKIP_CLEANUP=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --required) REQUIRED=1; shift ;;
    --skip-cleanup) SKIP_CLEANUP=1; shift ;;
    --tag)
      TAG="${2:-}"
      [[ -n "${TAG}" ]] || { echo "ERROR: --tag requires a value" >&2; exit 1; }
      shift 2
      ;;
    *) shift ;;
  esac
done

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

# Direct-Blaze seeds (docs/api/fhir/seed/*) target BLAZE_BASE_URL. Default to
# the standard dev Blaze port so the suite works even when the collection .env
# does not set it; mirrors the default in scripts/bru-cleanup.sh.
export BLAZE_BASE_URL="${BLAZE_BASE_URL:-http://localhost:8080}"

HEALTH_URL="${APP_BASE_URL%/}/health"

if RESPONSE="$(curl -sf --max-time 5 "${HEALTH_URL}")"; then
  echo "API healthy at ${HEALTH_URL}: ${RESPONSE}"
  cd "${COLLECTION_DIR}"

  # Run Bruno collection with JSON + JUnit reports for CI artifact upload.
  # --reporter-json and --reporter-junit always produce their files (even on
  # failure) so the workflow can upload them for debugging.
  BRU_REPORT="bru-report.json"
  BRU_JUNIT="bru-report.xml"
  CLI_RC=0
  BRU_ARGS=(run --bail --reporter-json "${BRU_REPORT}" --reporter-junit "${BRU_JUNIT}")
  if [[ -n "${TAG}" ]]; then
    BRU_ARGS+=(--tags="${TAG}")
  fi
  # A tag-filtered run never contains the chained teardown requests, so the
  # journey's final setNextRequest("Cleanup: ...") would dangle and loop the
  # runner; --tag therefore always implies skipping the chained teardown.
  if [[ "${SKIP_CLEANUP}" -eq 1 || -n "${TAG}" ]]; then
    BRU_ARGS+=(--env-var skipCleanup=true)
  fi
  bru "${BRU_ARGS[@]}" || CLI_RC=$?

  # Parse the report and fail CI if any assertions/tests/requests failed.
  # This guards against bru CLI exit-code regressions (issue #155) where
  # the process may exit 0 despite failures. scripts/bru-report-gate.mjs
  # understands the array-root report format ([{iterationIndex, results,
  # summary}]) and treats a bail (skippedByBail > 0) as a hard failure.
  BRU_RC=0
  if [[ -f "${BRU_REPORT}" ]]; then
    if command -v node >/dev/null 2>&1; then
      node "${SCRIPT_DIR}/bru-report-gate.mjs" "${BRU_REPORT}" || BRU_RC=$?
    else
      # Fallback: grep for failure indicators in the raw JSON (pretty-printed,
      # so allow optional whitespace around the colon).
      if grep -qE '"status"\s*:\s*"fail"' "${BRU_REPORT}" 2>/dev/null; then
        BRU_RC=1
      fi
    fi
    if [[ "${BRU_RC}" -ne 0 ]]; then
      echo "Bruno tests failed (see ${BRU_REPORT} for details)" >&2
    fi
  else
    echo "WARNING: ${BRU_REPORT} not produced — treating as failure" >&2
    BRU_RC=1
  fi

  # The CLI's own exit code is the backup signal: if bru itself bailed or
  # errored, fail regardless of what the report parse concluded.
  if [[ "${CLI_RC}" -ne 0 ]]; then
    BRU_RC=${CLI_RC}
  fi

  # Decoupled cleanup: runs after the suite whether it passed or failed, so a
  # mid-run failure can never leave the fixed-id seed resources behind. The
  # cleanup is Blaze-direct and non-fatal (see scripts/bru-cleanup.sh).
  bash "${SCRIPT_DIR}/bru-cleanup.sh" || true
  exit "${BRU_RC}"
else
  if [[ "${REQUIRED}" -eq 1 ]]; then
    echo "FAIL: API server not reachable at ${HEALTH_URL} — refusing to push untested API" >&2
    exit 1
  fi
  echo "SKIP: API server not reachable at ${HEALTH_URL} (start the server to run Bruno tests)"
fi
