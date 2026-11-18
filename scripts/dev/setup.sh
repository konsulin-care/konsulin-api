#!/bin/sh
# First-time setup: create .env if missing, start stack, wait for health.
set -eu

cd "$(dirname "$0")/../.."

if [ -f .env ]; then
	echo ".env exists, leaving it untouched"
else
	cp .env.example .env
	echo "created .env from .env.example"
fi

# Enable the git hooks (lefthook) so commits and pushes run the local gates.
mise run hooks:install

sh scripts/dev/stack.sh --up
docker compose up -d --wait
sh scripts/dev/stack.sh --status
