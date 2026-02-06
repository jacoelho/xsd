package value

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/xmlnames"
)

type mapResolver map[string]string

func (m mapResolver) ResolvePrefix(prefix []byte) ([]byte, bool) {
	ns, ok := m[string(prefix)]
	if !ok {
		return nil, false
	}
	return []byte(ns), true
}

func TestCanonicalQName(t *testing.T) {
	resolver := mapResolver{"": "urn:default", "p": "urn:pref"}
	got, err := CanonicalQName([]byte("p:local"), resolver, nil)
	if err != nil {
		t.Fatalf("CanonicalQName() error = %v", err)
	}
	want := append([]byte("urn:pref"), 0)
	want = append(want, []byte("local")...)
	if !bytes.Equal(got, want) {
		t.Fatalf("CanonicalQName(prefixed) = %q, want %q", string(got), string(want))
	}

	got, err = CanonicalQName([]byte("local"), resolver, nil)
	if err != nil {
		t.Fatalf("CanonicalQName() error = %v", err)
	}
	want = append([]byte("urn:default"), 0)
	want = append(want, []byte("local")...)
	if !bytes.Equal(got, want) {
		t.Fatalf("CanonicalQName(default) = %q, want %q", string(got), string(want))
	}
}

func TestCanonicalQNameNoDefault(t *testing.T) {
	resolver := mapResolver{"p": "urn:pref"}
	got, err := CanonicalQName([]byte("local"), resolver, nil)
	if err != nil {
		t.Fatalf("CanonicalQName() error = %v", err)
	}
	want := []byte{0}
	want = append(want, []byte("local")...)
	if !bytes.Equal(got, want) {
		t.Fatalf("CanonicalQName(no default) = %q, want %q", string(got), string(want))
	}
}

func TestCanonicalQNameMissingPrefix(t *testing.T) {
	resolver := mapResolver{"": "urn:default"}
	if _, err := CanonicalQName([]byte("p:local"), resolver, nil); err == nil {
		t.Fatalf("expected error for missing prefix")
	}
}

func TestCanonicalQNameInvalid(t *testing.T) {
	if _, err := CanonicalQName([]byte("bad name"), mapResolver{}, nil); err == nil {
		t.Fatalf("expected error for whitespace in QName")
	}
	if _, err := CanonicalQName([]byte("xmlns:local"), mapResolver{"xmlns": "urn:x"}, nil); err == nil {
		t.Fatalf("expected error for reserved xmlns prefix")
	}
}

func TestCanonicalQNameXMLPrefix(t *testing.T) {
	lexical := []byte("xml:lang")
	got, err := CanonicalQName(lexical, mapResolver{}, nil)
	if err != nil {
		t.Fatalf("CanonicalQName() error = %v", err)
	}
	want := append([]byte(xmlnames.XMLNamespace), 0)
	want = append(want, []byte("lang")...)
	if !bytes.Equal(got, want) {
		t.Fatalf("CanonicalQName(xml) = %q, want %q", string(got), string(want))
	}
}

func TestCanonicalQNameXMLPrefixRejectsWrongBinding(t *testing.T) {
	_, err := CanonicalQName([]byte("xml:lang"), mapResolver{"xml": "urn:wrong"}, nil)
	if err == nil {
		t.Fatalf("expected error for wrong xml namespace binding")
	}
}
