package source

import (
	"testing"
	"testing/fstest"
)

func TestLoadReturnsSchema(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": {
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{FS: fsys})
	sch, err := loader.Load("schema.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if sch == nil {
		t.Fatalf("expected parsed schema")
	}
}
