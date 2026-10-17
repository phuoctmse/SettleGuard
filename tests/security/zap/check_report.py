"""Decide pass/fail for ZAP API scans by risk, not by rule id.

zap-api-scan's own exit code splits WARN/FAIL per rule; we want "any High
not explicitly accepted fails the build", so read the JSON reports instead.
Every accepted risk must name the finding in docs/security/PENTEST_REPORT.md
that explains it.
"""
import json
import sys
from pathlib import Path

import yaml

HIGH, MEDIUM = "3", "2"
REQUIRED = ("plugin_id", "finding_id", "reason")


def load_accepted(path) -> list[dict]:
    entries = (yaml.safe_load(Path(path).read_text(encoding="utf-8")) or {}).get("accepted") or []
    for entry in entries:
        for field in REQUIRED:
            if not entry.get(field):
                raise ValueError(f"accepted risk {entry!r} is missing {field}")
    return entries


def _alerts(reports):
    for report in reports:
        for site in report.get("site", []):
            yield from site.get("alerts", [])


def evaluate(reports, accepted):
    accepted_ids = {str(e["plugin_id"]) for e in accepted}
    high = [a for a in _alerts(reports) if a["riskcode"] == HIGH and a["pluginid"] not in accepted_ids]
    medium = [a for a in _alerts(reports) if a["riskcode"] == MEDIUM]
    return high, medium


def _table(title, alerts):
    if not alerts:
        return f"**{title}:** none\n"
    rows = "\n".join(f"| {a['pluginid']} | {a['name']} | {a['instances'][0]['uri'] if a.get('instances') else ''} |" for a in alerts)
    return f"**{title}:**\n\n| Plugin | Alert | Example URI |\n|---|---|---|\n{rows}\n"


def main(argv):
    accepted = load_accepted(argv[1])
    reports = [json.loads(Path(p).read_text(encoding="utf-8")) for p in argv[2:]]
    if not any(report.get("site") for report in reports):
        print("**No ZAP results found — failing closed.**")
        return 1
    high, medium = evaluate(reports, accepted)
    print("## ZAP API scan\n")
    print(_table("High (unaccepted) — fails the build", high))
    print(_table("Medium — reported only", medium))
    return 1 if high else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
