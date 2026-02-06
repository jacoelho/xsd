package source

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestLoadParsedReturnsParsedPhase(t *testing.T) {
	fsys := fstest.MapFS{
		"schema.xsd": {
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{FS: fsys})
	sch, err := loader.LoadParsed("schema.xsd")
	if err != nil {
		t.Fatalf("LoadParsed() error = %v", err)
	}
	if sch.Phase != parser.PhaseParsed {
		t.Fatalf("schema phase = %s, want %s", sch.Phase, parser.PhaseParsed)
	}
}
