#!/usr/bin/env bash
# Authenticated ZAP API scan of all four services, through the gateway.
# Usage (repo root): bash tests/security/zap/run_zap.sh
# Windows/Docker Desktop: ZAP_GATEWAY_URL=http://host.docker.internal:8000 ZAP_DOCKER_NETWORK=bridge bash tests/security/zap/run_zap.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
ZAP_DIR="$ROOT/tests/security/zap"
OUT="$ZAP_DIR/out"
ZAP_GATEWAY_URL="${ZAP_GATEWAY_URL:-http://localhost:8000}"
ZAP_DOCKER_NETWORK="${ZAP_DOCKER_NETWORK:-host}"
ZAP_MAX_SCAN_MINUTES="${ZAP_MAX_SCAN_MINUTES:-8}"
ACCOUNTS_API_URL="${ACCOUNTS_API_URL:-http://localhost:8081}"
GATEWAY_DATABASE_URL="${GATEWAY_DATABASE_URL:-postgres://gateway:gateway@localhost:5437/gateway?sslmode=disable}"
export PYTHONIOENCODING=utf-8   # reports contain non-ASCII; Windows consoles default to cp1252

rm -rf "$OUT" && mkdir -p "$OUT"
chmod 777 "$OUT"   # the ZAP image runs as uid 1000 and writes reports here

# Git Bash on Windows: docker needs a Windows path for the bind mount (pwd -W);
# on Linux pwd -W fails and we fall back to the plain path.
OUT_MOUNT=$( (cd "$OUT" && pwd -W) 2>/dev/null || echo "$OUT")

client_id=$(curl -fsS -X POST "$ACCOUNTS_API_URL/clients" -H 'Content-Type: application/json' \
  -d '{"name":"zap-scan"}' | python -c 'import json,sys; print(json.load(sys.stdin)["id"])')
read -r -a adminctl <<< "${GATEWAY_ADMINCTL:-go run ./cmd/adminctl}"
raw_key=$(cd "$ROOT/services/gateway" && DATABASE_URL="$GATEWAY_DATABASE_URL" "${adminctl[@]}" create \
  --client-id "$client_id" --label zap-scan | sed -n 's/^key:[[:space:]]*//p')
[ -n "$raw_key" ] || { echo "could not issue a ZAP key"; exit 1; }

python "$ZAP_DIR/prepare_specs.py" "$ZAP_GATEWAY_URL" "$OUT"

# notification-service stays LAST: SG-PT-07 can leave it unable to serve /notifications.
for spec in ledger-service accounts-service settlement-engine notification-service; do
  echo "::group::ZAP $spec"
  # -I: rule-level WARNs never fail the run; check_report.py decides by risk.
  # Exit code 3 is a scan error and must fail.
  # MSYS_NO_PATHCONV stops Git Bash rewriting /zap/wrk/... container paths (no-op on Linux).
  set +e
  MSYS_NO_PATHCONV=1 docker run --rm --network "$ZAP_DOCKER_NETWORK" -v "$OUT_MOUNT:/zap/wrk:rw" \
    -e ZAP_AUTH_HEADER=Authorization -e ZAP_AUTH_HEADER_VALUE="Bearer $raw_key" \
    ghcr.io/zaproxy/zaproxy:stable zap-api-scan.py \
      -t "/zap/wrk/$spec.yaml" -f openapi -I \
      -r "$spec.html" -J "$spec.json" \
      -z "-config scanner.maxScanDurationInMins=$ZAP_MAX_SCAN_MINUTES -config scanner.maxRuleDurationInMins=2" 2>&1 | tee "$OUT/$spec.log"
  code=${PIPESTATUS[0]}   # ZAP's own exit code, not tee's
  set -e
  echo "::endgroup::"
  case $code in 0|1|2) ;; *) echo "ZAP failed on $spec (exit $code)"; exit 3;; esac
  [ -s "$OUT/$spec.json" ] || { echo "ZAP produced no report for $spec"; exit 3; }
done

python "$ZAP_DIR/check_report.py" "$ZAP_DIR/accepted_risks.yaml" "$OUT"/*.json
