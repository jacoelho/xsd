#!/bin/sh
# Usage: oracle.sh INSTANCE SCHEMA...; INSTANCE is "" for a schema-only run.
# Prints "version<TAB>verdict<TAB>message".
set -eu

[ $# -ge 2 ] || { echo "usage: oracle.sh INSTANCE SCHEMA..." >&2; exit 2; }

version=$(xmllint --version 2>&1 | sed -n '1s/.*libxml version \([0-9]*\).*/\1/p')
version="libxml2 $((version / 10000)).$((version / 100 % 100)).$((version % 100))"

emit() {
    printf '%s\t%s\t%s\n' "$version" "$1" "$(printf '%s' "$2" | tr '\n\t' '  ')"
}

# First diagnostic other than xmllint's per-file summary lines.
first_diagnostic() {
    printf '%s\n' "$1" | grep -v -e ' validates$' -e ' fails to validate$' -e '^-:' | sed -n 1p
}

instance=$1
shift
if [ $# -gt 1 ]; then
    emit blocked "xmllint --schema accepts one schema root; $# given"
    exit 0
fi

rc=0
if [ -z "$instance" ]; then
    # xmllint compiles a schema only alongside an instance; only the compile outcome counts here.
    out=$(printf '<xsd-oracle-probe/>' | xmllint --noout --nonet --schema "$1" - 2>&1) || rc=$?
    case $rc in 3) rc=0 ;; 1) rc=5 ;; esac
    valid=schema-valid
else
    out=$(xmllint --noout --nonet --schema "$1" "$instance" 2>&1) || rc=$?
    valid=valid
fi
diag=$(first_diagnostic "$out")
# xmllint has no resolver hook; its load-failure diagnostics mark a reference absent from the
# mount. A reference to an existing image file outside the mount is not detected.
missing=$(printf '%s\n' "$out" | grep -e 'failed to load external entity' -e 'Attempt to load network entity' | sed -n 1p) || :
if [ -n "$missing" ]; then
    emit blocked "acquisition blocked: $missing"
    exit 0
fi
# libxml2 reports unsupported constructs, such as unsubstituted entity references, as internal errors.
internal=$(printf '%s\n' "$out" | grep -e 'Internal error' | sed -n 1p) || :
if [ -n "$internal" ]; then
    emit error "$internal"
    exit 0
fi
case $rc in
    0) emit "$valid" "${diag:+warning: $diag}" ;;
    1 | 3) emit invalid "$diag" ;;
    5) emit schema-invalid "$diag" ;;
    *) emit error "xmllint exit $rc: $diag" ;;
esac
