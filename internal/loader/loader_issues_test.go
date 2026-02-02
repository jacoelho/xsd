package loader

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/types"
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

func TestLoadRollbackClearsPendingAndMerges(t *testing.T) {
	rootSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           xmlns:tns="urn:root"
           xmlns:c="urn:c"
           xmlns:d="urn:d"
           elementFormDefault="qualified">
  <xs:include schemaLocation="b.xsd"/>
  <xs:import namespace="urn:c" schemaLocation="c.xsd"/>
  <xs:import namespace="urn:d" schemaLocation="d.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`
	includeSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           xmlns:tns="urn:root"
           elementFormDefault="qualified">
  <xs:include schemaLocation="a.xsd"/>
  <xs:element name="fromB" type="xs:string"/>
</xs:schema>`
	importSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:c"
           xmlns:c="urn:c"
           elementFormDefault="qualified">
  <xs:element name="fromC" type="xs:string"/>
</xs:schema>`
	fixedImport := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:d"
           xmlns:d="urn:d"
           elementFormDefault="qualified">
  <xs:element name="fromD" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"a.xsd": &fstest.MapFile{Data: []byte(rootSchema)},
		"b.xsd": &fstest.MapFile{Data: []byte(includeSchema)},
		"c.xsd": &fstest.MapFile{Data: []byte(importSchema)},
	}
	loader := NewLoader(Config{FS: fs})
	if _, err := loader.Load("a.xsd"); err == nil {
		t.Fatalf("expected load error for missing d.xsd")
	}

	rootKey := loader.loadKey("a.xsd", types.NamespaceURI("urn:root"))
	includeKey := loader.loadKey("b.xsd", types.NamespaceURI("urn:root"))
	importKey := loader.loadKey("c.xsd", types.NamespaceURI("urn:c"))

	if loader.imports.alreadyMergedInclude(rootKey, includeKey) {
		t.Fatalf("include merge should be rolled back")
	}
	if loader.imports.alreadyMergedImport(rootKey, importKey) {
		t.Fatalf("import merge should be rolled back")
	}

	if entry, ok := loader.state.entry(rootKey); ok {
		if entry.pendingCount != 0 || len(entry.pendingDirectives) != 0 {
			t.Fatalf("root pending state not cleared: count=%d directives=%d", entry.pendingCount, len(entry.pendingDirectives))
		}
	}
	if entry, ok := loader.state.entry(includeKey); ok {
		if entry.pendingCount != 0 || len(entry.pendingDirectives) != 0 {
			t.Fatalf("include pending state not cleared: count=%d directives=%d", entry.pendingCount, len(entry.pendingDirectives))
		}
	}

	fs["d.xsd"] = &fstest.MapFile{Data: []byte(fixedImport)}
	schema, err := loader.Load("a.xsd")
	if err != nil {
		t.Fatalf("expected reload to succeed, got %v", err)
	}
	if _, ok := schema.ElementDecls[types.QName{Namespace: "urn:root", Local: "fromB"}]; !ok {
		t.Fatalf("expected included declaration from b.xsd to be present")
	}
	if _, ok := schema.ElementDecls[types.QName{Namespace: "urn:c", Local: "fromC"}]; !ok {
		t.Fatalf("expected imported declaration from c.xsd to be present")
	}
}
