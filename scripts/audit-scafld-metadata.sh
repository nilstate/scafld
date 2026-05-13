#!/usr/bin/env bash
set -uo pipefail

version="${1:-}"
if [[ -z "$version" ]]; then
  echo "usage: $0 <version>" >&2
  exit 2
fi
version="${version#v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]]; then
  echo "usage: $0 <version>" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tag="v${version}"
module="github.com/nilstate/scafld/v2"
cmd_pkg="${module}/cmd/scafld"
canonical="https://0state.com/scafld"

failures=0
warnings=0
pending=0
rows=()
tmp_dirs=()

cleanup() {
  local dir
  for dir in "${tmp_dirs[@]}"; do
    rm -rf "$dir"
  done
}
trap cleanup EXIT

escape_md() {
  local value="$*"
  value="${value//$'\n'/ }"
  value="${value//|/\\|}"
  printf '%s' "$value"
}

record() {
  local status="$1"
  local surface="$2"
  local detail="$3"
  case "$status" in
    FAIL) failures=$((failures + 1)) ;;
    WARN) warnings=$((warnings + 1)) ;;
    PENDING) pending=$((pending + 1)) ;;
  esac
  rows+=("| ${status} | $(escape_md "$surface") | $(escape_md "$detail") |")
}

need_cmd() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    record FAIL "$name" "required command not found"
    return 1
  fi
  return 0
}

http_code() {
  curl -L -sS -o /dev/null -w '%{http_code}' "$1" 2>/dev/null || printf '000'
}

fetch_body() {
  curl -fsSL \
    -H 'Accept: application/vnd.github.raw' \
    -H 'Cache-Control: no-cache' \
    "$1" 2>/dev/null
}

check_http_200() {
  local surface="$1"
  local url="$2"
  local code
  code="$(http_code "$url")"
  if [[ "$code" == "200" ]]; then
    record PASS "$surface" "$url"
  else
    record FAIL "$surface" "$url returned HTTP $code"
  fi
}

check_file_contains() {
  local surface="$1"
  local path="$2"
  local needle="$3"
  if grep -Fq "$needle" "$path"; then
    record PASS "$surface" "$path contains $needle"
  else
    record FAIL "$surface" "$path does not contain $needle"
  fi
}

check_url_contains() {
  local surface="$1"
  local url="$2"
  local needle="$3"
  local body
  body="$(fetch_body "$url")"
  if [[ $? -ne 0 ]]; then
    record FAIL "$surface" "$url could not be fetched"
    return
  fi
  if grep -Fq "$needle" <<<"$body"; then
    record PASS "$surface" "$url contains $needle"
  else
    record FAIL "$surface" "$url does not contain $needle"
  fi
}

check_go_proxy() {
  if ! need_cmd go; then
    return
  fi
  local out
  out="$(GOPROXY=proxy.golang.org go list -m "${module}@${tag}" 2>&1)"
  if [[ $? -eq 0 && "$out" == "${module} ${tag}" ]]; then
    record PASS "Go proxy" "$out"
  else
    record FAIL "Go proxy" "$out"
  fi
}

check_go_install() {
  if ! need_cmd go; then
    return
  fi
  local tmp gobin out
  tmp="$(mktemp -d)"
  tmp_dirs+=("$tmp")
  gobin="$tmp/bin"
  mkdir -p "$gobin"
  out="$(GOBIN="$gobin" GOPROXY=proxy.golang.org go install "${cmd_pkg}@${tag}" 2>&1)"
  if [[ $? -ne 0 ]]; then
    record FAIL "go install" "$out"
    return
  fi
  out="$("$gobin/scafld" --version 2>&1)"
  if [[ "$out" == *"$version"* ]]; then
    record PASS "go install" "${cmd_pkg}@${tag} -> $out"
  else
    record FAIL "go install" "installed binary version output did not contain $version: $out"
  fi
}

check_deps_dev() {
  if ! need_cmd jq; then
    return
  fi
  local url json
  url="https://api.deps.dev/v3/systems/go/packages/github.com%2Fnilstate%2Fscafld%2Fv2"
  json="$(curl -fsSL "$url" 2>/dev/null)"
  if [[ $? -ne 0 ]]; then
    record FAIL "deps.dev" "$url could not be fetched"
    return
  fi
  if jq -e --arg tag "$tag" '.versions[]?.versionKey.version == $tag' >/dev/null <<<"$json"; then
    record PASS "deps.dev" "${module}@${tag} is indexed"
  else
    record PENDING "deps.dev" "${module}@${tag} not indexed yet"
  fi
}

check_npm() {
  if ! need_cmd jq; then
    return
  fi
  local url json homepage description
  url="https://registry.npmjs.org/scafld/${version}"
  json="$(curl -fsSL "$url" 2>/dev/null)"
  if [[ $? -ne 0 ]]; then
    record PENDING "npm" "$url could not be fetched"
    return
  fi
  homepage="$(jq -r '.homepage // ""' <<<"$json")"
  description="$(jq -r '.description // ""' <<<"$json")"
  if [[ "$homepage" == "$canonical" ]]; then
    record PASS "npm" "homepage is $canonical"
  else
    record FAIL "npm" "published homepage is ${homepage:-empty}"
  fi
  if [[ "$description" == "Deterministic protocol for multi-phase agent work." ]]; then
    record PASS "npm" "description is current"
  else
    record WARN "npm" "published description is: ${description:-empty}"
  fi
}

check_pypi() {
  if ! need_cmd jq; then
    return
  fi
  local url json homepage summary
  url="https://pypi.org/pypi/scafld/${version}/json"
  json="$(curl -fsSL "$url" 2>/dev/null)"
  if [[ $? -ne 0 ]]; then
    record PENDING "PyPI" "$url could not be fetched"
    return
  fi
  homepage="$(jq -r '.info.project_urls.Homepage // .info.home_page // ""' <<<"$json")"
  summary="$(jq -r '.info.summary // ""' <<<"$json")"
  if [[ "$homepage" == "$canonical" ]]; then
    record PASS "PyPI" "Homepage is $canonical"
  else
    record FAIL "PyPI" "published Homepage is ${homepage:-empty}"
  fi
  if [[ "$summary" == "Deterministic protocol for multi-phase agent work." ]]; then
    record PASS "PyPI" "summary is current"
  else
    record WARN "PyPI" "published summary is: ${summary:-empty}"
  fi
}

check_homebrew() {
  local url
  url="https://api.github.com/repos/nilstate/homebrew-tap/contents/Formula/scafld.rb?ref=main"
  check_url_contains "Homebrew" "$url" "version \"${version}\""
  check_url_contains "Homebrew" "$url" "homepage \"${canonical}\""
}

check_scoop() {
  if ! need_cmd jq; then
    return
  fi
  local url json homepage live_version
  url="https://api.github.com/repos/nilstate/scoop-bucket/contents/bucket/scafld.json?ref=main"
  json="$(fetch_body "$url")"
  if [[ $? -ne 0 ]]; then
    record FAIL "Scoop" "$url could not be fetched"
    return
  fi
  live_version="$(jq -r '.version // ""' <<<"$json")"
  homepage="$(jq -r '.homepage // ""' <<<"$json")"
  if [[ "$live_version" == "$version" ]]; then
    record PASS "Scoop" "version is $version"
  else
    record FAIL "Scoop" "version is ${live_version:-empty}"
  fi
  if [[ "$homepage" == "$canonical" ]]; then
    record PASS "Scoop" "homepage is $canonical"
  else
    record FAIL "Scoop" "homepage is ${homepage:-empty}"
  fi
}

check_winget() {
  local url code
  url="https://api.github.com/repos/microsoft/winget-pkgs/contents/manifests/0/0state/scafld/${version}/0state.scafld.locale.en-US.yaml?ref=master"
  code="$(http_code "$url")"
  if [[ "$code" == "404" ]]; then
    record PENDING "WinGet" "version ${version} is not merged upstream yet"
    return
  fi
  if [[ "$code" != "200" ]]; then
    record FAIL "WinGet" "$url returned HTTP $code"
    return
  fi
  check_url_contains "WinGet" "$url" "PackageUrl: ${canonical}"
}

check_ghcr() {
  if ! command -v docker >/dev/null 2>&1; then
    record WARN "GHCR" "docker not found; cannot inspect ghcr.io/nilstate/scafld:${tag}"
    return
  fi
  local out
  out="$(docker manifest inspect "ghcr.io/nilstate/scafld:${tag}" 2>&1 >/dev/null)"
  if [[ $? -eq 0 ]]; then
    record PASS "GHCR" "ghcr.io/nilstate/scafld:${tag} is inspectable"
  else
    record FAIL "GHCR" "ghcr.io/nilstate/scafld:${tag} is not publicly inspectable: $out"
  fi
}

check_github_repo() {
  if ! command -v gh >/dev/null 2>&1; then
    record WARN "GitHub repo" "gh not found; cannot inspect repository metadata"
    return
  fi
  if ! need_cmd jq; then
    return
  fi
  local json homepage description
  json="$(gh repo view nilstate/scafld --json homepageUrl,description 2>/dev/null)"
  if [[ $? -ne 0 ]]; then
    record WARN "GitHub repo" "gh repo view failed"
    return
  fi
  homepage="$(jq -r '.homepageUrl // ""' <<<"$json")"
  description="$(jq -r '.description // ""' <<<"$json")"
  if [[ "$homepage" == "$canonical" ]]; then
    record PASS "GitHub repo" "homepage is $canonical"
  else
    record FAIL "GitHub repo" "homepage is ${homepage:-empty}"
  fi
  if [[ "$description" == "Deterministic protocol for multi-phase agent work." ]]; then
    record PASS "GitHub repo" "description is current"
  else
    record WARN "GitHub repo" "description is: ${description:-empty}"
  fi
}

check_source_metadata() {
  check_file_contains "npm source" "$root/package/npm/package.json" "\"homepage\": \"${canonical}\""
  check_file_contains "PyPI source" "$root/package/pypi/pyproject.toml" "Homepage = \"${canonical}\""
  check_file_contains "Homebrew source" "$root/package/homebrew/scafld.rb.tmpl" "homepage \"${canonical}\""
  check_file_contains "Scoop source" "$root/package/scoop/scafld.json.tmpl" "\"homepage\": \"${canonical}\""
  check_file_contains "WinGet source" "$root/package/winget/scafld.yaml.tmpl" "PackageUrl: ${canonical}"
}

check_go_proxy
check_go_install
check_http_200 "pkg.go.dev module" "https://pkg.go.dev/${module}"
check_http_200 "pkg.go.dev command" "https://pkg.go.dev/${cmd_pkg}"
check_deps_dev
check_npm
check_pypi
check_homebrew
check_scoop
check_winget
check_ghcr
check_github_repo
check_source_metadata

cat <<EOF
# scafld Package Metadata Audit

Version: ${tag}
Canonical project URL: ${canonical}

| Status | Surface | Detail |
| --- | --- | --- |
EOF
printf '%s\n' "${rows[@]}"
cat <<EOF

Summary: ${failures} fail, ${warnings} warn, ${pending} pending.
EOF

if [[ "$failures" -gt 0 ]]; then
  exit 1
fi
