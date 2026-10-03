#!/usr/bin/env python3
"""Summarize a Go coverage profile read from standard input.

Every coverage block is counted, including package-level function literals, and
duplicate blocks emitted by -coverpkg runs are merged. When a unified diff
follows the profile, the statements on changed lines are reported as well:

    python3 scripts/ci/verify_go_coverage.py < coverage.out
    { cat coverage.out; git diff --unified=0 "$base" HEAD -- '*.go'; } |
        python3 scripts/ci/verify_go_coverage.py --module "$(go list -m)" --changed-minimum 100

Input is read only from standard input, so no command-line value selects a file.
"""

import argparse
import re
import sys
from collections import defaultdict
from decimal import Decimal

MODES = {"mode: set", "mode: count", "mode: atomic"}
DIFF_START = "diff --git "
LOCATION = re.compile(r"^(.+\.go):(\d+)\.\d+,(\d+)\.\d+$")
HUNK = re.compile(r"^@@ -\d+(?:,(\d+))? \+(\d+)(?:,(\d+))? @@")


def percent(value: str) -> Decimal:
    try:
        number = Decimal(value)
    except ArithmeticError:
        raise argparse.ArgumentTypeError(f"invalid percentage: {value!r}") from None
    if not number.is_finite() or not Decimal(0) <= number <= Decimal(100):
        raise argparse.ArgumentTypeError("percentage must be between 0 and 100")
    return number


def split_input(text: str) -> tuple[list[str], list[str]]:
    """Split standard input into the coverage profile and an optional trailing diff."""
    lines = text.splitlines()
    for index, line in enumerate(lines):
        if line.startswith(DIFF_START):
            return lines[:index], lines[index:]
    return lines, []


def parse_location(location: str) -> tuple[str, int, int]:
    match = LOCATION.match(location)
    if not match:
        raise ValueError("invalid coverage block")
    return match.group(1), int(match.group(2)), int(match.group(3))


def parse_profile(lines: list[str]) -> dict[str, tuple[int, int]]:
    if not lines or lines[0] not in MODES:
        raise ValueError("missing or invalid coverage mode")
    blocks = {}
    for line in lines[1:]:
        location, count, hits = line.rsplit(maxsplit=2)
        count, hits = int(count), int(hits)
        if count < 0 or hits < 0:
            raise ValueError("invalid coverage block")
        parse_location(location)
        old_count, old_hits = blocks.get(location, (count, 0))
        if old_count != count:
            raise ValueError("inconsistent duplicate block")
        blocks[location] = count, old_hits + hits
    if not sum(count for count, _ in blocks.values()):
        raise ValueError("coverage profile has no statements")
    return blocks


def summarize(blocks: dict[str, tuple[int, int]]) -> dict[str, tuple[int, int]]:
    packages = defaultdict(lambda: [0, 0])
    for location, (count, hits) in blocks.items():
        package = parse_location(location)[0].rsplit("/", 1)[0]
        packages[package][1] += count
        if hits:
            packages[package][0] += count
    return {package: tuple(counts) for package, counts in packages.items()}


def production_path(target: str):
    """Return the repository path of a changed non-test Go file, or None."""
    if not target.startswith("b/"):
        return None
    path = target[2:]
    return path if path.endswith(".go") and not path.endswith("_test.go") else None


def added_lines(header, lines) -> list[int]:
    """Consume one hunk body from lines and return the new-side numbers it adds."""
    old_left, line_number, new_left = int(header.group(1) or 1), int(header.group(2)), int(header.group(3) or 1)
    added = []
    while old_left > 0 or new_left > 0:
        line = next(lines, None)
        if line is None:
            break
        if line.startswith("+"):
            added.append(line_number)
            line_number, new_left = line_number + 1, new_left - 1
        elif line.startswith("-"):
            old_left -= 1
        elif line.startswith(" "):
            line_number, old_left, new_left = line_number + 1, old_left - 1, new_left - 1
    return added


def changed_lines(lines: list[str]) -> dict[str, set[int]]:
    """Map each changed production Go file to the new-side line numbers it adds."""
    changes = defaultdict(set)
    path = None
    remaining = iter(lines)
    for line in remaining:
        if line.startswith("+++ "):
            path = production_path(line[4:])
        header = HUNK.match(line)
        if header:
            added = added_lines(header, remaining)
            if path and added:
                changes[path].update(added)
    return dict(changes)


def changed_coverage(blocks: dict[str, tuple[int, int]], changes: dict[str, set[int]], module: str):
    """Return covered and total statements on changed lines, plus the uncovered blocks."""
    prefix = module.rstrip("/") + "/"
    covered = total = 0
    missing = []
    for location, (count, hits) in sorted(blocks.items()):
        file, start, end = parse_location(location)
        lines = changes.get(file[len(prefix):]) if file.startswith(prefix) else None
        if not count or not lines or lines.isdisjoint(range(start, end + 1)):
            continue
        total += count
        if hits:
            covered += count
        else:
            missing.append((file[len(prefix):], start, end, count))
    return covered, total, missing


def meets(covered: int, total: int, minimum: Decimal) -> bool:
    # Compare exactly so 99.999% never rounds up to a 100% requirement.
    return Decimal(covered) * 100 >= minimum * total


def report_total(packages, minimum: Decimal, out) -> bool:
    covered = sum(count for count, _ in packages.values())
    total = sum(count for _, count in packages.values())
    print(f"Go statement coverage: {Decimal(covered) * 100 / total:.2f}% ({covered}/{total})", file=out)
    print("\n| Package | Covered / total | Coverage |\n|---|---:|---:|", file=out)
    for package, (count, size) in sorted(packages.items()):
        coverage = Decimal(count) * 100 / size if size else Decimal(100)
        print(f"| {package} | {count} / {size} | {coverage:.2f}% |", file=out)
    if meets(covered, total, minimum):
        return True
    print(f"\nCoverage is below the required {minimum}%.", file=out)
    return False


def report_changed(blocks, diff: list[str], module: str, minimum: Decimal, out) -> bool:
    covered, total, missing = changed_coverage(blocks, changed_lines(diff), module)
    if not total:
        print("\nChanged Go statement coverage: no changed statements.", file=out)
        return True
    print(f"\nChanged Go statement coverage: {Decimal(covered) * 100 / total:.2f}% ({covered}/{total})", file=out)
    if missing:
        print("\n| Uncovered changed block | Statements |\n|---|---:|", file=out)
        for file, start, end, count in missing:
            print(f"| {file}:{start}-{end} | {count} |", file=out)
    if meets(covered, total, minimum):
        return True
    print(f"\nChanged-line coverage is below the required {minimum}%.", file=out)
    return False


def main(argv=None, stdin=None, stdout=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--minimum", type=percent, default=Decimal(0), help="required total statement coverage")
    parser.add_argument("--changed-minimum", type=percent, default=Decimal(0), help="required changed-line coverage")
    parser.add_argument("--module", default="", help="Go module path that prefixes profile file names")
    args = parser.parse_args(argv)
    out = stdout or sys.stdout
    profile, diff = split_input((stdin or sys.stdin).read())
    if diff and not args.module:
        parser.error("--module is required when a diff follows the profile")
    try:
        blocks = parse_profile(profile)
    except ValueError as error:
        parser.error(str(error))
    passed = report_total(summarize(blocks), args.minimum, out)
    if diff:
        passed = report_changed(blocks, diff, args.module, args.changed_minimum, out) and passed
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
