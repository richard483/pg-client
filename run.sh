#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

DB=""
SCHEMA=""
QUERY=""

usage() {
  cat <<'USAGE'
Usage:
  ./run.sh --db "karasu" --schema "public" --query "SELECT 1;"

Required environment variables:
  KARASU_TOKEN_URL      Example: http://127.0.0.1:8080/oauth/token
  PG_CLIENT_URL         Example: http://127.0.0.1:8090/execute
  CLIENT_ID             Karasu OAuth client_id for this caller
  CLIENT_PRIVATE_KEY    Path to RSA private key PEM used for client assertion signing

Optional environment variables:
  TOKEN_AUDIENCE        Default: pg-client-api
  TOKEN_SCOPE           Default: pg-client:execute
  KARASU_ASSERTION_AUD  Default: KARASU_TOKEN_URL
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --db)
      DB="${2:-}"
      shift 2
      ;;
    --schema)
      SCHEMA="${2:-}"
      shift 2
      ;;
    --query)
      QUERY="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

required() {
  local name="$1"
  local value="${!name:-}"
  if [[ -z "$value" ]]; then
    echo "Missing required value: $name" >&2
    exit 2
  fi
}

required "DB"
required "SCHEMA"
required "QUERY"
required "KARASU_TOKEN_URL"
required "PG_CLIENT_URL"
required "CLIENT_ID"
required "CLIENT_PRIVATE_KEY"

TOKEN_AUDIENCE="${TOKEN_AUDIENCE:-pg-client-api}"
TOKEN_SCOPE="${TOKEN_SCOPE:-pg-client:execute}"
KARASU_ASSERTION_AUD="${KARASU_ASSERTION_AUD:-$KARASU_TOKEN_URL}"

case "$CLIENT_PRIVATE_KEY" in
  /*|[A-Za-z]:/*|[A-Za-z]:\\*)
    ;;
  *)
    CLIENT_PRIVATE_KEY="$ROOT_DIR/$CLIENT_PRIVATE_KEY"
    ;;
esac

if [[ ! -f "$CLIENT_PRIVATE_KEY" ]]; then
  echo "CLIENT_PRIVATE_KEY does not point to a file: $CLIENT_PRIVATE_KEY" >&2
  exit 2
fi

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 2
  fi
}

need_cmd curl
need_cmd openssl

if command -v python3 >/dev/null 2>&1; then
  PYTHON_BIN="python3"
elif command -v python >/dev/null 2>&1; then
  PYTHON_BIN="python"
else
  echo "Missing required command: python3 or python" >&2
  exit 2
fi

base64url() {
  openssl base64 -A | tr '+/' '-_' | tr -d '='
}

now="$(date +%s)"
exp="$((now + 300))"
jti="$(openssl rand -hex 16)"
token_audience="${KARASU_ASSERTION_AUD}"

header='{"alg":"RS256","typ":"JWT"}'
payload="$("$PYTHON_BIN" - "$CLIENT_ID" "$token_audience" "$now" "$exp" "$jti" <<'PY'
import json
import sys

client_id, audience, now, exp, jti = sys.argv[1:]
print(json.dumps({
    "iss": client_id,
    "sub": client_id,
    "aud": audience,
    "iat": int(now),
    "exp": int(exp),
    "jti": jti,
}, separators=(",", ":")))
PY
)"

encoded_header="$(printf '%s' "$header" | base64url)"
encoded_payload="$(printf '%s' "$payload" | base64url)"
signing_input="${encoded_header}.${encoded_payload}"
signature="$(printf '%s' "$signing_input" | openssl dgst -sha256 -sign "$CLIENT_PRIVATE_KEY" -binary | base64url)"
client_assertion="${signing_input}.${signature}"

token_response="$(curl -sS -X POST "$KARASU_TOKEN_URL" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=client_credentials" \
  --data-urlencode "client_id=$CLIENT_ID" \
  --data-urlencode "audience=$TOKEN_AUDIENCE" \
  --data-urlencode "scope=$TOKEN_SCOPE" \
  --data-urlencode "client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer" \
  --data-urlencode "client_assertion=$client_assertion")"

access_token="$(TOKEN_RESPONSE="$token_response" "$PYTHON_BIN" - <<'PY'
import json
import os
import sys

try:
    data = json.loads(os.environ["TOKEN_RESPONSE"])
except Exception as exc:
    raise SystemExit(f"Failed to parse token response as JSON: {exc}")

token = data.get("access_token")
if not token:
    raise SystemExit("Token response did not contain access_token: " + json.dumps(data))

print(token)
PY
)"

request_body="$("$PYTHON_BIN" - "$DB" "$SCHEMA" "$QUERY" <<'PY'
import json
import sys

db, schema, query = sys.argv[1:]
print(json.dumps({
    "db": db,
    "schema": schema,
    "query": query,
}))
PY
)"

curl -sS -X POST "$PG_CLIENT_URL" \
  -H "Authorization: Bearer $access_token" \
  -H "Content-Type: application/json" \
  --data "$request_body"
printf '\n'
