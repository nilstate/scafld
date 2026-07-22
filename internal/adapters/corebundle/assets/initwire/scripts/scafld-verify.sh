#!/usr/bin/env sh
set -eu

receipt="${SCAFLD_RECEIPT_PATH:-${1:-}}"
target="${SCAFLD_VERIFY_TARGET:-${2:-}}"
head_ref="${SCAFLD_VERIFY_HEAD:-HEAD}"
trusted_keys="${SCAFLD_TRUSTED_KEYS:-}"
version="${SCAFLD_VERSION:-v2.5.2}"
tmp_dir="${RUNNER_TEMP:-${TMPDIR:-/tmp}}"
mode="${SCAFLD_VERIFY_MODE:-full}"

if [ -z "$target" ] || [ "$target" = "0000000000000000000000000000000000000000" ]; then
  echo "error: SCAFLD_VERIFY_TARGET must be a base commit sha or ref" >&2
  exit 2
fi

case "$mode" in
  full|material) ;;
  *)
    echo "error: SCAFLD_VERIFY_MODE must be full or material" >&2
    exit 2
    ;;
esac

if [ "$mode" = "full" ] && [ "${SCAFLD_VERIFY_SANITIZED:-}" != "1" ]; then
  exec env -i \
    PATH="$PATH" \
    HOME="${HOME:-}" \
    USER="${USER:-}" \
    LOGNAME="${LOGNAME:-}" \
    CI="${CI:-}" \
    RUNNER_TEMP="${RUNNER_TEMP:-}" \
    TMPDIR="${TMPDIR:-}" \
    SCAFLD_VERIFY_SANITIZED=1 \
    SCAFLD_RECEIPT_PATH="$receipt" \
    SCAFLD_VERIFY_TARGET="$target" \
    SCAFLD_VERIFY_HEAD="$head_ref" \
    SCAFLD_TRUSTED_KEYS="$trusted_keys" \
    SCAFLD_VERSION="$version" \
    SCAFLD_VERIFY_MODE="$mode" \
    GOBIN="${GOBIN:-}" \
    GOPATH="${GOPATH:-}" \
    GOCACHE="${GOCACHE:-}" \
    GOMODCACHE="${GOMODCACHE:-}" \
    GOFLAGS="${GOFLAGS:-}" \
    CAPTURE_ARGS="${CAPTURE_ARGS:-}" \
    CAPTURE_RECEIPT="${CAPTURE_RECEIPT:-}" \
    sh "$0" "$@"
fi

current_head="$(git rev-parse HEAD 2>/dev/null || true)"
resolved_head=""
if [ "$head_ref" != "HEAD" ]; then
  resolved_head="$(git rev-parse "$head_ref^{commit}" 2>/dev/null || true)"
  if [ -z "$resolved_head" ]; then
    echo "error: could not resolve verify head $head_ref" >&2
    exit 2
  fi
fi

acceptance_root=""
cleanup_acceptance_root() {
  if [ -n "$acceptance_root" ]; then
    git worktree remove --force "$acceptance_root" >/dev/null 2>&1 || rm -rf "$acceptance_root"
  fi
}
trap cleanup_acceptance_root EXIT

if [ -z "$trusted_keys" ]; then
  mkdir -p "$tmp_dir"
  trusted_keys="$tmp_dir/scafld-trusted-keys.json"
  if ! git show "$target:.scafld/trusted-keys.json" > "$trusted_keys"; then
    echo "error: could not load .scafld/trusted-keys.json from verify target $target" >&2
    echo "       bootstrap the trusted key on the protected branch before enabling the merge gate" >&2
    exit 2
  fi
fi

if [ -z "$receipt" ]; then
  receipts="$(git diff --name-only "$target" "$head_ref" -- '.scafld/receipts/*.json' 2>/dev/null | grep -v '^.scafld/receipts/latest\.json$' || true)"
  count="$(printf '%s\n' "$receipts" | sed '/^$/d' | wc -l | tr -d ' ')"
  if [ "$count" = "1" ]; then
    receipt="$receipts"
  elif [ "$count" = "0" ] && [ -z "${CI:-}" ] && [ -f ".scafld/receipts/latest.json" ]; then
    receipt=".scafld/receipts/latest.json"
  else
    echo "error: expected exactly one changed task receipt, found $count" >&2
    printf '%s\n' "$receipts" >&2
    exit 2
  fi
fi

if [ -n "$resolved_head" ] && [ "$current_head" != "$resolved_head" ]; then
  case "$receipt" in
    /*) ;;
    *)
      mkdir -p "$tmp_dir"
      receipt_copy="$tmp_dir/scafld-receipt.json"
      if ! git show "$head_ref:$receipt" > "$receipt_copy"; then
        echo "error: could not load receipt $receipt from verify head $head_ref" >&2
        exit 2
      fi
      receipt="$receipt_copy"
      ;;
  esac
  if [ "$mode" = "full" ]; then
    acceptance_root="$(mktemp -d "${tmp_dir%/}/scafld-acceptance-root.XXXXXX")"
    rmdir "$acceptance_root"
    if ! git worktree add --detach "$acceptance_root" "$resolved_head" >/dev/null; then
      echo "error: could not create acceptance worktree for verify head $resolved_head" >&2
      exit 2
    fi
  fi
fi

if ! command -v scafld >/dev/null 2>&1; then
  if ! command -v go >/dev/null 2>&1; then
    echo "error: scafld is not on PATH and go is unavailable to install scafld@$version" >&2
    exit 2
  fi
  gobin="$(go env GOBIN)"
  if [ -z "$gobin" ]; then
    gobin="$(go env GOPATH)/bin"
  fi
  export PATH="$gobin:$PATH"
  go install "github.com/nilstate/scafld/v2/cmd/scafld@$version"
fi

if [ -n "$resolved_head" ]; then
  set -- "$receipt" --target "$target" --trusted-keys "$trusted_keys" --material-ref "$resolved_head"
else
  set -- "$receipt" --target "$target" --trusted-keys "$trusted_keys"
fi
if [ "$mode" = "material" ]; then
  set -- "$@" --material-only
fi
if [ -n "$acceptance_root" ]; then
  # The acceptance lane keeps scafld and trust anchors on the protected base
  # checkout, while recorded commands run from a detached fetched-head worktree.
  set -- "$@" --acceptance-root "$acceptance_root"
fi

scafld verify "$@"
