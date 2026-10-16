package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestCachedGlobalLookupConsumesDependencyBudget(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		local  string
		lookup func(*compiler, QName) (uint32, error)
	}{
		{name: "simple", local: "S", lookup: func(c *compiler, q QName) (uint32, error) {
			id, err := c.compileSimpleByQName(q)
			return uint32(id), err
		}},
		{name: "complex", local: "C", lookup: func(c *compiler, q QName) (uint32, error) {
			id, err := c.compileComplexByQName(q)
			return uint32(id), err
		}},
		{name: "attribute", local: "a", lookup: func(c *compiler, q QName) (uint32, error) {
			id, err := c.compileAttributeByQName(q)
			return uint32(id), err
		}},
		{name: "element", local: "r", lookup: func(c *compiler, q QName) (uint32, error) {
			id, err := c.compileElementByQName(q)
			return uint32(id), err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			limits, err := NormalizeOptions(Options{})
			if err != nil {
				t.Fatal(err)
			}
			c, err := newCompiler(limits)
			if err != nil {
				t.Fatal(err)
			}
			const input = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="S"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:complexType name="C"/>
  <xs:attribute name="a" type="S"/>
  <xs:element name="r" type="C"/>
</xs:schema>`
			if loadErr := c.loadOwned([]source.Source{source.Bytes("cache.xsd", []byte(input))}); loadErr != nil {
				t.Fatal(loadErr)
			}
			if indexErr := c.index(); indexErr != nil {
				t.Fatal(indexErr)
			}
			if compileErr := c.compileGlobals(); compileErr != nil {
				t.Fatal(compileErr)
			}
			q, err := c.rt.internQName("", test.local)
			if err != nil {
				t.Fatal(err)
			}
			original, err := test.lookup(c, q)
			if err != nil {
				t.Fatal(err)
			}
			c.dependencyWork = newWorkBudget(dependencyWorkBudget, 1)
			cachedID, cachedErr := test.lookup(c, q)
			if cachedErr != nil || cachedID != original {
				t.Fatalf("cached lookup with one unit = %d, %v; want stable ID %d", cachedID, cachedErr, original)
			}
			_, err = test.lookup(c, q)
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaLimit || diagnostic.Message() != "compiler work exceeds MaxSchemaDependencySteps (limit 1)" {
				t.Fatalf("cached lookup after budget spent = %v; want schema dependency limit", err)
			}
		})
	}
}
