package loader

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

func TestLoader_Load(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if schema == nil {
		t.Fatal("Load() returned nil schema")
	}

	if schema.TargetNamespace != "http://example.com" {
		t.Errorf("TargetNamespace = %v, want %v", schema.TargetNamespace, "http://example.com")
	}
}

func TestLoader_CircularDependency(t *testing.T) {
	loader := NewLoader(Config{
		FS: fstest.MapFS{
			"test.xsd": &fstest.MapFile{
				Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`),
			},
		},
	})

	absLoc, err := loader.resolveLocation("test.xsd")
	if err != nil {
		t.Fatalf("resolveLocation error = %v", err)
	}
	key := loader.loadKey(loader.defaultFSContext(), absLoc)
	loader.state.loading[key] = true

	_, err = loader.Load("test.xsd")
	if err == nil {
		t.Error("Load() should return error for circular dependency")
	}

	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error should mention circular dependency, got: %v", err)
	}
}

// TestLoader_MutualImport checks that mutual imports across namespaces are allowed.
// It mirrors W3C particlesDa001 and should not count as circular.
func TestLoader_MutualImport(t *testing.T) {
	testFS := fstest.MapFS{
		"schemaA.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://namespaceA"
           xmlns:b="http://namespaceB"
           elementFormDefault="qualified">
  <xs:import namespace="http://namespaceB" schemaLocation="schemaB.xsd"/>
  <xs:element name="rootA" type="xs:string"/>
  <xs:complexType name="TypeA">
    <xs:sequence>
      <xs:element name="childA" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`),
		},
		"schemaB.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://namespaceB"
           xmlns:a="http://namespaceA"
           elementFormDefault="qualified">
  <xs:import namespace="http://namespaceA" schemaLocation="schemaA.xsd"/>
  <xs:element name="rootB" type="a:TypeA"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	// load schemaA - this should succeed even though schemaB imports schemaA back
	schema, err := loader.Load("schemaA.xsd")
	if err != nil {
		t.Fatalf("Load() should succeed for mutual imports between different namespaces, got error: %v", err)
	}

	if schema == nil {
		t.Fatal("Load() returned nil schema")
	}

	if schema.TargetNamespace != "http://namespaceA" {
		t.Errorf("TargetNamespace = %v, want %v", schema.TargetNamespace, "http://namespaceA")
	}

	// verify that TypeA was loaded
	typeAQName := types.QName{Namespace: "http://namespaceA", Local: "TypeA"}
	if _, ok := schema.TypeDefs[typeAQName]; !ok {
		t.Error("TypeA should be in schema.TypeDefs")
	}
}

func TestLoader_MutualImportSameBasename(t *testing.T) {
	testFS := fstest.MapFS{
		"a/common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:a"
           xmlns:b="urn:b"
           elementFormDefault="qualified">
  <xs:import namespace="urn:b" schemaLocation="../b/common.xsd"/>
  <xs:simpleType name="TypeA">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
</xs:schema>`),
		},
		"b/common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:b"
           xmlns:a="urn:a"
           elementFormDefault="qualified">
  <xs:import namespace="urn:a" schemaLocation="../a/common.xsd"/>
  <xs:simpleType name="TypeB">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("a/common.xsd")
	if err != nil {
		t.Fatalf("Load() should succeed for mutual imports with same basename, got error: %v", err)
	}

	typeAQName := types.QName{Namespace: "urn:a", Local: "TypeA"}
	if _, ok := schema.TypeDefs[typeAQName]; !ok {
		t.Errorf("type %s should be in schema.TypeDefs", typeAQName)
	}

	typeBQName := types.QName{Namespace: "urn:b", Local: "TypeB"}
	if _, ok := schema.TypeDefs[typeBQName]; !ok {
		t.Errorf("type %s should be in schema.TypeDefs", typeBQName)
	}
}

func TestLoadCompiled_LocalElementTypeResolution(t *testing.T) {
	testDir := filepath.Join("..", "..", "testdata", "xsdtests", "msData", "additional")
	loader := NewLoader(Config{
		FS: os.DirFS(testDir),
	})

	compiled, err := loader.LoadCompiled("test74834.xsd")
	if err != nil {
		t.Fatalf("LoadCompiled() error = %v", err)
	}

	typeQName := types.QName{Namespace: "urn:myxsdschema", Local: "mySmallDateTime"}
	ct := compiled.Types[typeQName]
	if ct == nil || ct.ContentModel == nil {
		t.Fatalf("compiled type %s not found", typeQName)
	}

	var timeElem *grammar.CompiledElement
	var findTimeElem func(particles []*grammar.CompiledParticle)
	findTimeElem = func(particles []*grammar.CompiledParticle) {
		for _, particle := range particles {
			switch particle.Kind {
			case grammar.ParticleElement:
				if particle.Element != nil && particle.Element.QName.Local == "time" {
					timeElem = particle.Element
					return
				}
			case grammar.ParticleGroup:
				findTimeElem(particle.Children)
				if timeElem != nil {
					return
				}
			}
		}
	}
	findTimeElem(ct.ContentModel.Particles)

	if timeElem == nil || timeElem.Type == nil {
		t.Fatal("time element declaration not found in mySmallDateTime content model")
	}

	expectedType := types.QName{Namespace: "urn:myxsdschema", Local: "mySmallTime"}
	if timeElem.Type.QName != expectedType {
		t.Fatalf("time element type = %s, want %s", timeElem.Type.QName, expectedType)
	}
}

// TestLoader_CircularInclude tests that circular includes (same namespace) are errors.
func TestLoader_CircularInclude(t *testing.T) {
	testFS := fstest.MapFS{
		"schemaA.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:include schemaLocation="schemaB.xsd"/>
  <xs:element name="elemA" type="xs:string"/>
</xs:schema>`),
		},
		"schemaB.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:include schemaLocation="schemaA.xsd"/>
  <xs:element name="elemB" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("schemaA.xsd")
	if err != nil {
		t.Fatalf("Load() should succeed for circular includes, got error: %v", err)
	}

	elemAQName := types.QName{Namespace: "http://example.com", Local: "elemA"}
	if _, ok := schema.ElementDecls[elemAQName]; !ok {
		t.Error("elemA should be in schema.ElementDecls")
	}

	elemBQName := types.QName{Namespace: "http://example.com", Local: "elemB"}
	if _, ok := schema.ElementDecls[elemBQName]; !ok {
		t.Error("elemB should be in schema.ElementDecls")
	}
}

func TestLoader_IncludeTraversalRejected(t *testing.T) {
	testFS := fstest.MapFS{
		"schemas/root.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           elementFormDefault="qualified">
  <xs:include schemaLocation="../outside.xsd"/>
</xs:schema>`),
		},
		"outside.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`),
		},
	}

	loader := NewLoader(Config{
		FS:       testFS,
		BasePath: "schemas",
	})

	_, err := loader.Load("root.xsd")
	if err == nil {
		t.Fatal("Load() should reject traversal outside base path")
	}
	if !strings.Contains(err.Error(), "escapes base path") {
		t.Fatalf("expected base path error, got: %v", err)
	}
}

func TestLoader_IncludeMissingIgnored(t *testing.T) {
	testFS := fstest.MapFS{
		"root.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="missing.xsd"/>
  <xs:include schemaLocation="included.xsd"/>
</xs:schema>`),
		},
		"included.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="ok" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("root.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	okQName := types.QName{Namespace: types.NamespaceEmpty, Local: "ok"}
	if _, ok := schema.ElementDecls[okQName]; !ok {
		t.Error("element 'ok' from included schema not found")
	}
}

func TestLoader_RestrictionAttributesIncludeBaseChain(t *testing.T) {
	testFS := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:attribute name="id" type="xs:ID"/>
  <xs:complexType name="Base">
    <xs:attribute ref="tns:id" use="optional"/>
  </xs:complexType>
  <xs:complexType name="Intermediate">
    <xs:complexContent>
      <xs:extension base="tns:Base"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="Restricted">
    <xs:complexContent>
      <xs:restriction base="tns:Intermediate">
        <xs:attribute ref="tns:id" use="required"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	if _, err := loader.Load("schema.xsd"); err != nil {
		t.Fatalf("Load() should succeed for restriction inheriting base attributes, got error: %v", err)
	}
}

func TestLoader_InlineTypeUsesImportContext(t *testing.T) {
	testFS := fstest.MapFS{
		"root.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:mid="urn:mid"
           targetNamespace="urn:root"
           elementFormDefault="qualified">
  <xs:import namespace="urn:mid" schemaLocation="mid.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`),
		},
		"mid.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:leaf="urn:leaf"
           targetNamespace="urn:mid"
           elementFormDefault="qualified">
  <xs:import namespace="urn:leaf" schemaLocation="leaf.xsd"/>
  <xs:element name="mid" type="xs:string"/>
</xs:schema>`),
		},
		"leaf.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:leaf="urn:leaf"
           targetNamespace="urn:leaf"
           elementFormDefault="qualified">
  <xs:element name="title" type="xs:string"/>
  <xs:element name="locator">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="leaf:title"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	if _, err := loader.Load("root.xsd"); err != nil {
		t.Fatalf("Load() should succeed with inline types referencing same-namespace elements, got error: %v", err)
	}
}

func TestLoader_IncludeDuplicateFromDifferentPaths(t *testing.T) {
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:include schemaLocation="a.xsd"/>
  <xs:include schemaLocation="b.xsd"/>
</xs:schema>`),
		},
		"a.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:include schemaLocation="common.xsd"/>
</xs:schema>`),
		},
		"b.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:include schemaLocation="common.xsd"/>
</xs:schema>`),
		},
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:simpleType name="T">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load() should succeed for repeated include of same schema, got error: %v", err)
	}

	typeQName := types.QName{Namespace: "http://example.com", Local: "T"}
	if _, ok := schema.TypeDefs[typeQName]; !ok {
		t.Errorf("type %s should be in schema.TypeDefs", typeQName)
	}
}

func TestLoader_Cache(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema1, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("First Load() error = %v", err)
	}

	schema2, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Second Load() error = %v", err)
	}

	if schema1 != schema2 {
		t.Error("Load() should return cached schema on second call")
	}
}

func TestLoader_FileNotFound(t *testing.T) {
	loader := NewLoader(Config{
		FS: fstest.MapFS{},
	})

	_, err := loader.Load("nonexistent.xsd")
	if err == nil {
		t.Error("Load() should return error for nonexistent file")
	}
}

func TestLoader_Include(t *testing.T) {
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`),
		},
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="common" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	rootQName := types.QName{
		Namespace: "http://example.com",
		Local:     "root",
	}
	if _, ok := schema.ElementDecls[rootQName]; !ok {
		t.Error("element 'root' not found in schema")
	}

	commonQName := types.QName{
		Namespace: "http://example.com",
		Local:     "common",
	}
	if _, ok := schema.ElementDecls[commonQName]; !ok {
		t.Error("element 'common' from included schema not found")
	}
}

func TestLoader_Import(t *testing.T) {
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/main"
           elementFormDefault="qualified">
  <xs:import namespace="http://example.com/common" schemaLocation="common.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`),
		},
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/common"
           elementFormDefault="qualified">
  <xs:element name="common" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	rootQName := types.QName{
		Namespace: "http://example.com/main",
		Local:     "root",
	}
	if _, ok := schema.ElementDecls[rootQName]; !ok {
		t.Error("element 'root' not found in schema")
	}

	commonQName := types.QName{
		Namespace: "http://example.com/common",
		Local:     "common",
	}
	if _, ok := schema.ElementDecls[commonQName]; !ok {
		t.Error("element 'common' from imported schema not found")
	}
}

func TestLoader_ImportNamespaceFS(t *testing.T) {
	mainFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:common="urn:common"
           targetNamespace="urn:main"
           elementFormDefault="qualified">
  <xs:import namespace="urn:common" schemaLocation="common.xsd"/>
  <xs:element name="root" type="common:CustomerType"/>
</xs:schema>`),
		},
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:common"
           elementFormDefault="qualified">
  <xs:complexType name="WrongType">
    <xs:sequence>
      <xs:element name="value" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`),
		},
	}

	commonFS := fstest.MapFS{
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:common"
           elementFormDefault="qualified">
  <xs:complexType name="CustomerType">
    <xs:sequence>
      <xs:element name="id" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: mainFS,
		NamespaceFS: map[string]fs.FS{
			"urn:common": commonFS,
		},
	})

	schema, err := loader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	loadedType := types.QName{Namespace: "urn:common", Local: "CustomerType"}
	if _, ok := schema.TypeDefs[loadedType]; !ok {
		t.Fatalf("expected imported type %s to be loaded", loadedType)
	}

	unexpectedType := types.QName{Namespace: "urn:common", Local: "WrongType"}
	if _, ok := schema.TypeDefs[unexpectedType]; ok {
		t.Fatalf("unexpected type %s from default FS", unexpectedType)
	}
}

func TestLoader_IncludeNamespaceMismatch(t *testing.T) {
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/main"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
</xs:schema>`),
		},
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/common"
           elementFormDefault="qualified">
  <xs:element name="common" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("main.xsd")
	if err == nil {
		t.Error("Load() should return error for namespace mismatch in include")
	}
	if !strings.Contains(err.Error(), "different target namespace") {
		t.Errorf("error should mention namespace mismatch, got: %v", err)
	}
}

func TestLoader_IncludeWithNoNamespaceIncluded(t *testing.T) {
	// including schema has namespace, included schema has no namespace - should succeed
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`),
		},
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:element name="common" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v (should allow include with no namespace)", err)
	}

	rootQName := types.QName{
		Namespace: "http://example.com",
		Local:     "root",
	}
	if _, ok := schema.ElementDecls[rootQName]; !ok {
		t.Error("element 'root' not found in schema")
	}

	// the included element should be in the including schema's namespace
	commonQName := types.QName{
		Namespace: "http://example.com",
		Local:     "common",
	}
	if _, ok := schema.ElementDecls[commonQName]; !ok {
		t.Error("element 'common' from included schema not found")
	}
}

func TestLoader_IncludeBothNoNamespace(t *testing.T) {
	// both schemas have no namespace - should succeed
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`),
		},
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:element name="common" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v (should allow include when both have no namespace)", err)
	}

	// check that both elements are present (both in no namespace)
	rootQName := types.QName{
		Namespace: "",
		Local:     "root",
	}
	if _, ok := schema.ElementDecls[rootQName]; !ok {
		t.Error("element 'root' not found in schema")
	}

	commonQName := types.QName{
		Namespace: "",
		Local:     "common",
	}
	if _, ok := schema.ElementDecls[commonQName]; !ok {
		t.Error("element 'common' from included schema not found")
	}
}

func TestSchemaValidation_ListTypeMissingItemType(t *testing.T) {
	// list type missing itemType attribute (invalid)
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:simpleType name="InvalidList">
    <xs:list/>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Load() should return error for list type missing itemType attribute")
	}
	if !strings.Contains(err.Error(), "list missing itemType") && !strings.Contains(err.Error(), "list type must have itemType") && !strings.Contains(err.Error(), "list must have either itemType") {
		t.Errorf("error should mention missing itemType, got: %v", err)
	}
}

func TestSchemaValidation_InvalidListType(t *testing.T) {
	// list type with itemType that is itself a list (invalid)
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:simpleType name="StringList">
    <xs:list itemType="xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="InvalidList">
    <xs:list itemType="tns:StringList"/>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Load() should return error for list type with non-atomic itemType")
	}
	if !strings.Contains(err.Error(), "list itemType must be atomic or union") {
		t.Errorf("error should mention list itemType must be atomic or union, got: %v", err)
	}
}

func TestSchemaValidation_ValidListTypeWithUnion(t *testing.T) {
	// list type with itemType that is a union (valid per XSD 1.0 spec)
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:simpleType name="IntOrString">
    <xs:union memberTypes="xs:int xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="IntOrStringList">
    <xs:list itemType="tns:IntOrString"/>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err != nil {
		t.Errorf("Load() should succeed for list type with union itemType, got error: %v", err)
	}
}

func TestSchemaValidation_ValidListTypeWithInlineSimpleType(t *testing.T) {
	// list type with inline simpleType child (valid per XSD 1.0 spec)
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:simpleType name="IntegerList">
    <xs:list>
      <xs:simpleType>
        <xs:restriction base="xs:integer"/>
      </xs:simpleType>
    </xs:list>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Errorf("Load() should succeed for list type with inline simpleType, got error: %v", err)
		return
	}

	// verify the list type was parsed correctly
	qname := types.QName{
		Namespace: "http://example.com",
		Local:     "IntegerList",
	}
	listType, ok := schema.TypeDefs[qname]
	if !ok {
		t.Fatal("IntegerList type not found in schema")
	}

	st, ok := listType.(*types.SimpleType)
	if !ok {
		t.Fatal("IntegerList is not a SimpleType")
	}

	if st.List == nil {
		t.Fatal("IntegerList should have List definition")
	}

	// for inline simpleType, InlineItemType should be set and List.ItemType should be zero
	if st.List.InlineItemType == nil {
		t.Error("List.InlineItemType should be set for inline simpleType")
	}
	if !st.List.ItemType.IsZero() {
		t.Error("List.ItemType should be zero for inline simpleType")
	}
	// ItemType should also be set for validator access
	if st.ItemType == nil {
		t.Error("ItemType should be set for inline simpleType (for validator access)")
	}
}

func TestSchemaValidation_InvalidListTypeWithBothItemTypeAndInlineSimpleType(t *testing.T) {
	// list type cannot have both itemType attribute and inline simpleType child (invalid per XSD spec)
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:simpleType name="InvalidList">
    <xs:list itemType="xs:string">
      <xs:simpleType>
        <xs:restriction base="xs:integer"/>
      </xs:simpleType>
    </xs:list>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Load() should return error for list type with both itemType attribute and inline simpleType")
	}
	if !strings.Contains(err.Error(), "list cannot have both itemType") {
		t.Errorf("error should mention both itemType and inline simpleType, got: %v", err)
	}
}

func TestSchemaValidation_InvalidUnionType(t *testing.T) {
	// union type with no member types (invalid)
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:simpleType name="InvalidUnion">
    <xs:union/>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Load() should return error for union type with no member types")
	}
	if !strings.Contains(err.Error(), "union simpleType must declare member types") {
		t.Errorf("error should mention union must have member types, got: %v", err)
	}
}

func TestSchemaValidation_UnionWithInlineTypes(t *testing.T) {
	// union type with inline simpleType children (valid)
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:simpleType name="StringOrInteger">
    <xs:union>
      <xs:simpleType>
        <xs:restriction base="xs:string">
          <xs:maxLength value="10"/>
        </xs:restriction>
      </xs:simpleType>
      <xs:simpleType>
        <xs:restriction base="xs:integer">
          <xs:minInclusive value="0"/>
        </xs:restriction>
      </xs:simpleType>
    </xs:union>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// verify the union type was parsed correctly
	qname := types.QName{
		Namespace: "http://example.com",
		Local:     "StringOrInteger",
	}
	st, ok := schema.TypeDefs[qname].(*types.SimpleType)
	if !ok {
		t.Fatalf("Type %v should be a SimpleType", qname)
	}

	if st.Union == nil {
		t.Fatal("Union should not be nil")
	}

	if len(st.Union.InlineTypes) != 2 {
		t.Errorf("Union should have 2 inline types, got %d", len(st.Union.InlineTypes))
	}

	if len(st.Union.MemberTypes) != 0 {
		t.Errorf("Union should have 0 memberTypes (QNames), got %d", len(st.Union.MemberTypes))
	}

	// verify inline types are parsed correctly
	if st.Union.InlineTypes[0].Variety() != types.AtomicVariety {
		t.Errorf("First inline type should be atomic, got %v", st.Union.InlineTypes[0].Variety())
	}
	if st.Union.InlineTypes[1].Variety() != types.AtomicVariety {
		t.Errorf("Second inline type should be atomic, got %v", st.Union.InlineTypes[1].Variety())
	}
}

func TestSchemaValidation_InvalidFacetCombination_LengthWithMinMax(t *testing.T) {
	// length facet cannot be used with minLength or maxLength
	// per XSD 1.0 Errata E1-17, they are mutually exclusive regardless of derivation step
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:simpleType name="InvalidType">
    <xs:restriction base="xs:string">
      <xs:length value="5"/>
      <xs:minLength value="3"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Load() should return error for length facet with minLength")
	}
	if !strings.Contains(err.Error(), "length facet cannot be used together with minLength or maxLength") {
		t.Errorf("error should mention length/minLength conflict, got: %v", err)
	}
}

func TestSchemaValidation_InvalidFacetCombination_MinLengthGreaterThanMaxLength(t *testing.T) {
	// minLength must be <= maxLength
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:simpleType name="InvalidType">
    <xs:restriction base="xs:string">
      <xs:minLength value="10"/>
      <xs:maxLength value="5"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Load() should return error for minLength > maxLength")
	}
	if !strings.Contains(err.Error(), "minLength") || !strings.Contains(err.Error(), "maxLength") {
		t.Errorf("error should mention minLength/maxLength conflict, got: %v", err)
	}
}

func TestSchemaValidation_InvalidRangeFacet_OnNonOrderedType(t *testing.T) {
	// range facets (minInclusive, etc.) are only applicable to ordered types
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:simpleType name="InvalidType">
    <xs:restriction base="xs:boolean">
      <xs:minInclusive value="true"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Load() should return error for range facet on non-ordered type")
		return
	}
	if !strings.Contains(err.Error(), "only applicable to ordered types") && !strings.Contains(err.Error(), "no parser available") {
		t.Errorf("error should mention ordered types or unsupported type, got: %v", err)
	}
}

func TestSchemaValidation_InvalidRangeFacet_ConflictingValues(t *testing.T) {
	// minInclusive > maxInclusive is invalid
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com">
  <xs:simpleType name="InvalidType">
    <xs:restriction base="xs:integer">
      <xs:minInclusive value="10"/>
      <xs:maxInclusive value="5"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Load() should return error for minInclusive > maxInclusive")
	}
	if !strings.Contains(err.Error(), "minInclusive") || !strings.Contains(err.Error(), "maxInclusive") {
		t.Errorf("error should mention minInclusive/maxInclusive conflict, got: %v", err)
	}
}

func TestLoader_ImportNoTargetNamespaceWithoutNamespaceAttr(t *testing.T) {
	// schema without targetNamespace cannot use import without namespace attribute
	// per XSD 1.0 spec constraint: "if namespace is absent, schema must have targetNamespace"
	// by contrapositive: if schema has no targetNamespace, namespace attribute must be present
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:import schemaLocation="common.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`),
		},
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:element name="common" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("main.xsd")
	if err == nil {
		t.Error("Load() should return error for import without namespace attribute in no-targetNamespace schema")
	}
	if !strings.Contains(err.Error(), "namespace attribute") {
		t.Errorf("error should mention namespace attribute, got: %v", err)
	}
}

func TestLoader_ImportNoTargetNamespaceWithNamespaceAttr(t *testing.T) {
	// schema without targetNamespace can use import WITH namespace attribute (required)
	// per XSD 1.0 spec constraint: "if namespace is absent, schema must have targetNamespace"
	// by contrapositive: if schema has no targetNamespace, namespace attribute must be present
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified">
  <xs:import namespace="http://example.com/common" schemaLocation="common.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`),
		},
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/common"
           elementFormDefault="qualified">
  <xs:element name="common" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v (should allow import with namespace attribute for no-targetNamespace schema)", err)
	}

	rootQName := types.QName{
		Namespace: "",
		Local:     "root",
	}
	if _, ok := schema.ElementDecls[rootQName]; !ok {
		t.Error("element 'root' not found in schema")
	}

	commonQName := types.QName{
		Namespace: "http://example.com/common",
		Local:     "common",
	}
	if _, ok := schema.ElementDecls[commonQName]; !ok {
		t.Error("element 'common' from imported schema not found")
	}
}

func TestLoader_LocalUntypedAttribute(t *testing.T) {
	// test that local untyped attributes (attributes with name but no type) are accepted.
	// per XSD spec, local untyped attributes implicitly have type xs:anySimpleType
	// and are always in the empty namespace (not target namespace).
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="untypedAttr"/>
      <xs:attribute name="typedAttr" type="xs:string"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v (local untyped attributes should be valid)", err)
	}

	// verify the element exists
	rootQName := types.QName{
		Namespace: "http://example.com",
		Local:     "root",
	}
	rootElem, ok := schema.ElementDecls[rootQName]
	if !ok {
		t.Fatal("element 'root' not found in schema")
	}

	// verify the complex type has both attributes
	ct, ok := rootElem.Type.(*types.ComplexType)
	if !ok {
		t.Fatal("root element type is not a ComplexType")
	}

	attrs := ct.Attributes()
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(attrs))
	}

	var untypedAttr *types.AttributeDecl
	var typedAttr *types.AttributeDecl
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "untypedAttr":
			untypedAttr = attr
		case "typedAttr":
			typedAttr = attr
		}
	}

	if untypedAttr == nil {
		t.Fatal("untypedAttr not found")
	}
	if typedAttr == nil {
		t.Fatal("typedAttr not found")
	}

	// verify untyped attribute has Type=nil and empty namespace
	if untypedAttr.Type != nil {
		t.Errorf("untypedAttr should have Type=nil, got %v", untypedAttr.Type)
	}
	if untypedAttr.Name.Namespace != "" {
		t.Errorf("untypedAttr should have empty namespace (local attributes are always unqualified), got %q", untypedAttr.Name.Namespace)
	}

	// verify typed attribute has a type
	if typedAttr.Type == nil {
		t.Error("typedAttr should have a type")
	}
}

func TestLoader_AttributeReference(t *testing.T) {
	// test that attribute references (ref attribute) are validated correctly.
	// an attribute reference should exist in the schema's AttributeDecls.
	// note: Unprefixed attribute references resolve to empty namespace, which makes it
	// impossible to distinguish them from local untyped attributes after parsing.
	// so we test with a prefixed reference that resolves to target namespace.
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:attribute name="globalAttr" type="xs:string"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute ref="tns:globalAttr"/>
      <xs:attribute ref="tns:nonexistentAttr"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Fatal("Load() should return error for nonexistent attribute reference")
	}
	if !strings.Contains(err.Error(), "attribute reference") || !strings.Contains(err.Error(), "nonexistentAttr") {
		t.Errorf("error should mention nonexistent attribute reference, got: %v", err)
	}
}

func TestLoader_LocalUntypedAttributeInType(t *testing.T) {
	// test that local untyped attributes are accepted in named complex types.
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:complexType name="TestType">
    <xs:attribute name="untypedAttr"/>
  </xs:complexType>
  <xs:element name="root" type="tns:TestType"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v (local untyped attributes in named types should be valid)", err)
	}

	// verify the type exists and has the untyped attribute
	typeQName := types.QName{
		Namespace: "http://example.com",
		Local:     "TestType",
	}
	typ, ok := schema.TypeDefs[typeQName]
	if !ok {
		t.Fatal("type 'TestType' not found in schema")
	}

	ct, ok := typ.(*types.ComplexType)
	if !ok {
		t.Fatal("TestType is not a ComplexType")
	}

	attrs := ct.Attributes()
	if len(attrs) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(attrs))
	}

	attr := attrs[0]
	if attr.Name.Local != "untypedAttr" {
		t.Errorf("expected attribute 'untypedAttr', got %q", attr.Name.Local)
	}
	if attr.Type != nil {
		t.Errorf("untypedAttr should have Type=nil, got %v", attr.Type)
	}
	if attr.Name.Namespace != "" {
		t.Errorf("untypedAttr should have empty namespace, got %q", attr.Name.Namespace)
	}
}
