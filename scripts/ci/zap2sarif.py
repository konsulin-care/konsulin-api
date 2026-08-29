#!/usr/bin/env python3
"""Convert a ZAP JSON report (zap.core.jsonreport()) to SARIF 2.1.0.

Stdlib only, so the weekly security workflow can upload ZAP findings to GitHub
Code Scanning without extra tooling. zap-full-scan.py supports -J (JSON) but
not -S sarif in current releases.

Usage:
    python3 scripts/ci/zap2sarif.py zap-report.json zap-report.sarif
"""

import json
import sys
from pathlib import Path

RISK_LEVEL = {"0": "none", "1": "note", "2": "warning", "3": "error"}


def _resolve_ci_path(arg, allowed_exts):
    """Resolve a CLI-supplied path and require it to stay inside the workspace.

    SonarCloud flags file access with unvalidated argument-constructed paths
    (path escape). Rejects anything outside the current working directory and
    any file type outside the allowed suffix set.

    Returns the resolved Path.
    """
    p = Path(arg).expanduser().resolve()
    if not p.is_relative_to(Path.cwd().resolve()):
        raise SystemExit(f"refusing path outside workspace: {p}")
    if p.suffix not in allowed_exts:
        raise SystemExit(f"refusing unexpected file type ({p.suffix}): {p}")
    return p


def convert(zap_json, out_path):
    """Flatten ZAP's site[*].alerts[*].instances[] into SARIF results."""
    results = []
    rules = {}
    for site in zap_json.get("site", []):
        for alert in site.get("alerts", []):
            rule_id = str(alert.get("pluginid", ""))
            title = alert.get("alert") or f"ZAP alert {rule_id}"
            reference = alert.get("reference") or ""
            rules[rule_id] = {
                "id": rule_id,
                "name": title,
                "shortDescription": {"text": (alert.get("desc") or "")[:400]},
                "helpUri": reference.split()[0] if reference else None,
            }
            level = RISK_LEVEL.get(str(alert.get("riskcode", "0")), "none")
            for inst in alert.get("instances", []):
                uri = inst.get("uri") or "unknown"
                results.append(
                    {
                        "ruleId": rule_id,
                        "level": level,
                        "message": {"text": title},
                        "locations": [
                            {
                                "physicalLocation": {
                                    "artifactLocation": {"uri": uri}
                                }
                            }
                        ],
                    }
                )

    sarif = {
        "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
        "version": "2.1.0",
        "runs": [
            {
                "tool": {
                    "driver": {
                        "name": "ZAP",
                        "version": str(zap_json.get("@version", "")),
                        "informationUri": "https://www.zaproxy.org/",
                        "rules": list(rules.values()),
                    }
                },
                "results": results,
            }
        ],
    }
    with out_path.open("w", encoding="utf-8") as f:
        json.dump(sarif, f, indent=2)


def main(argv=None):
    """Validate both CLI paths (inside the workspace, expected suffixes), then convert."""
    args = list(sys.argv[1:] if argv is None else argv[1:])
    if len(args) != 2:
        sys.exit("usage: zap2sarif.py <zap-json> <out.sarif>")
    in_path = _resolve_ci_path(args[0], {".json"})
    out_path = _resolve_ci_path(args[1], {".sarif"})
    with in_path.open(encoding="utf-8") as f:
        convert(json.load(f), out_path)
    print(f"wrote {out_path}")


if __name__ == "__main__":
    main()
