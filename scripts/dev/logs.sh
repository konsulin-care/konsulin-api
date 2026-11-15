#!/bin/sh
# Follow docker compose logs. Optional service arg via mise (usage_service) or $1.
set -eu

cd "$(dirname "$0")/.."/..

service="${usage_service:-${1:-}}"

if [ -n "$service" ]; then
	exec docker compose logs -f --tail=100 "$service"
fi
exec docker compose logs -f --tail=100
