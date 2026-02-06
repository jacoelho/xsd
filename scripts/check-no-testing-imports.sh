#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

offenders=$(
	rg -n '^\s*([A-Za-z_][A-Za-z0-9_]*\s+)?"testing"$' \
		--glob '*.go' \
		--glob '!**/*_test.go' \
		. || true
)

if [[ -n "$offenders" ]]; then
	echo "non-test Go files must not import testing:"
	echo "$offenders"
	exit 1
fi

