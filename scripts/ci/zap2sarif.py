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

RISK_LEVEL = {"0": "none", "1": "note", "2": "warning", "3": "error"}


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
    with open(out_path, "w") as f:
        json.dump(sarif, f, indent=2)


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit("usage: zap2sarif.py <zap-json> <out.sarif>")
    with open(sys.argv[1]) as f:
        convert(json.load(f), sys.argv[2])
    print(f"wrote {sys.argv[2]}")
