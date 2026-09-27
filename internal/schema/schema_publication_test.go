package schema

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestPublishSchemaRejectsRawCorruptionWithoutMutation(t *testing.T) {
	badName := QName{Local: 1}
	build := schemaBuild{
		GlobalElements: map[QName]ElementID{badName: 0},
		Elements:       []ElementDecl{{Name: badName}},
	}
	want := schemaBuild{
		GlobalElements: map[QName]ElementID{badName: 0},
		Elements:       []ElementDecl{{Name: badName}},
	}

	_, err := publishSchema(&build, unlimitedContentModelWork)
	if err == nil {
		t.Fatal("PublishSchema() succeeded for invalid name references")
	}
	if !reflect.DeepEqual(build, want) {
		t.Fatalf("PublishSchema() mutated failed build: got %#v want %#v", build, want)
	}
}

func TestPublishSchemaRejectsEmptyComplexTypeModelMismatchWithoutConsumingBuild(t *testing.T) {
	t.Parallel()

	const document = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType/></xs:element>
</xs:schema>`
	tests := []struct {
		name   string
		mutate func(*ContentModel)
	}{
		{
			name: "required choice without particles",
			mutate: func(model *ContentModel) {
				model.Kind = ModelChoice
				model.Occurs = Occurrence{Min: 1, Max: 1}
			},
		},
		{
			name: "mixed empty model",
			mutate: func(model *ContentModel) {
				model.Mixed = true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, modelID := newEmptyComplexTypePublicationCompiler(t, document)
			validModel := c.rt.Models[modelID]
			if validModel.Kind != ModelEmpty || validModel.Mixed {
				t.Fatalf("compiled source model = %#v, want non-mixed ModelEmpty", validModel)
			}
			tt.mutate(&c.rt.Models[modelID])
			if err := c.compileContentModels(); err != nil {
				t.Fatalf("compileContentModels(corrupt) error = %v", err)
			}

			expected, expectedModelID := newEmptyComplexTypePublicationCompiler(t, document)
			tt.mutate(&expected.rt.Models[expectedModelID])
			if err := expected.compileContentModels(); err != nil {
				t.Fatalf("compileContentModels(expected) error = %v", err)
			}

			published, err := c.publishSchema()
			expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if published != nil {
				t.Fatal("publishSchema() returned a schema for an empty content model mismatch")
			}
			if !reflect.DeepEqual(c.rt, expected.rt) {
				t.Fatalf("publishSchema() changed failed build: got %#v want %#v", c.rt, expected.rt)
			}

			c.rt.Models[modelID] = validModel
			if err := c.compileContentModels(); err != nil {
				t.Fatalf("compileContentModels(restored) error = %v", err)
			}
			if published, err := c.publishSchema(); err != nil || published == nil {
				t.Fatalf("publishSchema() retry = %v/%v, want success", published, err)
			}
		})
	}
}

func newEmptyComplexTypePublicationCompiler(t *testing.T, document string) (*compiler, ContentModelID) {
	t.Helper()
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatalf("NormalizeOptions() error = %v", err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatalf("newCompiler() error = %v", err)
	}
	if err := c.loadOwned([]source.Source{source.Bytes("schema.xsd", []byte(document))}); err != nil {
		t.Fatalf("loadOwned() error = %v", err)
	}
	if err := c.index(); err != nil {
		t.Fatalf("index() error = %v", err)
	}
	if err := c.reserveIndexedComponentStorage(); err != nil {
		t.Fatalf("reserveIndexedComponentStorage() error = %v", err)
	}
	if err := c.compileGlobals(); err != nil {
		t.Fatalf("compileGlobals() error = %v", err)
	}
	rootName, ok := c.rt.Names.LookupQName("", "root")
	if !ok {
		t.Fatal("root QName not interned")
	}
	rootID, ok := c.rt.GlobalElements[rootName]
	if !ok {
		t.Fatal("root element not registered")
	}
	typ, ok := c.rt.Elements[rootID].Type.Complex()
	if !ok {
		t.Fatal("root element does not use a complex type")
	}
	return c, c.rt.ComplexTypes[typ].Content
}

func TestPublishedSchemaExcludesCompilerSources(t *testing.T) {
	t.Parallel()

	// A QName enumeration forces compilation to create both namespace replay
	// and notation callbacks in the temporary facet literal. The published
	// value program owns the resulting value; those callbacks must not remain
	// reachable through another schema projection.
	const document = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:test">
  <xs:simpleType name="QNameEnum">
    <xs:restriction base="xs:QName"><xs:enumeration value="t:item"/></xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="QNameEnum"/>
</xs:schema>`
	published, err := Compile(Options{}, []source.Source{source.Bytes("schema.xsd", []byte(document))})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := auditPublishedSchemaOwnership(published); err != nil {
		t.Fatal(err)
	}
}

var (
	schemaPackagePath = reflect.TypeFor[Schema]().PkgPath()
	sourcePackagePath = reflect.TypeFor[source.Source]().PkgPath()
	valuePackagePath  = reflect.TypeFor[valuepkg.TypeSpec]().PkgPath()
)

type publishedSchemaVisit struct {
	typ reflect.Type
	ptr uintptr
}

// auditPublishedSchemaOwnership walks the values reachable from Schema. It
// checks ownership by type and value category rather than relying on the
// private field layout of the published program. Compiler records, source
// documents, and live callbacks are construction-time state and must not be
// retained by a sealed schema.
func auditPublishedSchemaOwnership(schema *Schema) error {
	if schema == nil {
		return fmt.Errorf("published schema is nil")
	}
	return auditPublishedSchemaValue(reflect.ValueOf(schema), "Schema", make(map[publishedSchemaVisit]bool))
}

func auditPublishedSchemaValue(value reflect.Value, path string, seen map[publishedSchemaVisit]bool) error {
	if !value.IsValid() {
		return nil
	}
	forbidden := forbiddenPublishedSchemaType(value.Type())

	switch value.Kind() {
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String:
	case reflect.Chan, reflect.UnsafePointer:
		return fmt.Errorf("published schema retains opaque mutable state %s at %s", value.Type(), path)
	case reflect.Interface:
		if value.IsNil() {
			return nil
		}
		if err := auditPublishedSchemaValue(value.Elem(), path, seen); err != nil {
			return err
		}
	case reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		visit := publishedSchemaVisit{typ: value.Type(), ptr: value.Pointer()}
		if seen[visit] {
			return nil
		}
		seen[visit] = true
		if err := auditPublishedSchemaValue(value.Elem(), path, seen); err != nil {
			return err
		}
	case reflect.Func:
		if !value.IsNil() {
			return fmt.Errorf("published schema retains callback %s at %s", value.Type(), path)
		}
	case reflect.Struct:
		for i := range value.NumField() {
			field := value.Type().Field(i)
			if err := auditPublishedSchemaValue(value.Field(i), path+"."+field.Name, seen); err != nil {
				return err
			}
		}
	case reflect.Array, reflect.Slice:
		if value.Kind() == reflect.Slice {
			if value.IsNil() {
				return nil
			}
			if ptr := value.Pointer(); ptr != 0 {
				visit := publishedSchemaVisit{typ: value.Type(), ptr: ptr}
				if seen[visit] {
					return nil
				}
				seen[visit] = true
			}
		}
		for i := range value.Len() {
			if err := auditPublishedSchemaValue(value.Index(i), fmt.Sprintf("%s[%d]", path, i), seen); err != nil {
				return err
			}
		}
	case reflect.Map:
		if value.IsNil() {
			return nil
		}
		iter := value.MapRange()
		for iter.Next() {
			if err := auditPublishedSchemaValue(iter.Key(), path+".key", seen); err != nil {
				return err
			}
			if err := auditPublishedSchemaValue(iter.Value(), path+".value", seen); err != nil {
				return err
			}
		}
	}
	if forbidden {
		return fmt.Errorf("published schema retains construction type %s at %s", value.Type(), path)
	}
	return nil
}

func forbiddenPublishedSchemaType(typ reflect.Type) bool {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.PkgPath() == sourcePackagePath {
		return true
	}
	if typ.PkgPath() == valuePackagePath {
		switch typ.Name() {
		case "BoundFacet", "Builder", "FacetSpec", "LiteralSpec", "Resolver", "TypeSpec":
			return true
		}
	}
	if typ.PkgPath() != schemaPackagePath {
		return false
	}
	name := typ.Name()
	if name == "schemaBuild" || name == "compiler" || name == "schemaNode" || name == "schemaComponent" || name == "schemaContext" {
		return true
	}
	return strings.Contains(name, "Document") || strings.HasSuffix(name, "Source") || strings.HasPrefix(name, "compiler")
}

func TestComplexTypeReadDerivesValidationViews(t *testing.T) {
	t.Parallel()

	ct := ComplexType{
		Content:     3,
		Attrs:       4,
		TextType:    5,
		ContentKind: ContentSimpleMixed,
		Block:       DerivationExtension,
		Abstract:    true,
	}
	read := newComplexTypeRead(ct)
	if read.contentModel != ct.Content || read.attributeUseSet != ct.Attrs {
		t.Fatalf("complex type IDs = content %d attrs %d", read.contentModel, read.attributeUseSet)
	}
	wantInfo := newTypeInfo(typeInfoShape{Block: ct.Block, Abstract: ct.Abstract})
	if got := read.typeInfo(); got != wantInfo {
		t.Fatalf("typeInfo() = %+v, want %+v", got, wantInfo)
	}
	wantSimple := newSimpleContentTypeRead(simpleContentTypeReadShape{Type: ct.TextType, Present: ct.SimpleContent()})
	if got := read.simpleContent(); got != wantSimple {
		t.Fatalf("simpleContent() = %+v, want %+v", got, wantSimple)
	}
	for _, fixed := range []bool{false, true} {
		wantText := ElementTextContent{kind: ct.ContentKind, fixed: fixed, constrained: fixed}
		if got := read.textContent(fixed, fixed); got != wantText {
			t.Fatalf("textContent(%v) = %+v, want %+v", fixed, got, wantText)
		}
	}
}
