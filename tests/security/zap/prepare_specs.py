"""Copy the four service specs for ZAP with the gateway as the ONLY server,
so the scan always goes through auth and never straight at 8080-8083."""
import sys
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parents[3]
SPECS = {
    "ledger-service": "ledger",
    "accounts-service": "accounts",
    "settlement-engine": "settlement",
    "notification-service": "notifications",
}


def main(gateway_url: str, out_dir: str) -> None:
    out = Path(out_dir)
    out.mkdir(parents=True, exist_ok=True)
    for name, prefix in SPECS.items():
        spec = yaml.safe_load((REPO / "docs" / "openapi" / f"{name}.yaml").read_text(encoding="utf-8"))
        spec["servers"] = [{"url": f"{gateway_url.rstrip('/')}/{prefix}"}]
        (out / f"{name}.yaml").write_text(yaml.safe_dump(spec, sort_keys=False, allow_unicode=True), encoding="utf-8")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
