#!/usr/bin/env bash
set -euo pipefail

# Agent bootstrap script for this repository.
# Safe to run multiple times.
#
# What it does:
# - Validates required files exist
# - Validates backlog JSON against schema
# - Verifies tooling availability (python3, gh, git; go optional)
# - Runs optional Go checks if go.mod exists
# - Optionally initializes local git repo and local main branch
# - Prints next-step commands for autonomous PR flow

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

log() {
  printf "[agent-bootstrap] %s\n" "$*"
}

warn() {
  printf "[agent-bootstrap][WARN] %s\n" "$*" >&2
}

err() {
  printf "[agent-bootstrap][ERROR] %s\n" "$*" >&2
  exit 1
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    err "Missing required command: $cmd"
  fi
}

check_optional_cmd() {
  local cmd="$1"
  if command -v "$cmd" >/dev/null 2>&1; then
    log "Optional command available: $cmd"
  else
    warn "Optional command not found: $cmd"
  fi
}

validate_required_files() {
  local required_files=(
    "backlog.v1.json"
    "backlog.v1.schema.json"
    "backlog.v1.strict.json"
    "agent.autonomy.rules.json"
    "AGENTS.md"
    "story-status.json"
    ".github/workflows/ci.yml"
    ".github/workflows/cd-main.yml"
    ".github/pull_request_template.md"
  )
  for f in "${required_files[@]}"; do
    if [[ ! -f "$f" ]]; then
      err "Required file missing: $f"
    fi
  done
  log "Required repository files are present."
}

validate_json_files() {
  local json_files=(
    "backlog.v1.json"
    "backlog.v1.schema.json"
    "backlog.v1.strict.json"
    "agent.autonomy.rules.json"
    "story-status.json"
  )
  for jf in "${json_files[@]}"; do
    python3 -m json.tool "$jf" >/dev/null
  done
  log "All JSON files are syntactically valid."
}

validate_backlog_schema() {
  if ! python3 -c "import jsonschema" >/dev/null 2>&1; then
    warn "python jsonschema package not installed; skipping schema validation."
    warn "Install with: pip install jsonschema"
    return 0
  fi
  python3 -m jsonschema -i backlog.v1.json backlog.v1.schema.json
  log "backlog.v1.json validates against backlog.v1.schema.json."
}

run_optional_go_checks() {
  if [[ ! -f "go.mod" ]]; then
    warn "No go.mod found; skipping Go checks."
    return 0
  fi

  if ! command -v go >/dev/null 2>&1; then
    err "go.mod exists but Go is not installed."
  fi

  log "Running go fmt ./..."
  go fmt ./...

  log "Running go test ./..."
  go test ./...

  log "Running go test -race ./..."
  go test -race ./...

  if command -v golangci-lint >/dev/null 2>&1; then
    log "Running golangci-lint run"
    golangci-lint run
  else
    warn "golangci-lint not found; skipping."
  fi

  if command -v gosec >/dev/null 2>&1; then
    log "Running gosec ./..."
    gosec ./...
  else
    warn "gosec not found; skipping."
  fi
}

init_local_git_if_requested() {
  if [[ "${INIT_LOCAL_GIT:-0}" != "1" ]]; then
    return 0
  fi

  if [[ -d ".git" ]]; then
    log "Git repository already initialized; skipping git init."
    return 0
  fi

  log "Initializing local git repository (INIT_LOCAL_GIT=1)."
  git init
  git checkout -b main
  log "Initialized local git repository with main branch."
}

check_gh_auth_if_available() {
  if ! command -v gh >/dev/null 2>&1; then
    warn "gh CLI not installed; PR automation commands will be unavailable."
    return 0
  fi

  if gh auth status >/dev/null 2>&1; then
    log "gh auth status: authenticated."
  else
    warn "gh CLI found but not authenticated. Run: gh auth login"
  fi
}

print_next_steps() {
  cat <<'EOF'

Next steps:
1) Pick next story from backlog.v1.strict.json with dependencies satisfied.
2) Create branch:
   git checkout main
   git pull origin main
   git checkout -b feat/<story-id-lower>-<slug>
3) Implement scope + tests from selected story.
4) Open PR:
   gh pr create --title "<STORY_ID>: <short title>" --body-file .github/pull_request_template.md
5) Merge (after required checks are green):
   gh pr merge --squash --delete-branch --auto

EOF
}

main() {
  require_cmd "python3"
  check_optional_cmd "git"
  check_optional_cmd "gh"
  check_optional_cmd "go"
  check_optional_cmd "golangci-lint"
  check_optional_cmd "gosec"

  validate_required_files
  validate_json_files
  validate_backlog_schema
  run_optional_go_checks
  init_local_git_if_requested
  check_gh_auth_if_available
  print_next_steps

  log "Bootstrap completed successfully."
}

main "$@"
