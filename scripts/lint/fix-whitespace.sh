#!/bin/sh
# Strip trailing spaces and tabs from every line, in place. Replaces the
# trailing-whitespace hook from the Python pre-commit-hooks package.
#
# Usage: sh scripts/lint/fix-whitespace.sh [FILE...]
# With no arguments every tracked file is checked. CRLF files keep their CRLF
# line endings. Modified files are reported as "fixed: <file>".
set -eu

cd "$(dirname "$0")/../.."

if [ "$#" -eq 0 ]; then
	# shellcheck disable=SC2046 # tracked paths in this repo never contain spaces
	set -- $(git ls-files)
fi

for f in "$@"; do
	[ -f "$f" ] || continue
	# Skip binary and empty files: grep -I refuses binary, empty has no match.
	grep -Iq . "$f" 2>/dev/null || continue

	tmp=$(mktemp)
	# A trailing CR stays part of $0 so CRLF files are not rewritten to LF.
	awk 'BEGIN { ORS = "\n" }
		{
			cr = ""
			if (sub(/\r$/, "")) cr = "\r"
			sub(/[ \t]+$/, "")
			print $0 cr
		}' "$f" >"$tmp"

	if ! cmp -s "$f" "$tmp"; then
		cat "$tmp" >"$f"
		echo "fixed: $f"
	fi
	rm -f "$tmp"
done

exit 0
