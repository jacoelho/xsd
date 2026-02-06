#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

rules_file="scripts/internal-deps-forbidden.txt"
if [[ ! -f "$rules_file" ]]; then
	echo "missing dependency rules file: $rules_file" >&2
	exit 1
fi

rules=()
while IFS= read -r line; do
	rules+=("$line")
done < <(awk 'NF == 2 && $1 !~ /^#/' "$rules_file")

if [[ ${#rules[@]} -eq 0 ]]; then
	exit 0
fi

module_path=$(go list -m -f '{{.Path}}')
violations=()

while IFS='|' read -r pkg deps_csv; do
	pkg=${pkg#"$module_path"/}
	for rule in "${rules[@]}"; do
		from=${rule%% *}
		to=${rule##* }
		if [[ "$pkg" != "$from" ]]; then
			continue
		fi
		target="$module_path/$to"
		if [[ ",$deps_csv," == *",$target,"* ]]; then
			violations+=("$from imports forbidden dependency $to")
		fi
	done
done < <(go list -f '{{.ImportPath}}|{{join .Imports ","}}' ./internal/...)

if [[ ${#violations[@]} -gt 0 ]]; then
	echo "forbidden internal dependencies found:"
	printf '  - %s\n' "${violations[@]}"
	exit 1
fi
