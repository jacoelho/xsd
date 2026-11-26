package uriref

import (
	"errors"
	"testing"
)

func FuzzResolveAdmittedFailsOnlyOpaqueBase(f *testing.F) {
	for _, base := range []string{"http://h/a/b.xsd", "file:///tmp/a.xsd", "urn:opaque", "a/b", "http://h:80/"} {
		for _, ref := range []string{"http://h:abc/x.xsd", "http://a%41b/x.xsd", "http://h:abc/", "b.xsd", "../c.xsd", "./d/./e", "?q", "#f", "//", "/..//x", "x:y"} {
			f.Add(base, ref)
		}
	}
	f.Fuzz(func(t *testing.T, rawBase, rawRef string) {
		base, err := Parse(rawBase)
		if err != nil {
			return
		}
		ref, err := Parse(rawRef)
		if err != nil {
			return
		}
		if _, err := Resolve(base, ref); err != nil && !errors.Is(err, ErrOpaqueBase) {
			t.Fatalf("Resolve(%q, %q) error = %v, want nil or ErrOpaqueBase", rawBase, rawRef, err)
		}
	})
}
