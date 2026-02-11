package source

import (
	"testing"
	"testing/fstest"
)

func TestPendingTrackingInvariantAfterDeferredFailureRetry(t *testing.T) {
	fs := fstest.MapFS{
		"a.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:cycle"
           xmlns:tns="urn:cycle"
           elementFormDefault="qualified">
  <xs:include schemaLocation="b.xsd"/>
  <xs:element name="a" type="xs:string"/>
</xs:schema>`),
		},
		"b.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:cycle"
           xmlns:tns="urn:cycle"
           xmlns:m="urn:missing"
           elementFormDefault="qualified">
  <xs:include schemaLocation="a.xsd"/>
  <xs:import namespace="urn:missing" schemaLocation="missing.xsd"/>
  <xs:element name="b" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{FS: fs})
	for i := range 2 {
		if _, err := loader.Load("a.xsd"); err == nil {
			t.Fatalf("expected load failure for missing import on iteration %d", i+1)
		}
		assertPendingTrackingInvariant(t, loader)
	}
}

func TestPendingTrackingInvariantAfterIncludeCycleSuccess(t *testing.T) {
	fs := fstest.MapFS{
		"a.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:cycle"
           xmlns:tns="urn:cycle"
           elementFormDefault="qualified">
  <xs:include schemaLocation="b.xsd"/>
  <xs:element name="a" type="xs:string"/>
</xs:schema>`),
		},
		"b.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:cycle"
           xmlns:tns="urn:cycle"
           elementFormDefault="qualified">
  <xs:include schemaLocation="a.xsd"/>
  <xs:element name="b" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{FS: fs})
	if _, err := loader.Load("a.xsd"); err != nil {
		t.Fatalf("Load(a.xsd) error = %v", err)
	}
	assertPendingTrackingInvariant(t, loader)
	assertNoPendingTracking(t, loader)

	if _, err := loader.Load("b.xsd"); err != nil {
		t.Fatalf("Load(b.xsd) error = %v", err)
	}
	assertPendingTrackingInvariant(t, loader)
	assertNoPendingTracking(t, loader)
}

func assertPendingTrackingInvariant(t *testing.T, loader *SchemaLoader) {
	t.Helper()
	if loader == nil {
		t.Fatalf("loader is nil")
	}

	expectedIncoming := make(map[loadKey]int)
	for sourceKey, entry := range loader.state.entries {
		if entry == nil {
			continue
		}
		if entry.pendingCount < 0 {
			t.Fatalf("entry %s pendingCount = %d, want >= 0", sourceKey.systemID, entry.pendingCount)
		}
		for _, directive := range entry.pendingDirectives {
			expectedIncoming[directive.targetKey]++
		}
	}

	for key, entry := range loader.state.entries {
		if entry == nil {
			continue
		}
		want := expectedIncoming[key]
		if entry.pendingCount != want {
			t.Fatalf("entry %s pendingCount = %d, want %d", key.systemID, entry.pendingCount, want)
		}
	}
}

func assertNoPendingTracking(t *testing.T, loader *SchemaLoader) {
	t.Helper()
	if loader == nil {
		t.Fatalf("loader is nil")
	}
	for key, entry := range loader.state.entries {
		if entry == nil {
			continue
		}
		if entry.pendingCount != 0 {
			t.Fatalf("entry %s pendingCount = %d, want 0", key.systemID, entry.pendingCount)
		}
		if len(entry.pendingDirectives) != 0 {
			t.Fatalf("entry %s pendingDirectives = %d, want 0", key.systemID, len(entry.pendingDirectives))
		}
	}
}
