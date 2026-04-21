#!/usr/bin/env bash
# End-to-end API demo with assertions and optional cleanup.
# Requires: curl, jq
# Usage:
#   BASE_URL=http://localhost:8080 ./scripts/demo-api.sh [--cleanup] [--report-file path] [--suffix value]
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
PASSWORD='DemoPass1!'
WORK_DATE="$(date -u +%Y-%m-%d)"
SUFFIX="$(date +%s)"
REPORT_FILE="${REPORT_FILE:-./tmp/demo-api-report.json}"
CLEANUP_MODE="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --cleanup)
      CLEANUP_MODE="true"
      shift
      ;;
    --report-file)
      REPORT_FILE="${2:-}"
      shift 2
      ;;
    --suffix)
      SUFFIX="${2:-}"
      shift 2
      ;;
    --help|-h)
      cat <<'EOF'
Usage: ./scripts/demo-api.sh [--cleanup] [--report-file path] [--suffix value]
  --cleanup            Deactivate the synthetic org at the end of the run.
  --report-file PATH   Write machine-readable run summary JSON (default: ./tmp/demo-api-report.json).
  --suffix VALUE       Override generated unique suffix for deterministic reruns.
EOF
      exit 0
      ;;
    *)
      echo "error: unknown flag '$1'" >&2
      exit 1
      ;;
  esac
done

OWNER_EMAIL="demo-owner-${SUFFIX}@demo.local"
CONTRACTOR_EMAIL="demo-contractor-${SUFFIX}@demo.local"

STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
CHECK_LINES=""
ORG_ID=""
CLIENT_ID=""
PROJECT_ID=""
ENTRY_ID=""
INVOICE_ID=""
INVOICE_TOTAL_MINOR="0"
OUTSTANDING_MINOR="0"
TOKEN_OWNER=""
TOKEN_CONTRACTOR=""
CLEANUP_RESULT="not-requested"

die() { echo "error: $*" >&2; exit 1; }

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "missing dependency: $1"; }
need_cmd curl
need_cmd jq

record_check() {
  local name="$1"
  local result="$2"
  local detail="$3"
  CHECK_LINES+=$(jq -nc --arg n "$name" --arg r "$result" --arg d "$detail" '{name:$n,result:$r,detail:$d}')
  CHECK_LINES+=$'\n'
}

assert_jq() {
  local json="$1"
  local expr="$2"
  local name="$3"
  local detail="$4"
  if ! echo "$json" | jq -e "$expr" >/dev/null; then
    record_check "$name" "fail" "$detail"
    die "$name assertion failed: $detail"
  fi
  record_check "$name" "pass" "$detail"
}

assert_time_entry_in_status_list() {
  local entry_id="$1"
  local token="$2"
  local org="$3"
  local expected_status="$4"
  local check_name="$5"
  local detail="$6"
  local list_json
  list_json="$(curl -sS -G "${BASE_URL}/v1/time-entries" \
    -H "Authorization: Bearer ${token}" \
    -H "X-Organization-ID: ${org}" \
    --data-urlencode "status=${expected_status}")"
  if ! echo "$list_json" | jq -e --arg eid "$entry_id" '(.entries // .items // []) | map(.id) | index($eid) != null' >/dev/null; then
    record_check "$check_name" "fail" "${detail} (status filter=${expected_status})"
    die "${check_name} assertion failed: entry ${entry_id} not found in status=${expected_status} list"
  fi
  record_check "$check_name" "pass" "${detail} (status=${expected_status})"
}

finalize_report() {
  local exit_code=$?
  local ended_at status report_dir checks_json
  set +e
  ended_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  status="success"
  if [[ $exit_code -ne 0 ]]; then
    status="failure"
  fi
  if [[ "$CLEANUP_MODE" == "true" ]] && [[ "$CLEANUP_RESULT" == "not-requested" ]] && [[ -n "$ORG_ID" ]] && [[ -n "$TOKEN_OWNER" ]]; then
    local cleanup_code
    cleanup_code="$(curl -sS -o /dev/null -w "%{http_code}" -X POST "${BASE_URL}/v1/organizations/${ORG_ID}/deactivate" \
      -H 'Content-Type: application/json' \
      -H "Authorization: Bearer ${TOKEN_OWNER}" \
      -H "X-Organization-ID: ${ORG_ID}" \
      -d '{}')"
    if [[ "$cleanup_code" == "204" ]]; then
      CLEANUP_RESULT="deactivated-on-exit"
      record_check "cleanup_deactivate_org" "pass" "organization deactivated in finalize hook"
    else
      CLEANUP_RESULT="cleanup-failed-http-${cleanup_code}"
      record_check "cleanup_deactivate_org" "fail" "finalize cleanup failed with HTTP ${cleanup_code}"
    fi
  fi
  checks_json="$(printf '%s' "$CHECK_LINES" | jq -s '.')"
  report_dir="$(dirname "$REPORT_FILE")"
  mkdir -p "$report_dir"
  jq -n \
    --arg status "$status" \
    --arg started_at "$STARTED_AT" \
    --arg ended_at "$ended_at" \
    --arg base_url "$BASE_URL" \
    --arg owner_email "$OWNER_EMAIL" \
    --arg contractor_email "$CONTRACTOR_EMAIL" \
    --arg organization_id "$ORG_ID" \
    --arg client_id "$CLIENT_ID" \
    --arg project_id "$PROJECT_ID" \
    --arg time_entry_id "$ENTRY_ID" \
    --arg invoice_id "$INVOICE_ID" \
    --arg invoice_total_minor "$INVOICE_TOTAL_MINOR" \
    --arg outstanding_minor "$OUTSTANDING_MINOR" \
    --arg cleanup_mode "$CLEANUP_MODE" \
    --arg cleanup_result "$CLEANUP_RESULT" \
    --argjson checks "$checks_json" \
    '{
      status: $status,
      started_at: $started_at,
      ended_at: $ended_at,
      base_url: $base_url,
      cleanup_mode: ($cleanup_mode == "true"),
      cleanup_result: $cleanup_result,
      entities: {
        owner_email: $owner_email,
        contractor_email: $contractor_email,
        organization_id: $organization_id,
        client_id: $client_id,
        project_id: $project_id,
        time_entry_id: $time_entry_id,
        invoice_id: $invoice_id
      },
      summary: {
        invoice_total_minor: ($invoice_total_minor | tonumber),
        outstanding_minor: ($outstanding_minor | tonumber)
      },
      checks: $checks
    }' > "$REPORT_FILE"

  if [[ "$status" == "success" ]]; then
    echo "==> Demo complete (PASS)."
    echo "    owner_email=${OWNER_EMAIL}"
    echo "    contractor_email=${CONTRACTOR_EMAIL}"
    echo "    organization_id=${ORG_ID}"
    echo "    report_file=${REPORT_FILE}"
  else
    echo "==> Demo failed (see report: ${REPORT_FILE})" >&2
  fi
  exit $exit_code
}
trap finalize_report EXIT

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

echo "==> ${BASE_URL}/healthz"
HEALTH_JSON="$(curl -sf "${BASE_URL}/healthz")"
assert_jq "$HEALTH_JSON" '.status == "ok"' "healthz" "health endpoint returns ok"

echo "==> Register owner (${OWNER_EMAIL})"
REGISTER_OWNER="$(json_post "${BASE_URL}/auth/register" "$(jq -nc \
  --arg e "$OWNER_EMAIL" --arg p "$PASSWORD" --arg n 'Demo Owner' \
  '{email:$e,password:$p,full_name:$n}')" "201")"
ORG_ID="$(echo "$REGISTER_OWNER" | jq -r .organization_id)"
assert_jq "$REGISTER_OWNER" '.organization_id | type == "string"' "register_owner" "organization_id returned"

echo "==> Register contractor (${CONTRACTOR_EMAIL})"
REGISTER_CONTRACTOR="$(json_post "${BASE_URL}/auth/register" "$(jq -nc \
  --arg e "$CONTRACTOR_EMAIL" --arg p "$PASSWORD" --arg n 'Demo Contractor' \
  '{email:$e,password:$p,full_name:$n}')" "201")"
assert_jq "$REGISTER_CONTRACTOR" '.user_id | type == "string"' "register_contractor" "contractor user_id returned"

echo "==> Owner login"
LOGIN_OWNER="$(json_post "${BASE_URL}/auth/login" "$(jq -nc \
  --arg e "$OWNER_EMAIL" --arg p "$PASSWORD" '{email:$e,password:$p}')" "200")"
TOKEN_OWNER="$(echo "$LOGIN_OWNER" | jq -r .access_token)"
assert_jq "$LOGIN_OWNER" '.access_token != "" and .refresh_token != ""' "owner_login" "owner tokens issued"

echo "==> Create client"
CREATE_CLIENT_JSON="$(json_post_auth "${BASE_URL}/v1/clients" "$(jq -nc \
  --arg n 'Demo Client Co' --arg b 'billing@demo-client.local' --arg c 'USD' \
  '{name:$n,billing_email:$b,currency_preference:$c}')" \
  "$TOKEN_OWNER" "$ORG_ID" "201")"
CLIENT_ID="$(echo "$CREATE_CLIENT_JSON" | jq -r .id)"
assert_jq "$CREATE_CLIENT_JSON" '.id | type == "string"' "create_client" "client id returned"

echo "==> Create project"
CREATE_PROJECT_JSON="$(json_post_auth "${BASE_URL}/v1/projects" "$(jq -nc \
  --arg cid "$CLIENT_ID" \
  '{client_id:$cid,name:"Demo Project",billing_mode:"hourly",default_rate_minor:15000}')" \
  "$TOKEN_OWNER" "$ORG_ID" "201")"
PROJECT_ID="$(echo "$CREATE_PROJECT_JSON" | jq -r .id)"
assert_jq "$CREATE_PROJECT_JSON" '.id | type == "string"' "create_project" "project id returned"

echo "==> Add contractor membership"
ADD_MEMBERSHIP_JSON="$(json_post_auth "${BASE_URL}/v1/memberships" "$(jq -nc \
  --arg e "$CONTRACTOR_EMAIL" '{email:$e,role:"contractor"}')" \
  "$TOKEN_OWNER" "$ORG_ID" "201")"
assert_jq "$ADD_MEMBERSHIP_JSON" '.id | type == "string"' "add_membership" "membership id returned"

echo "==> Contractor login"
LOGIN_CONTRACTOR="$(json_post "${BASE_URL}/auth/login" "$(jq -nc \
  --arg e "$CONTRACTOR_EMAIL" --arg p "$PASSWORD" '{email:$e,password:$p}')" "200")"
TOKEN_CONTRACTOR="$(echo "$LOGIN_CONTRACTOR" | jq -r .access_token)"
assert_jq "$LOGIN_CONTRACTOR" '.access_token != ""' "contractor_login" "contractor token issued"

echo "==> Contractor creates time entry (${WORK_DATE})"
CREATE_ENTRY_JSON="$(json_post_auth "${BASE_URL}/v1/time-entries" "$(jq -nc \
  --arg pid "$PROJECT_ID" --arg wd "$WORK_DATE" \
  '{project_id:$pid,work_date:$wd,minutes:120,hourly_rate_minor:15000,notes:"demo-api.sh"}')" \
  "$TOKEN_CONTRACTOR" "$ORG_ID" "201")"
ENTRY_ID="$(echo "$CREATE_ENTRY_JSON" | jq -r .id)"
assert_jq "$CREATE_ENTRY_JSON" '.id | type == "string"' "create_time_entry" "time entry id returned"
assert_time_entry_in_status_list "$ENTRY_ID" "$TOKEN_CONTRACTOR" "$ORG_ID" "draft" "create_time_entry_status" "new time entry starts in draft"

echo "==> Contractor submits time entry"
json_post_auth "${BASE_URL}/v1/time-entries/${ENTRY_ID}/submit" "{}" \
  "$TOKEN_CONTRACTOR" "$ORG_ID" "204" >/dev/null
record_check "submit_time_entry" "pass" "submit transition accepted"
assert_time_entry_in_status_list "$ENTRY_ID" "$TOKEN_OWNER" "$ORG_ID" "submitted" "submit_time_entry_status" "time entry transitions to submitted"

echo "==> Owner approves time entry"
json_post_auth "${BASE_URL}/v1/time-entries/${ENTRY_ID}/approve" "{}" \
  "$TOKEN_OWNER" "$ORG_ID" "204" >/dev/null
record_check "approve_time_entry" "pass" "approve transition accepted"
assert_time_entry_in_status_list "$ENTRY_ID" "$TOKEN_OWNER" "$ORG_ID" "approved" "approve_time_entry_status" "time entry transitions to approved"

echo "==> Owner generates invoice"
INV_JSON="$(json_post_auth "${BASE_URL}/v1/invoices/generate" "$(jq -nc \
  --arg from "${WORK_DATE}" --arg to "${WORK_DATE}" \
  '{from_date:$from,to_date:$to,currency:"USD"}')" \
  "$TOKEN_OWNER" "$ORG_ID" "201")"
INVOICE_ID="$(echo "$INV_JSON" | jq -r .id)"
INVOICE_TOTAL_MINOR="$(echo "$INV_JSON" | jq -r .total_minor)"
assert_jq "$INV_JSON" '(.status == "draft") or (.status == null)' "generate_invoice_status" "generated invoice payload is compatible with draft-status contract"
assert_jq "$INV_JSON" '.total_minor > 0' "generate_invoice_total" "generated invoice has positive total"

echo "==> Owner sends invoice"
SEND_INV_JSON="$(json_post_auth "${BASE_URL}/v1/invoices/${INVOICE_ID}/send" "{}" \
  "$TOKEN_OWNER" "$ORG_ID" "200")"
assert_jq "$SEND_INV_JSON" '.status == "issued"' "send_invoice_status" "invoice transitions to issued"

echo "==> Owner: outstanding report"
OUT_JSON="$(json_get_auth "${BASE_URL}/v1/reports/outstanding" "$TOKEN_OWNER" "$ORG_ID" "200")"
OUTSTANDING_MINOR="$(echo "$OUT_JSON" | jq -r '.by_currency.USD.amount_minor // 0')"
assert_jq "$OUT_JSON" '.by_currency.USD.amount_minor >= 0' "outstanding_report_presence" "USD outstanding totals are present"
if (( OUTSTANDING_MINOR < INVOICE_TOTAL_MINOR )); then
  record_check "outstanding_report_amount" "fail" "outstanding report should include generated invoice amount"
  die "outstanding report amount ${OUTSTANDING_MINOR} is less than invoice total ${INVOICE_TOTAL_MINOR}"
fi
record_check "outstanding_report_amount" "pass" "outstanding report includes generated invoice amount"

echo "==> Owner: list time entries (invoiced after billing)"
INVOICED_LIST="$(curl -sS -G "${BASE_URL}/v1/time-entries" \
  -H "Authorization: Bearer ${TOKEN_OWNER}" \
  -H "X-Organization-ID: ${ORG_ID}" \
  --data-urlencode "status=invoiced")"
if ! echo "$INVOICED_LIST" | jq -e --arg eid "$ENTRY_ID" '(.entries // .items // []) | map(.id) | index($eid) != null' >/dev/null; then
  record_check "invoiced_time_entry_list" "fail" "approved entry should be listed as invoiced"
  die "invoiced_time_entry_list assertion failed: entry ${ENTRY_ID} not found in status=invoiced list"
fi
record_check "invoiced_time_entry_list" "pass" "approved entry is listed as invoiced"

if [[ "$CLEANUP_MODE" == "true" ]] && [[ -n "$ORG_ID" ]] && [[ -n "$TOKEN_OWNER" ]]; then
  echo "==> Cleanup mode: deactivate demo organization"
  json_post_auth "${BASE_URL}/v1/organizations/${ORG_ID}/deactivate" "{}" "$TOKEN_OWNER" "$ORG_ID" "204" >/dev/null
  CLEANUP_RESULT="deactivated"
  record_check "cleanup_deactivate_org" "pass" "organization deactivated to isolate demo artifacts"
fi
