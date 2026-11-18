#!/bin/sh
# Ensure every text file ends with exactly one newline: adds a missing final
# newline and trims extra trailing blank lines. Replaces the end-of-file-fixer
# hook from the Python pre-commit-hooks package.
#
# Usage: sh scripts/lint/fix-eof.sh [FILE...]
# With no arguments every tracked file is checked. CRLF files keep their CRLF
# line endings. Modified files are reported as "fixed: <file>".
set -eu

cd "$(dirname "$0")/../.."

if [ "$#" -eq 0 ]; then
	# shellcheck disable=SC2046 # tracked paths in this repo never contain spaces
	set -- $(git ls-files)
fi

fixed=0
for f in "$@"; do
	[ -f "$f" ] || continue
	# Skip binary and empty files: grep -I refuses binary, empty has no match.
	grep -Iq . "$f" 2>/dev/null || continue

	tmp=$(mktemp)
	# $0 keeps a trailing CR when the file uses CRLF, so CRLF survives the
	# rewrite. Trailing blank lines (with or without CR) are dropped.
	awk 'BEGIN { ORS = "\n" }
		{ lines[NR] = $0 }
		END {
			n = NR
			while (n > 0 && (lines[n] == "" || lines[n] == "\r")) n--
			for (i = 1; i <= n; i++) print lines[i]
		}' "$f" >"$tmp"

	if ! cmp -s "$f" "$tmp"; then
		cat "$tmp" >"$f"
		echo "fixed: $f"
		fixed=$((fixed + 1))
	fi
	rm -f "$tmp"
done

exit 0
