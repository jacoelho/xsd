package set

import (
	"slices"
	"testing"
	"testing/fstest"
)

func TestSetPrepareBuildRuntime(t *testing.T) {
	set := NewSet()
	err := set.Prepare(PrepareConfig{
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
		t.Fatalf("Prepare() error = %v", err)
	}
	if !set.IsPrepared() {
		t.Fatal("IsPrepared() = false, want true")
	}

	order := slices.Collect(set.GlobalElementOrderSeq())
	if len(order) != 1 || order[0].Local != "root" {
		t.Fatalf("GlobalElementOrderSeq() = %v, want root", order)
	}

	rt, err := set.BuildRuntime(CompileConfig{})
	if err != nil {
		t.Fatalf("BuildRuntime() error = %v", err)
	}
	if rt == nil {
		t.Fatal("BuildRuntime() returned nil")
	}
}

func TestSetBuildRuntimeWithoutPrepare(t *testing.T) {
	set := NewSet()
	if _, err := set.BuildRuntime(CompileConfig{}); err == nil {
		t.Fatal("BuildRuntime() error = nil, want error")
	}
}
