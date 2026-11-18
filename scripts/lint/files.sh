#!/bin/sh
# File hygiene entry point: EOF newline, trailing whitespace, large files and
# YAML parsing. Every check runs; the exit status is non-zero when any of the
# checking (non-fixing) steps fails.
#
# Usage: sh scripts/lint/files.sh [FILE...]
# With no arguments every tracked file is checked.
set -eu

dir=$(dirname "$0")

rc=0
sh "$dir/fix-eof.sh" "$@" || rc=1
sh "$dir/fix-whitespace.sh" "$@" || rc=1
sh "$dir/check-large-files.sh" "$@" || rc=1
sh "$dir/check-yaml.sh" "$@" || rc=1

exit "$rc"
