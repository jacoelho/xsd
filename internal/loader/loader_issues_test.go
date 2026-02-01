package loader

import (
	"testing"
	"testing/fstest"
)

func TestLoadInvalidSchemaDoesNotCache(t *testing.T) {
	badSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:unknownType"/>
</xs:schema>`
	goodSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(badSchema)},
	}
	loader := NewLoader(Config{FS: fs})
	if _, err := loader.Load("schema.xsd"); err == nil {
		t.Fatalf("expected schema validation error")
	}

	fs["schema.xsd"] = &fstest.MapFile{Data: []byte(goodSchema)}
	if _, err := loader.Load("schema.xsd"); err != nil {
		t.Fatalf("expected reload to succeed, got %v", err)
	}
}

func TestAllowMissingImportLocationsSkipsResolve(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           elementFormDefault="qualified"
           xmlns:other="urn:other">
  <xs:import namespace="urn:other"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"root.xsd": &fstest.MapFile{Data: []byte(schema)},
	}
	loader := NewLoader(Config{
		FS:                          fs,
		AllowMissingImportLocations: true,
	})
	if _, err := loader.Load("root.xsd"); err != nil {
		t.Fatalf("expected load success, got %v", err)
	}
}
