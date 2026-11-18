#!/bin/sh
# Reject files above a size limit. Replaces the check-added-large-files hook
# from the Python pre-commit-hooks package (same 500 KB default).
#
# Usage: sh scripts/lint/check-large-files.sh [FILE...]
# With no arguments every tracked file is checked. The limit is overridable
# through LARGE_FILE_MAX_KB. Exits 1 listing every oversized file.
set -eu

cd "$(dirname "$0")/../.."

max_kb="${LARGE_FILE_MAX_KB:-500}"
max_bytes=$((max_kb * 1024))

if [ "$#" -eq 0 ]; then
	# shellcheck disable=SC2046 # tracked paths in this repo never contain spaces
	set -- $(git ls-files)
fi

failed=0
for f in "$@"; do
	[ -f "$f" ] || continue
	size=$(wc -c <"$f" | tr -d ' ')
	if [ "$size" -gt "$max_bytes" ]; then
		if [ "$failed" -eq 0 ]; then
			printf 'files above %s KB:\n' "$max_kb" >&2
		fi
		printf '  %s (%s KB)\n' "$f" "$((size / 1024))" >&2
		failed=1
	fi
done

exit "$failed"
