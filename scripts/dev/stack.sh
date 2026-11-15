#!/bin/sh
# Docker stack lifecycle. Flags arrive as mise usage env vars.
set -eu

cd "$(dirname "$0")/../.."

up="${usage_up:-false}"
down="${usage_down:-false}"
status="${usage_status:-false}"
reset="${usage_reset:-false}"

# Also accept plain CLI flags when run directly.
for arg in "$@"; do
	case "$arg" in
	--up) up=true ;;
	--down) down=true ;;
	--status) status=true ;;
	--reset) reset=true ;;
	*)
		echo "unknown flag: $arg" >&2
		exit 2
		;;
	esac
done

if [ "$up" = true ]; then
	docker compose up -d
elif [ "$down" = true ]; then
	docker compose down
elif [ "$status" = true ]; then
	docker compose ps
elif [ "$reset" = true ]; then
	printf "This deletes all stack volumes. Type 'yes' to continue: "
	read -r answer
	if [ "$answer" != "yes" ]; then
		echo "aborted"
		exit 1
	fi
	docker compose down -v
else
	echo "usage: stack --up | --down | --status | --reset" >&2
	exit 2
fi
