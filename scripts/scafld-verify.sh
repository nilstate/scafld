#!/usr/bin/env sh
set -eu

receipt="${SCAFLD_RECEIPT_PATH:-${1:-}}"
target="${SCAFLD_VERIFY_TARGET:-${GITHUB_BASE_REF:-}}"

if [ -z "$receipt" ]; then
  receipt=".scafld/receipts/latest.json"
fi

if [ -z "$target" ]; then
  if [ -n "${GITHUB_BASE_SHA:-}" ]; then
    target="$GITHUB_BASE_SHA"
  elif [ -n "${GITHUB_EVENT_PULL_REQUEST_BASE_SHA:-}" ]; then
    target="$GITHUB_EVENT_PULL_REQUEST_BASE_SHA"
  fi
fi

if [ -z "$target" ]; then
  echo "error: SCAFLD_VERIFY_TARGET or a GitHub base ref/sha is required" >&2
  exit 2
fi

scafld verify "$receipt" --target "$target"
