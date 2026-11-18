#!/bin/sh
# gofumpt formatting gate with CI semantics: whole-file check (gofumpt -l),
# never --fix, so formatting problems are reported instead of rewritten.
#
# Usage: sh scripts/lint/gofumpt.sh [FILE...]
#        sh scripts/lint/gofumpt.sh --base REF
# File arguments are checked as given (non-.go paths are ignored); --base
# checks every .go file changed since the merge-base with REF; with neither,
# staged .go files are checked. Flags are also read from the mise usage env var
# usage_base.
set -eu

cd "$(dirname "$0")/../.."

base="${usage_base:-}"

list=$(mktemp)
trap 'rm -f "$list"' EXIT INT TERM
: >"$list"

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
	-*)
		echo "unknown flag: $1" >&2
		exit 2
		;;
	*)
		printf '%s\n' "$1" >>"$list"
		shift
		;;
	esac
done

if [ -s "$list" ]; then
	: # explicit file arguments
elif [ -n "$base" ]; then
	ref=$(git merge-base HEAD "$base" 2>/dev/null || echo HEAD)
	git diff --name-only "$ref" HEAD -- '*.go' >"$list"
else
	git diff --cached --name-only --diff-filter=ACM -- '*.go' >"$list"
fi

set --
while IFS= read -r f; do
	case "$f" in
	*.go)
		if [ -f "$f" ]; then
			set -- "$@" "$f"
		fi
		;;
	esac
done <"$list"

# Nothing to check (for example a docs-only change).
[ "$#" -gt 0 ] || exit 0

rc=0
violations=$(go tool gofumpt -l "$@") || rc=$?

if [ -n "$violations" ]; then
	printf 'gofumpt violations:\n%s\n' "$violations" >&2
	exit 1
fi

exit "$rc"
