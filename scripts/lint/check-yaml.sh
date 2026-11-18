#!/bin/sh
# Validate YAML files by parsing them with yq. Replaces the check-yaml hook
# from the Python pre-commit-hooks package. yq accepts custom tags (for
# example Ansible's !vault), so the old --unsafe escape hatch is not needed.
#
# Usage: sh scripts/lint/check-yaml.sh [FILE...]
# With no arguments every tracked YAML file is checked. Non-YAML arguments are
# ignored. Exits 1 listing every file that fails to parse.
set -eu

cd "$(dirname "$0")/../.."

if [ "$#" -eq 0 ]; then
	# shellcheck disable=SC2046 # tracked paths in this repo never contain spaces
	set -- $(git ls-files '*.yml' '*.yaml')
fi

failed=0
for f in "$@"; do
	case "$f" in
	*.yml | *.yaml) ;;
	*) continue ;;
	esac
	[ -f "$f" ] || continue
	if ! yq e '.' "$f" >/dev/null 2>&1; then
		if [ "$failed" -eq 0 ]; then
			echo "invalid YAML:" >&2
		fi
		echo "  $f" >&2
		failed=1
	fi
done

exit "$failed"
