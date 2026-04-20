#!/usr/bin/env bash
# End-to-end API demo: register owner + contractor, tenant client/project,
# contractor time entry → submit → owner approve → generate + send invoice.
#
# Requires: curl, jq
# Usage: BASE_URL=http://localhost:8080 ./scripts/demo-api.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
SUFFIX="$(date +%s)"
OWNER_EMAIL="demo-owner-${SUFFIX}@demo.local"
CONTRACTOR_EMAIL="demo-contractor-${SUFFIX}@demo.local"
PASSWORD='DemoPass1!'
WORK_DATE="$(date -u +%Y-%m-%d)"

die() { echo "error: $*" >&2; exit 1; }

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "missing dependency: $1"; }
need_cmd curl
need_cmd jq

json_post() {
  local url="$1"
  local body="$2"
  local expected="$3"
  local tmp code
  tmp="$(mktemp)"
  code="$(curl -sS -o "$tmp" -w "%{http_code}" -X POST "$url" \
    -H 'Content-Type: application/json' \
    -d "$body")"
  if [[ "$code" != "$expected" ]]; then
    echo "HTTP $code (expected $expected) for POST $url" >&2
    cat "$tmp" >&2
    rm -f "$tmp"
    exit 1
  fi
  cat "$tmp"
  rm -f "$tmp"
}

json_post_auth() {
  local url="$1"
  local body="$2"
  local token="$3"
  local org="$4"
  local expected="$5"
  local tmp code
  tmp="$(mktemp)"
  code="$(curl -sS -o "$tmp" -w "%{http_code}" -X POST "$url" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${token}" \
    -H "X-Organization-ID: ${org}" \
    -d "$body")"
  if [[ "$code" != "$expected" ]]; then
    echo "HTTP $code (expected $expected) for POST $url" >&2
    cat "$tmp" >&2
    rm -f "$tmp"
    exit 1
  fi
  cat "$tmp"
  rm -f "$tmp"
}

json_get_auth() {
  local url="$1"
  local token="$2"
  local org="$3"
  local expected="$4"
  local tmp code
  tmp="$(mktemp)"
  code="$(curl -sS -o "$tmp" -w "%{http_code}" -X GET "$url" \
    -H "Authorization: Bearer ${token}" \
    -H "X-Organization-ID: ${org}")"
  if [[ "$code" != "$expected" ]]; then
    echo "HTTP $code (expected $expected) for GET $url" >&2
    cat "$tmp" >&2
    rm -f "$tmp"
    exit 1
  fi
  cat "$tmp"
  rm -f "$tmp"
}

login_token() {
  local email="$1"
  local body
  body="$(jq -nc --arg e "$email" --arg p "$PASSWORD" '{email:$e,password:$p}')"
  json_post "${BASE_URL}/auth/login" "$body" "200" | jq -r .access_token
}

echo "==> ${BASE_URL}/healthz"
curl -sf "${BASE_URL}/healthz" | jq .

echo "==> Register owner (${OWNER_EMAIL})"
REGISTER_OWNER="$(json_post "${BASE_URL}/auth/register" "$(jq -nc \
  --arg e "$OWNER_EMAIL" --arg p "$PASSWORD" --arg n 'Demo Owner' \
  '{email:$e,password:$p,full_name:$n}')" "201")"
ORG_ID="$(echo "$REGISTER_OWNER" | jq -r .organization_id)"
echo "    organization_id=${ORG_ID}"

echo "==> Register contractor (${CONTRACTOR_EMAIL})"
json_post "${BASE_URL}/auth/register" "$(jq -nc \
  --arg e "$CONTRACTOR_EMAIL" --arg p "$PASSWORD" --arg n 'Demo Contractor' \
  '{email:$e,password:$p,full_name:$n}')" "201" >/dev/null

echo "==> Owner login"
TOKEN_OWNER="$(login_token "$OWNER_EMAIL")"

echo "==> Create client"
CLIENT_ID="$(json_post_auth "${BASE_URL}/v1/clients" "$(jq -nc \
  --arg n 'Demo Client Co' --arg b 'billing@demo-client.local' --arg c 'USD' \
  '{name:$n,billing_email:$b,currency_preference:$c}')" \
  "$TOKEN_OWNER" "$ORG_ID" "201" | jq -r .id)"
echo "    client_id=${CLIENT_ID}"

echo "==> Create project"
PROJECT_ID="$(json_post_auth "${BASE_URL}/v1/projects" "$(jq -nc \
  --arg cid "$CLIENT_ID" \
  '{client_id:$cid,name:"Demo Project",billing_mode:"hourly",default_rate_minor:15000}')" \
  "$TOKEN_OWNER" "$ORG_ID" "201" | jq -r .id)"
echo "    project_id=${PROJECT_ID}"

echo "==> Add contractor membership"
json_post_auth "${BASE_URL}/v1/memberships" "$(jq -nc \
  --arg e "$CONTRACTOR_EMAIL" '{email:$e,role:"contractor"}')" \
  "$TOKEN_OWNER" "$ORG_ID" "201" >/dev/null

echo "==> Contractor login"
TOKEN_CONTRACTOR="$(login_token "$CONTRACTOR_EMAIL")"

echo "==> Contractor creates time entry (${WORK_DATE})"
ENTRY_ID="$(json_post_auth "${BASE_URL}/v1/time-entries" "$(jq -nc \
  --arg pid "$PROJECT_ID" --arg wd "$WORK_DATE" \
  '{project_id:$pid,work_date:$wd,minutes:120,hourly_rate_minor:15000,notes:"demo-api.sh"}')" \
  "$TOKEN_CONTRACTOR" "$ORG_ID" "201" | jq -r .id)"
echo "    time_entry_id=${ENTRY_ID}"

echo "==> Contractor submits time entry"
json_post_auth "${BASE_URL}/v1/time-entries/${ENTRY_ID}/submit" "{}" \
  "$TOKEN_CONTRACTOR" "$ORG_ID" "204"

echo "==> Owner approves time entry"
json_post_auth "${BASE_URL}/v1/time-entries/${ENTRY_ID}/approve" "{}" \
  "$TOKEN_OWNER" "$ORG_ID" "204"

echo "==> Owner generates invoice"
INV_JSON="$(json_post_auth "${BASE_URL}/v1/invoices/generate" "$(jq -nc \
  --arg from "${WORK_DATE}" --arg to "${WORK_DATE}" \
  '{from_date:$from,to_date:$to,currency:"USD"}')" \
  "$TOKEN_OWNER" "$ORG_ID" "201")"
echo "$INV_JSON" | jq .
INVOICE_ID="$(echo "$INV_JSON" | jq -r .id)"

echo "==> Owner sends invoice"
json_post_auth "${BASE_URL}/v1/invoices/${INVOICE_ID}/send" "{}" \
  "$TOKEN_OWNER" "$ORG_ID" "200" | jq .

echo "==> Owner: outstanding report"
json_get_auth "${BASE_URL}/v1/reports/outstanding" "$TOKEN_OWNER" "$ORG_ID" "200" | jq .

echo "==> Owner: list time entries (invoiced after billing)"
curl -sS -G "${BASE_URL}/v1/time-entries" \
  -H "Authorization: Bearer ${TOKEN_OWNER}" \
  -H "X-Organization-ID: ${ORG_ID}" \
  --data-urlencode "status=invoiced" | jq .

echo "==> Demo complete."
echo "    owner_email=${OWNER_EMAIL}"
echo "    contractor_email=${CONTRACTOR_EMAIL}"
echo "    password=${PASSWORD}"
echo "    organization_id=${ORG_ID}"
