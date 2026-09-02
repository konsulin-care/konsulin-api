#!/usr/bin/env node
//
// Bruno report gate — decides whether a `bru run --reporter-json` report means
// the suite passed.
//
// The bru CLI writes the report as a JSON *array* of per-iteration results:
//   [ { iterationIndex, results, summary } ]
// An earlier inline `r.summary` read (array root) always saw `undefined`,
// computed 0 and let every failing PR check go green. This module normalises
// both array-root and object-root reports and exits 1 when any iteration's
// summary carries failures, or when `--bail` skipped requests.
//
// Interprets `skippedByBail` as a hard failure: a bailed run stopped early, so
// it must never be reported as green even if every recorded count is 0.
//
// Exit codes:
//   0  suite passed,
//   1  suite failed OR the report file is missing / unparseable (fail closed),
//   2  usage error.

import { readFileSync } from 'node:fs';
import { resolve, sep } from 'node:path';
import { tmpdir } from 'node:os';

const file = process.argv[2];
if (!file) {
  console.error('usage: bru-report-gate.mjs <bru-report.json>');
  process.exit(2);
}

// Only read reports inside the working directory (bru-run.sh writes
// bru-report.json in the collection dir) or the temp dir (test fixtures), so
// a faulty CLI argument cannot escape the file system sandbox.
const reportPath = resolve(file);
const safeRoots = [resolve('.'), resolve(tmpdir())];
const isSafe = safeRoots.some(
  (root) => reportPath === root || reportPath.startsWith(root + sep),
);
if (!isSafe) {
  console.error(`bru-report-gate: refusing to read report outside working/temp dirs: ${file}`);
  process.exit(2);
}

let reports;
try {
  reports = JSON.parse(readFileSync(reportPath, 'utf-8'));
} catch (err) {
  console.error(`bru-report-gate: cannot read or parse ${reportPath}: ${err.message}`);
  process.exit(1);
}

const list = Array.isArray(reports) ? reports : [reports];
const failed = list.reduce((acc, it) => {
  const s = it?.summary ?? {};
  return acc
    + (s.failedAssertions || 0)
    + (s.failedTests || 0)
    + (s.failedRequests || 0)
    + (s.errorRequests || 0)
    + (s.skippedByBail || 0);
}, 0);

process.exit(failed > 0 ? 1 : 0);
