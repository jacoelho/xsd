#!/bin/sh
set -eu

usage() {
    cat <<'EOF'
usage: tools/oracles/run.sh [--root DIR] [--schema EXTRA.xsd]... schema.xsd [instance.xml]

Compiles the schema roots, then validates the instance when given, with Xerces-J,
.NET System.Xml, Python xmlschema (XSD 1.0) and libxml2, each in a fresh process.
Prints one tab-separated row per validator: oracle key, version, verdict, first
diagnostic. Each --schema adds a schema root, in order; libxml2 accepts only one
and reports blocked when given more.

Verdicts:
  schema-valid    schema compiled; no instance given
  schema-invalid  schema compilation reported an error; instance not validated
  valid           instance validated without errors
  invalid         instance not well-formed or not valid
  blocked         a schema or entity reference resolved to a non-file URI or to a
                  file absent from DIR, or the adapter cannot represent the
                  roots; no validity result
  error           adapter or provider failure or timeout; no validity result

A "| warning:" suffix, or a sole "warning:" message, is the first provider warning.

Validators see only DIR, mounted read-only at its host path, with no network.
DIR defaults to the deepest directory containing every input file; pass --root
when the schema closure reaches outside it (for example via ../). DIR may not
contain a comma or double quote. Xerces-J, .NET and xmlschema block any
reference outside DIR; libxml2 blocks only references that fail to load, so it
can read an existing image file named by an absolute reference.

Environment:
  XSD_ORACLES_IMAGE    image tag to build and run (default xsd-oracles:local)
  XSD_ORACLE_TIMEOUT   per-validator time limit in seconds (default 120)
EOF
}

die() {
    echo "run.sh: $*" >&2
    exit 2
}

absolute() {
    [ -f "$1" ] || die "no such file: $1"
    printf '%s/%s\n' "$(cd "$(dirname "$1")" && pwd -P)" "$(basename "$1")"
}

common_dir() {
    dir=$1
    while :; do
        case "$2/" in "$dir"/*) break ;; esac
        [ "$dir" != / ] || break
        dir=$(dirname "$dir")
    done
    printf '%s\n' "$dir"
}

# Paths are kept newline-separated; paths containing newlines are unsupported.
root=
extras=
while [ $# -gt 0 ]; do
    case "$1" in
        -h | --help) usage; exit 0 ;;
        --root) [ $# -ge 2 ] || die "--root needs a directory"; root=$2; shift 2 ;;
        --schema)
            [ $# -ge 2 ] || die "--schema needs a file"
            extras="$extras$(absolute "$2")
"
            shift 2
            ;;
        --) shift; break ;;
        -*) die "unknown option: $1" ;;
        *) break ;;
    esac
done
[ $# -ge 1 ] && [ $# -le 2 ] || { usage >&2; exit 2; }

schema=$(absolute "$1")
instance=
[ $# -eq 1 ] || instance=$(absolute "$2")
inputs="$schema
$extras$instance"

# Split the newline-separated lists without globbing.
set -f
IFS='
'
if [ -n "$root" ]; then
    [ -d "$root" ] || die "no such directory: $root"
    root=$(cd "$root" && pwd -P)
else
    root=$(dirname "$schema")
    for f in $inputs; do root=$(common_dir "$root" "$(dirname "$f")"); done
fi
[ "$root" != / ] || die "refusing to mount /; pass a narrower --root"
case "$root" in *,* | *\"*) die "root $root contains a comma or double quote; pass a different --root" ;; esac
for f in $inputs; do
    case "$f" in "$root"/*) ;; *) die "$f is outside root $root" ;; esac
done

# Container arguments: INSTANCE ("" for schema-only) followed by every schema root.
set -- "$instance" "$schema"
for f in $extras; do set -- "$@" "$f"; done
unset IFS
set +f

image=${XSD_ORACLES_IMAGE:-xsd-oracles:local}
here=$(cd "$(dirname "$0")" && pwd -P)
docker build --quiet --tag "$image" "$here" >/dev/null

exec docker run --rm --network none --read-only --tmpfs /tmp \
    --user "$(id -u):$(id -g)" \
    --env XSD_ORACLE_TIMEOUT="${XSD_ORACLE_TIMEOUT:-120}" \
    --env XSD_ORACLE_ROOT="$root" \
    --mount "type=bind,source=$root,target=$root,readonly" \
    "$image" "$@"
