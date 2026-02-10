package source

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/model"
)

func TestPendingMergesRemainConsistentOnFailure(t *testing.T) {
	rootSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           xmlns:tns="urn:root"
           xmlns:c="urn:c"
           elementFormDefault="qualified">
  <xs:include schemaLocation="b.xsd"/>
  <xs:import namespace="urn:c" schemaLocation="c.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	includeSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           elementFormDefault="qualified">
  <xs:element name="fromB" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"a.xsd": &fstest.MapFile{Data: []byte(rootSchema)},
		"b.xsd": &fstest.MapFile{Data: []byte(includeSchema)},
	}
	loader := NewLoader(Config{FS: fs})
	if _, err := loader.Load("a.xsd"); err == nil {
		t.Fatalf("expected load error for missing c.xsd")
	}

	rootKey := loader.loadKey("a.xsd", model.NamespaceURI("urn:root"))
	if entry, ok := loader.state.entry(rootKey); ok && entry.schema != nil {
		if _, ok := entry.schema.ElementDecls[model.QName{Namespace: "urn:root", Local: "fromB"}]; ok {
			t.Fatalf("include merge should not be committed on failure")
		}
		if entry.pendingCount != 0 || len(entry.pendingDirectives) != 0 {
			t.Fatalf("pending directives not cleared after failure")
		}
	}
}
