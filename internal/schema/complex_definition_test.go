package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestComplexDefinitionCompletionConsumesDependencyBudget(t *testing.T) {
	t.Parallel()

	for _, exhausted := range []bool{false, true} {
		name := "available"
		if exhausted {
			name = "exhausted"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := complexDefinitionCompiler(t)
			q, err := c.rt.internQName("", "Base")
			if err != nil {
				t.Fatal(err)
			}
			id, err := c.reserveNamedComplexType(q)
			if err != nil {
				t.Fatal(err)
			}
			if exhausted {
				c.dependencyWork = newWorkBudget(dependencyWorkBudget, 1)
				if err = c.dependencyWork.spend(1); err != nil {
					t.Fatal(err)
				}
			}
			err = c.completeComplexDefinition(id)
			if !exhausted {
				if err != nil {
					t.Fatalf("completion with available budget: %v", err)
				}
				return
			}
			expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
			var diag *xsderrors.Error
			if !errors.As(err, &diag) || diag.Path() != "schema.xsd" || diag.Line() == 0 || diag.Column() == 0 {
				t.Fatalf("completion diagnostic lacks source location: %v", err)
			}
		})
	}
}

func TestCompilerPublicationRequiresCompletedComplexDefinitions(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		state complexTypeCompletion
	}{
		{name: "reserved", state: complexTypeReserved},
		{name: "completing", state: complexTypeCompleting},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := complexDefinitionCompiler(t)
			if err := c.compileGlobals(); err != nil {
				t.Fatal(err)
			}
			q, err := c.rt.internQName("", "Base")
			if err != nil {
				t.Fatal(err)
			}
			id, ok := c.rt.GlobalTypes[q].Complex()
			if !ok {
				t.Fatal("Base did not compile as a complex type")
			}
			// Corrupt only construction state so runtime validity cannot mask the gate.
			c.complexDefinitions[id].state = tt.state
			published, err := c.publishSchema()
			if published != nil {
				t.Fatal("published an incomplete complex definition")
			}
			expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			c.complexDefinitions[id].state = complexTypeComplete
			if published, err = c.publishSchema(); err != nil || published == nil {
				t.Fatalf("publication after restoring completion = %v, %v", published, err)
			}
		})
	}
}

func complexDefinitionCompiler(t *testing.T) *compiler {
	t.Helper()
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	input := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:complexType name="Base"/><xs:element name="root" type="Base"/></xs:schema>`
	if err = c.loadOwned([]source.Source{source.Bytes("schema.xsd", []byte(input))}); err != nil {
		t.Fatal(err)
	}
	if err = c.index(); err != nil {
		t.Fatal(err)
	}
	return c
}
