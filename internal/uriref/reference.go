// Package uriref validates and resolves the extended URI references used by
// XML Schema 1.0 and XML Base.
package uriref

import (
	"errors"
	"strings"
)

// ErrInvalid is returned when applying the XLink escaping procedure would
// not produce an RFC 2396 URI reference, as amended by RFC 2732.
var ErrInvalid = errors.New("invalid anyURI")

// Reference is a validated URI reference. Raw retains the normalized XML
// Schema value; Escaped materializes the URI spelling passed to URI backends.
// The zero value is the valid empty reference. Every Reference is one that
// [Parse] returns for its Raw spelling.
//
// Raw is the anyURI value and the spelling presented to custom resolvers.
// Escaped exists only for built-in generic and file fallback, which may be
// unable to represent a reference that a resolver accepts.
type Reference struct {
	raw                  string
	layout               layout
	escapedLen           int
	escapedFragmentStart int
}

// Check validates text as [Parse] does and returns its number of Unicode
// characters. It does not allocate.
func Check[T byteText](text T) (int, error) {
	s, _, err := analyze(text)
	if err != nil {
		return 0, err
	}
	return s.characters, nil
}

// Parse validates text as an RFC 2396 URI reference, as amended by RFC 2732,
// after XLink escaping, and returns its immutable reference representation.
func Parse(text string) (Reference, error) {
	s, l, err := analyze(text)
	if err != nil {
		return Reference{}, err
	}
	return Reference{raw: text, layout: l, escapedLen: s.escapedLen, escapedFragmentStart: s.escapedFragmentStart}, nil
}

// Raw returns the normalized XML Schema spelling.
func (r Reference) Raw() string { return r.raw }

// Escaped returns the RFC 2396 spelling obtained by applying XLink escaping.
// It aliases Raw when no escaping is required.
func (r Reference) Escaped() string {
	if r.escapedLen == len(r.raw) {
		return r.raw
	}
	var out strings.Builder
	out.Grow(r.escapedLen)
	for i := 0; i < len(r.raw); {
		b := r.raw[i]
		switch {
		case b >= 0x80:
			n := utf8SequenceLen(r.raw, i)
			for _, encoded := range []byte(r.raw[i : i+n]) {
				writeEscape(&out, encoded)
			}
			i += n
		case mustEscapeASCII(b):
			writeEscape(&out, b)
			i++
		default:
			out.WriteByte(b)
			i++
		}
	}
	return out.String()
}

// HasFragment reports whether the fragment separator is present, including
// when the fragment is empty.
func (r Reference) HasFragment() bool {
	return r.layout.hasFragment
}

// WithoutFragment removes the fragment separator and fragment, if present.
// The result keeps every other component span: RFC 2396 section 4 makes a URI
// reference an absolute or relative URI followed by an optional fragment, so
// the prefix before '#' is a valid reference with the same components.
func (r Reference) WithoutFragment() Reference {
	if !r.layout.hasFragment {
		return r
	}
	l := r.layout
	l.hasFragment = false
	return Reference{
		raw:                  r.raw[:l.fragmentStart],
		layout:               l,
		escapedLen:           r.escapedFragmentStart,
		escapedFragmentStart: r.escapedFragmentStart,
	}
}
