#!/usr/bin/env bash
set -euo pipefail

base_ref="${BASE_REF:-main}"
classification="${OPENAPI_CHANGE_CLASSIFICATION:-}"
allow_breaking="${ALLOW_BREAKING_API_CHANGES:-false}"
pr_body="${PR_BODY:-}"
labels_json="${PR_LABELS_JSON:-[]}"

if ! git rev-parse --verify "origin/${base_ref}" >/dev/null 2>&1; then
  git fetch --no-tags --depth=1 origin "${base_ref}"
fi

openapi_changed="false"
if git diff --name-only "origin/${base_ref}...HEAD" | awk '$0=="docs/openapi.yaml"{found=1} END{exit found?0:1}'; then
  openapi_changed="true"
fi

if [ -n "${pr_body}" ]; then
  parsed="$(printf '%s' "${pr_body}" | tr '[:upper:]' '[:lower:]' | sed -nE 's/.*classification:[[:space:]]*`?(additive|behavior-changing|breaking|none)`?.*/\1/p' | head -n1)"
  if [ -n "${parsed}" ]; then
    classification="${parsed}"
  fi
fi

if printf '%s' "${labels_json}" | tr '[:upper:]' '[:lower:]' | awk '/api-breaking-approved/{found=1} END{exit found?0:1}'; then
  allow_breaking="true"
fi

case "${classification}" in
  ""|additive|behavior-changing|breaking|none) ;;
  *)
    echo "Invalid OpenAPI classification: '${classification}'. Use additive, behavior-changing, breaking, or none."
    exit 1
    ;;
esac

if [ "${openapi_changed}" = "false" ]; then
  echo "OpenAPI unchanged; compatibility gate passed."
  exit 0
fi

if [ -z "${classification}" ] || [ "${classification}" = "none" ]; then
  cat <<'EOF'
OpenAPI changed, but compatibility classification is missing.
Add this to the PR body:
- Classification: `additive|behavior-changing|breaking`
EOF
  exit 1
fi

if [ "${classification}" = "breaking" ] && [ "${allow_breaking}" != "true" ]; then
  cat <<'EOF'
Breaking OpenAPI change detected and blocked by policy.
To permit, add the PR label `api-breaking-approved` (or set ALLOW_BREAKING_API_CHANGES=true for trusted override contexts).
EOF
  exit 1
fi

echo "OpenAPI compatibility gate passed with classification='${classification}'."
