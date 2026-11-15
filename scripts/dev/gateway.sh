#!/bin/sh
# Gateway onboarding helper. Flags arrive as mise usage env vars.
set -eu

cd "$(dirname "$0")/../.."

init="${usage_init:-false}"
enroll="${usage_enroll:-false}"
dev="${usage_dev:-false}"

# Also accept plain CLI flags when run directly.
for arg in "$@"; do
	case "$arg" in
	--init) init=true ;;
	--enroll) enroll=true ;;
	--dev) dev=true ;;
	*)
		echo "unknown flag: $arg" >&2
		exit 2
		;;
	esac
done

load_env() {
	if [ -f .env ]; then
		set -a
		# shellcheck disable=SC1091
		. ./.env
		set +a
	fi
}

if [ "$init" = true ]; then
	go run ./cmd/party keygen
	echo "Reminder: put the private keys straight into your deploy env; do not save them to files." >&2
elif [ "$enroll" = true ]; then
	load_env
	go run ./cmd/party enroll
elif [ "$dev" = true ]; then
	if [ -z "${GATEWAY_URL:-}" ] || [ -z "${OPERATOR_PASSWORD:-}" ]; then
		echo "GATEWAY_URL and OPERATOR_PASSWORD must be set in the environment" >&2
		exit 1
	fi
	sh scripts/ci/gateway-bootstrap.sh
else
	echo "usage: gateway.sh --init | --enroll | --dev" >&2
	exit 2
fi
