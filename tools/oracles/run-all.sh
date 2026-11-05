#!/bin/sh
# In-image entrypoint. Usage: run-all INSTANCE SCHEMA...; INSTANCE is "" for a schema-only run.
# Runs each adapter in a fresh process and prints one tab-separated row per validator.
set -eu

[ $# -ge 2 ] && [ -n "${XSD_ORACLE_ROOT:-}" ] || {
    echo "usage: XSD_ORACLE_ROOT=DIR run-all INSTANCE SCHEMA..." >&2
    exit 2
}

timeout_s=${XSD_ORACLE_TIMEOUT:-120}

row() {
    key=$1
    shift
    err=/tmp/$key.stderr
    rc=0
    out=$(timeout "$timeout_s" "$@" 2>"$err") || rc=$?
    if [ "$rc" -eq 124 ]; then
        out="?	error	timed out after ${timeout_s}s"
    elif [ "$rc" -ne 0 ] || [ -z "$out" ]; then
        out="?	error	exit $rc: $(tail -n 3 "$err" | tr '\n\t' '  ')"
    fi
    printf '%s\t%s\n' "$key" "$out"
}

printf 'oracle\tversion\tverdict\tmessage\n'
row xerces java -cp /opt/oracles/xerces/xercesImpl.jar:/opt/oracles/xerces XercesOracle "$@"
row dotnet dotnet /opt/oracles/dotnet/DotnetOracle.dll "$@"
row xmlschema env PYTHONPATH=/opt/oracles/xmlschema/lib python3.10 -s /opt/oracles/xmlschema/oracle.py "$@"
row libxml2 /opt/oracles/libxml2/oracle.sh "$@"
