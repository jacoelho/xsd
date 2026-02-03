package loader

import (
	"io"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtimebuild"
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

func TestMergeSchemaRollbackOnError(t *testing.T) {
	loader := &SchemaLoader{}
	target := parser.NewSchema()
	target.TargetNamespace = "urn:root"

	dupQName := types.QName{Namespace: target.TargetNamespace, Local: "Dup"}
	target.TypeDefs[dupQName] = &types.SimpleType{QName: dupQName}
	target.TypeOrigins[dupQName] = "target"

	source := parser.NewSchema()
	source.TargetNamespace = "urn:root"
	source.ElementDecls[types.QName{Namespace: source.TargetNamespace, Local: "Added"}] = &types.ElementDecl{
		Name:            types.QName{Namespace: source.TargetNamespace, Local: "Added"},
		SourceNamespace: source.TargetNamespace,
	}
	source.ElementOrigins[types.QName{Namespace: source.TargetNamespace, Local: "Added"}] = "source"
	source.TypeDefs[dupQName] = &types.SimpleType{QName: dupQName}
	source.TypeOrigins[dupQName] = "source"
	source.ImportedNamespaces[source.TargetNamespace] = map[types.NamespaceURI]bool{
		types.NamespaceURI("urn:other"): true,
	}
	source.ImportContexts["loc"] = parser.ImportContext{
		TargetNamespace: source.TargetNamespace,
		Imports: map[types.NamespaceURI]bool{
			types.NamespaceURI("urn:other"): true,
		},
	}

	err := loader.mergeSchema(target, source, mergeInclude, keepNamespace, len(target.GlobalDecls))
	if err == nil {
		t.Fatalf("expected merge error for duplicate type")
	}

	if len(target.ImportedNamespaces) != 0 {
		t.Fatalf("ImportedNamespaces mutated on failed merge")
	}
	if len(target.ImportContexts) != 0 {
		t.Fatalf("ImportContexts mutated on failed merge")
	}
	if _, ok := target.ElementDecls[types.QName{Namespace: target.TargetNamespace, Local: "Added"}]; ok {
		t.Fatalf("element declaration inserted on failed merge")
	}
	if len(target.GlobalDecls) != 0 {
		t.Fatalf("GlobalDecls mutated on failed merge")
	}
	if len(target.TypeDefs) != 1 {
		t.Fatalf("TypeDefs size = %d, want 1", len(target.TypeDefs))
	}
}

func TestLoadResolvedClosesDocOnPendingResolveError(t *testing.T) {
	loader := &SchemaLoader{
		state:   newLoadState(),
		imports: newImportTracker(),
	}
	key := loader.loadKey("schema.xsd", types.NamespaceURI("urn:test"))
	entry := loader.state.ensureEntry(key)
	entry.state = schemaStateLoaded
	entry.schema = parser.NewSchema()
	entry.pendingDirectives = []pendingDirective{{
		kind:      parser.DirectiveImport,
		targetKey: loadKey{systemID: "missing.xsd", etn: types.NamespaceURI("urn:missing")},
	}}

	doc := &trackingReadCloser{}
	if _, err := loader.loadResolved(doc, "schema.xsd", key, validateSchema); err == nil {
		t.Fatalf("expected pending resolve error")
	}
	if !doc.closed {
		t.Fatalf("expected doc to be closed")
	}
}

func TestSubstitutionGroupOrderDeterministic(t *testing.T) {
	rootSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:include schemaLocation="sub.xsd"/>
  <xs:element name="head" type="xs:string"/>
  <xs:element name="b" substitutionGroup="tns:head" type="xs:string"/>
</xs:schema>`
	subSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="a" substitutionGroup="tns:head" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"root.xsd": &fstest.MapFile{Data: []byte(rootSchema)},
		"sub.xsd":  &fstest.MapFile{Data: []byte(subSchema)},
	}
	head := types.QName{Namespace: "urn:test", Local: "head"}
	expected := []types.QName{
		{Namespace: "urn:test", Local: "a"},
		{Namespace: "urn:test", Local: "b"},
	}

	var prevHash uint64
	for i := range 5 {
		loader := NewLoader(Config{FS: fs})
		schema, err := loader.Load("root.xsd")
		if err != nil {
			t.Fatalf("load root: %v", err)
		}
		members := schema.SubstitutionGroups[head]
		if !equalQNameSlices(members, expected) {
			t.Fatalf("substitution group members = %v, want %v", members, expected)
		}
		rt, err := runtimebuild.BuildSchema(schema, runtimebuild.BuildConfig{})
		if err != nil {
			t.Fatalf("runtime build: %v", err)
		}
		if i == 0 {
			prevHash = rt.BuildHash
			continue
		}
		if rt.BuildHash != prevHash {
			t.Fatalf("build hash = %d, want %d", rt.BuildHash, prevHash)
		}
	}
}

func equalQNameSlices(a, b []types.QName) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(b[i]) {
			return false
		}
	}
	return true
}

type trackingReadCloser struct {
	closed bool
}

func (t *trackingReadCloser) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return nil
}
