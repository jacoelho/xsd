package uriref

import (
	"errors"
	"strings"
)

// ErrOpaqueBase is returned when a relative path cannot be resolved against
// a non-hierarchical base URI.
var ErrOpaqueBase = errors.New("relative URI reference cannot be resolved against an opaque base")

// Parts is the parsed component view of a validated Reference. Presence flags
// preserve the distinction between absent and explicitly empty components.
type Parts struct {
	Scheme       string
	Authority    string
	Path         string
	Query        string
	Fragment     string
	HasScheme    bool
	HasAuthority bool
	HasQuery     bool
	HasFragment  bool
}

// Parts returns a copy of the reference's parsed components.
func (r Reference) Parts() Parts {
	return r.parts
}

// Resolve resolves ref against base using the validating RFC 2396 section 5.2
// algorithm. It retains excess leading ".." segments, as shown in RFC 2396
// Appendix C, and does not normalize dot segments in absolute-path or
// scheme-bearing references.
func Resolve(base, ref Reference) (Reference, error) {
	b := base.parts
	r := ref.parts
	if isSameDocumentReference(r) {
		return resolveSameDocumentReference(base, r)
	}
	if r.HasScheme {
		return ref, nil
	}
	if r.HasAuthority {
		r.Scheme, r.HasScheme = b.Scheme, b.HasScheme
		return compose(r)
	}
	if r.Path != "" && r.Path[0] == '/' {
		r.Scheme, r.HasScheme = b.Scheme, b.HasScheme
		r.Authority, r.HasAuthority = b.Authority, b.HasAuthority
		return compose(r)
	}
	switch {
	case opaque(b):
		if r.Path != "" {
			return Reference{}, ErrOpaqueBase
		}
		// RFC 2396 permits relative resolution only for hierarchical bases.
		// Query-only references are retained as a narrow XML Base extension,
		// preserving the opaque path while replacing its query.
		r.Path = b.Path
	case r.Path == "":
		r.Path = b.Path
	default:
		r.Path = removeRelativeDotSegments(mergePath(b, r.Path))
	}
	r.Scheme, r.HasScheme = b.Scheme, b.HasScheme
	r.Authority, r.HasAuthority = b.Authority, b.HasAuthority
	return compose(r)
}

func isSameDocumentReference(c Parts) bool {
	return c.Path == "" && !c.HasScheme && !c.HasAuthority && !c.HasQuery
}

func resolveSameDocumentReference(base Reference, ref Parts) (Reference, error) {
	without := base.WithoutFragment()
	if !ref.HasFragment {
		return without, nil
	}
	b := base.parts
	return compose(Parts{
		Scheme: b.Scheme, Authority: b.Authority, Path: b.Path, Query: b.Query,
		Fragment: ref.Fragment, HasScheme: b.HasScheme, HasAuthority: b.HasAuthority,
		HasQuery: b.HasQuery, HasFragment: true,
	})
}

func compose(c Parts) (Reference, error) {
	return Parse(spell(c))
}

func spell(c Parts) string {
	size := len(c.Path)
	if c.HasScheme {
		size += len(c.Scheme) + 1
	}
	if c.HasAuthority {
		size += len(c.Authority) + 2
	}
	if c.HasQuery {
		size += len(c.Query) + 1
	}
	if c.HasFragment {
		size += len(c.Fragment) + 1
	}
	var out strings.Builder
	out.Grow(size)
	if c.HasScheme {
		out.WriteString(c.Scheme)
		out.WriteByte(':')
	}
	if c.HasAuthority {
		out.WriteString("//")
		out.WriteString(c.Authority)
	}
	out.WriteString(c.Path)
	if c.HasQuery {
		out.WriteByte('?')
		out.WriteString(c.Query)
	}
	if c.HasFragment {
		out.WriteByte('#')
		out.WriteString(c.Fragment)
	}
	return out.String()
}

func split(raw string) Parts {
	var c Parts
	rest := raw
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		c.Fragment, c.HasFragment = rest[i+1:], true
		rest = rest[:i]
	}
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		c.Query, c.HasQuery = rest[i+1:], true
		rest = rest[:i]
	}
	if i := schemeSeparator(rest); i >= 0 {
		c.Scheme, c.HasScheme = rest[:i], true
		rest = rest[i+1:]
	}
	if strings.HasPrefix(rest, "//") {
		c.HasAuthority = true
		rest = rest[2:]
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			c.Authority, c.Path = rest[:i], rest[i:]
		} else {
			c.Authority = rest
		}
	} else {
		c.Path = rest
	}
	return c
}

func schemeSeparator(raw string) int {
	for i := range len(raw) {
		switch raw[i] {
		case ':':
			return i
		case '/':
			return -1
		}
	}
	return -1
}

func opaque(c Parts) bool {
	return c.HasScheme && !c.HasAuthority && (c.Path == "" || c.Path[0] != '/')
}

func mergePath(base Parts, ref string) string {
	if i := strings.LastIndexByte(base.Path, '/'); i >= 0 {
		return base.Path[:i+1] + ref
	}
	if base.HasAuthority {
		return "/" + ref
	}
	return ref
}

func removeRelativeDotSegments(path string) string {
	absolute := strings.HasPrefix(path, "/")
	trailing := strings.HasSuffix(path, "/") || strings.HasSuffix(path, "/.") || strings.HasSuffix(path, "/..") || path == "." || path == ".."
	parts := strings.Split(path, "/")
	stack := make([]string, 0, len(parts))
	start := 0
	if absolute {
		start = 1
	}
	for _, part := range parts[start:] {
		stack = applyRelativeDotSegment(stack, part)
	}
	result := strings.Join(stack, "/")
	if absolute {
		result = "/" + result
	}
	if trailing && result != "" && !strings.HasSuffix(result, "/") {
		result += "/"
	}
	return result
}

func applyRelativeDotSegment(stack []string, part string) []string {
	switch part {
	case ".":
		return stack
	case "..":
		if len(stack) > 0 && stack[len(stack)-1] != ".." {
			return stack[:len(stack)-1]
		}
	}
	return append(stack, part)
}
