#!/usr/bin/env bash

set -euo pipefail

echo "==> architecture guardrails"

echo "-> checking package graph compiles"
go list ./... >/dev/null

echo "-> reporting internal package fan-out (top 12)"
go list -f '{{.ImportPath}} {{join .Imports " "}}' ./internal/... \
  | awk '{
      count=0
      for (i=2; i<=NF; i++) {
        if ($i ~ /^github.com\/jacoelho\/xsd\/internal\//) {
          count++
        }
      }
      printf "%3d %s\n", count, $1
    }' \
  | sort -nr \
  | head -n 12

echo "-> enforcing validator production import boundaries"
violations="$(
  rg -n 'github.com/jacoelho/xsd/internal/(semantic|semanticcheck|semanticresolve|runtimecompile)' \
    internal/validator \
    --glob '!**/*_test.go' \
    || true
)"

if [[ -n "${violations}" ]]; then
  echo "ERROR: forbidden imports found in internal/validator non-test files:"
  echo "${violations}"
  exit 1
fi

echo "-> guardrails passed"
