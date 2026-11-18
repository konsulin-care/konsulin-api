#!/bin/sh
# golangci-lint gate. Default mode lints only the change against HEAD
# (--new-from-patch), matching CI's only-new-issues policy; --base lints
# everything changed since the merge-base with a ref; --full lints the module.
#
# Usage: sh scripts/lint/golangci.sh [--base REF] [--full]
# Flags are also read from the mise usage env vars usage_base / usage_full.
set -eu

cd "$(dirname "$0")/../.."

base="${usage_base:-}"
full="${usage_full:-false}"

while [ "$#" -gt 0 ]; do
	case "$1" in
	--base)
		[ "$#" -ge 2 ] || {
			echo "--base needs a ref" >&2
			exit 2
		}
		base=$2
		shift 2
		;;
	--base=*)
		base="${1#*=}"
		shift
		;;
	--full)
		full=true
		shift
		;;
	*)
		echo "unknown flag: $1" >&2
		exit 2
		;;
	esac
done

if [ "$full" = "true" ]; then
	rc=0
	CGO_ENABLED=0 golangci-lint run --timeout=5m || rc=$?
	exit "$rc"
fi

if [ -n "$base" ]; then
	base=$(git merge-base HEAD "$base" 2>/dev/null || echo HEAD)
else
	base=HEAD
fi

patch=$(mktemp)
trap 'rm -f "$patch"' EXIT INT TERM

git diff "$base" -- . ':(exclude).pi/sandbox.json' >"$patch"

rc=0
CGO_ENABLED=0 golangci-lint run --new-from-patch="$patch" --timeout=5m || rc=$?
exit "$rc"
