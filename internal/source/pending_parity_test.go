package source

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestImmediateVsDeferredDirectiveParity(t *testing.T) {
	immediateFS := fstest.MapFS{
		"root_immediate.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           xmlns:tns="urn:root"
           xmlns:c="urn:c"
           elementFormDefault="qualified">
  <xs:include schemaLocation="shared_immediate.xsd"/>
  <xs:import namespace="urn:c" schemaLocation="c_immediate.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`),
		},
		"shared_immediate.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           elementFormDefault="qualified">
  <xs:element name="shared" type="xs:string"/>
</xs:schema>`),
		},
		"c_immediate.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:c"
           xmlns:c="urn:c"
           elementFormDefault="qualified">
  <xs:element name="c" type="xs:string"/>
</xs:schema>`),
		},
	}

	deferredFS := fstest.MapFS{
		"root_deferred.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           xmlns:tns="urn:root"
           xmlns:c="urn:c"
           elementFormDefault="qualified">
  <xs:include schemaLocation="mid_deferred.xsd"/>
  <xs:import namespace="urn:c" schemaLocation="c_deferred.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`),
		},
		"mid_deferred.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           elementFormDefault="qualified">
  <xs:include schemaLocation="root_deferred.xsd"/>
  <xs:include schemaLocation="shared_deferred.xsd"/>
</xs:schema>`),
		},
		"shared_deferred.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           elementFormDefault="qualified">
  <xs:element name="shared" type="xs:string"/>
</xs:schema>`),
		},
		"c_deferred.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:c"
           xmlns:c="urn:c"
           xmlns:r="urn:root"
           elementFormDefault="qualified">
  <xs:import namespace="urn:root" schemaLocation="root_deferred.xsd"/>
  <xs:element name="c" type="xs:string"/>
</xs:schema>`),
		},
	}

	immediateLoader := NewLoader(Config{FS: immediateFS})
	immediateSchema, err := immediateLoader.Load("root_immediate.xsd")
	if err != nil {
		t.Fatalf("Load(root_immediate.xsd) error = %v", err)
	}
	assertPendingTrackingInvariant(t, immediateLoader)
	assertNoPendingTracking(t, immediateLoader)

	deferredLoader := NewLoader(Config{FS: deferredFS})
	deferredSchema, err := deferredLoader.Load("root_deferred.xsd")
	if err != nil {
		t.Fatalf("Load(root_deferred.xsd) error = %v", err)
	}
	assertPendingTrackingInvariant(t, deferredLoader)
	assertNoPendingTracking(t, deferredLoader)

	immediateElems := elementNameSet(immediateSchema)
	deferredElems := elementNameSet(deferredSchema)
	if len(immediateElems) != len(deferredElems) {
		t.Fatalf("element set size immediate=%d deferred=%d", len(immediateElems), len(deferredElems))
	}
	for name := range immediateElems {
		if _, ok := deferredElems[name]; !ok {
			t.Fatalf("deferred schema missing element %s", name)
		}
	}
}

func elementNameSet(schema *parser.Schema) map[model.QName]struct{} {
	if schema == nil {
		return nil
	}
	out := make(map[model.QName]struct{}, len(schema.ElementDecls))
	for name := range schema.ElementDecls {
		out[name] = struct{}{}
	}
	return out
}
