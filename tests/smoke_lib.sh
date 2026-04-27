SMOKE_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JSON_ASSERT_PY="$SMOKE_LIB_DIR/json_assert.py"

if ! declare -p TMP_DIRS >/dev/null 2>&1; then
  TMP_DIRS=()
fi

smoke_cleanup() {
  if [ "${#TMP_DIRS[@]}" -gt 0 ]; then
    rm -rf "${TMP_DIRS[@]}"
  fi
}
trap smoke_cleanup EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

capture() {
  local __var="$1"
  shift
  local _captured
  set +e
  _captured="$("$@" 2>&1)"
  local status=$?
  set -e
  printf -v "$__var" '%s' "$_captured"
  return "$status"
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    fail "$message"
  fi
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  if [[ "$haystack" == *"$needle"* ]]; then
    fail "$message"
  fi
}

assert_contains_file() {
  local file="$1"
  local needle="$2"
  local message="$3"
  grep -Fq -- "$needle" "$file" || fail "$message"
}

assert_json() {
  local payload="$1"
  local expression="$2"
  local message="$3"
  JSON_PAYLOAD="$payload" python3 "$JSON_ASSERT_PY" "$expression" "$message" || fail "$message"
}

run_pty_command() {
  local command="$1"
  SCAFLD_SMOKE_PTY_COMMAND="$command" python3 "$SMOKE_LIB_DIR/pty_run.py"
}

complete_human_review_pty() {
  local repo="$1"
  local task_id="$2"
  local reason="$3"
  (
    cd "$repo"
    export PATH="$CLI_ROOT:$PATH"
    export SCAFLD_SMOKE_TASK_ID="$task_id"
    export SCAFLD_SMOKE_OVERRIDE_REASON="$reason"
    printf '%s\n' "$task_id" | run_pty_command 'scafld complete "$SCAFLD_SMOKE_TASK_ID" --human-reviewed --reason "$SCAFLD_SMOKE_OVERRIDE_REASON"'
  )
}
