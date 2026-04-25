package compiler

import (
	"testing"
	"testing/fstest"
)

func TestCanonicalFSSystemIDUsesDirectoryEntryCase(t *testing.T) {
	fsys := fstest.MapFS{
		"dir/schZ012_b.xsd": &fstest.MapFile{Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)},
	}

	got := canonicalFSSystemID(fsys, "dir/Schz012_b.xsd")
	if want := "dir/schZ012_b.xsd"; got != want {
		t.Fatalf("canonicalFSSystemID() = %q, want %q", got, want)
	}
}
