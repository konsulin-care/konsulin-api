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
// Input: JSON report on stdin (pipe or redirect from bru-run.sh).
//
// Exit codes:
//   0  suite passed,
//   1  suite failed OR input is missing / unparseable (fail closed).

let chunks = '';
process.stdin.setEncoding('utf-8');
process.stdin.on('data', (chunk) => { chunks += chunk; });
process.stdin.on('end', () => {
  if (!chunks) {
    console.error('bru-report-gate: no input on stdin');
    process.exit(1);
  }

  let reports;
  try {
    reports = JSON.parse(chunks);
  } catch (err) {
    console.error(`bru-report-gate: cannot parse input: ${err.message}`);
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
});
