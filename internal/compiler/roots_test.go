package compiler_test

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/compiler"
)

func TestPrepareRootsRejectsConflictingInputs(t *testing.T) {
	t.Parallel()

	_, err := compiler.PrepareRoots(compiler.LoadConfig{
		Roots:    []compiler.Root{{FS: fstest.MapFS{}, Location: "schema.xsd"}},
		FS:       fstest.MapFS{},
		Location: "schema.xsd",
	})
	if err == nil {
		t.Fatal("PrepareRoots() expected error for conflicting root inputs")
	}
}

func TestPrepareRootsBuildsRuntime(t *testing.T) {
	t.Parallel()

	prepared, err := compiler.PrepareRoots(compiler.LoadConfig{
		FS: fstest.MapFS{
			"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
		},
		Location: "schema.xsd",
	})
	if err != nil {
		t.Fatalf("PrepareRoots() error = %v", err)
	}

	rt, err := prepared.Build(compiler.BuildConfig{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if rt == nil {
		t.Fatal("Build() returned nil runtime schema")
	}
}
