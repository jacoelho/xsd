# Prints its input with paths relative to XSD_ORACLE_ROOT, so a row is identical on every
# machine: file://ROOT/, file:ROOT/ and ROOT/ are removed, then a bare ROOT becomes ".".
# File URIs percent-encode the root (RFC 3986 section 2.1), so its encoded form is removed
# first. Run under LC_ALL=C so the encoding works on bytes.
function replace(s, from, to,    out, i) {
    out = ""
    while ((i = index(s, from)) > 0) {
        out = out substr(s, 1, i - 1) to
        s = substr(s, i + length(from))
    }
    return out s
}

function encoded(s,    out, i, c) {
    out = ""
    for (i = 1; i <= length(s); i++) {
        c = substr(s, i, 1)
        out = out (c ~ /[A-Za-z0-9._~\/-]/ ? c : sprintf("%%%02X", code[c]))
    }
    return out
}

function strip(s, r) {
    s = replace(s, "file://" r "/", "")
    s = replace(s, "file:" r "/", "")
    s = replace(s, r "/", "")
    return replace(s, r, ".")
}

BEGIN {
    for (i = 1; i < 256; i++) code[sprintf("%c", i)] = i
    root = ENVIRON["XSD_ORACLE_ROOT"]
    encodedRoot = encoded(root)
}

{ print strip(strip($0, encodedRoot), root) }
