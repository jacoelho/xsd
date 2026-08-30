package compile

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestComponentDependencyWorkBoundsAcyclicChains(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"simple", "complex", "model-group", "attribute-group"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			schema := componentChainSchema(kind, 32)
			if _, err := Compile(Options{}, []source.Source{source.Bytes(kind+".xsd", []byte(schema))}); err != nil {
				t.Fatalf("Compile(default) error = %v", err)
			}
			_, err := Compile(
				Options{MaxSchemaDependencySteps: 8},
				[]source.Source{source.Bytes(kind+".xsd", []byte(schema))},
			)
			expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
		})
	}
}

func TestComponentDependencyDepthRejectsDeepAcyclicChains(t *testing.T) {
	for _, kind := range []string{"simple", "complex", "model-group", "attribute-group"} {
		t.Run(kind, func(t *testing.T) {
			schema := componentChainSchema(kind, maxComponentDependencyDepth+1)
			_, err := Compile(Options{}, []source.Source{source.Bytes(kind+".xsd", []byte(schema))})
			expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
			if !strings.Contains(err.Error(), "component dependency depth") {
				t.Fatalf("Compile() error = %v, want component-depth limit", err)
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Line() == 0 {
				t.Fatalf("Compile() error = %v, want source location", err)
			}
		})
	}
}

func TestComponentCyclePrecedesDependencyLimit(t *testing.T) {
	t.Parallel()

	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T0"><xs:restriction base="T1"/></xs:simpleType>
  <xs:simpleType name="T1"><xs:restriction base="T0"/></xs:simpleType>
</xs:schema>`
	_, err := Compile(
		Options{MaxSchemaDependencySteps: 16},
		[]source.Source{source.Bytes("cycle.xsd", []byte(schema))},
	)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaReference)
}

func TestSchemaGraphAndComponentsShareDependencyBudget(t *testing.T) {
	t.Parallel()

	child := []byte(componentChainSchema("simple", 3))
	budget := 1
	for ; budget <= 64; budget++ {
		if _, err := Compile(
			Options{MaxSchemaDependencySteps: budget},
			[]source.Source{source.Bytes("child.xsd", child)},
		); err == nil {
			break
		}
	}
	if budget > 64 {
		t.Fatal("component-only schema did not compile within calibration bound")
	}
	root := source.Bytes("root.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/></xs:schema>`)).WithResolver(
		source.Resolver(func(_, location string) (source.Source, error) {
			if location != "child.xsd" {
				return source.Source{}, xsderrors.ErrSchemaNotFound
			}
			return source.Bytes("child.xsd", child), nil
		}),
	)
	_, err := Compile(Options{MaxSchemaDependencySteps: budget}, []source.Source{root})
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
}

func componentChainSchema(kind string, count int) string {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	for index := range count {
		name := fmt.Sprintf("%s%04d", componentPrefix(kind), index)
		next := fmt.Sprintf("%s%04d", componentPrefix(kind), index+1)
		switch kind {
		case "simple":
			base := "xs:string"
			if index+1 < count {
				base = next
			}
			fmt.Fprintf(&schema, `<xs:simpleType name="%s"><xs:restriction base="%s"/></xs:simpleType>`, name, base)
		case "complex":
			if index+1 == count {
				fmt.Fprintf(&schema, `<xs:complexType name="%s"/>`, name)
				continue
			}
			fmt.Fprintf(&schema, `<xs:complexType name="%s"><xs:complexContent><xs:extension base="%s"/></xs:complexContent></xs:complexType>`, name, next)
		case "model-group":
			if index+1 == count {
				fmt.Fprintf(&schema, `<xs:group name="%s"><xs:sequence/></xs:group>`, name)
				continue
			}
			fmt.Fprintf(&schema, `<xs:group name="%s"><xs:sequence><xs:group ref="%s"/></xs:sequence></xs:group>`, name, next)
		case "attribute-group":
			if index+1 == count {
				fmt.Fprintf(&schema, `<xs:attributeGroup name="%s"><xs:attribute name="value" type="xs:string"/></xs:attributeGroup>`, name)
				continue
			}
			fmt.Fprintf(&schema, `<xs:attributeGroup name="%s"><xs:attributeGroup ref="%s"/></xs:attributeGroup>`, name, next)
		default:
			panic("unknown component chain kind")
		}
	}
	schema.WriteString(`</xs:schema>`)
	return schema.String()
}

func componentPrefix(kind string) string {
	switch kind {
	case "simple":
		return "T"
	case "complex":
		return "C"
	case "model-group":
		return "G"
	case "attribute-group":
		return "A"
	default:
		panic("unknown component chain kind")
	}
}
