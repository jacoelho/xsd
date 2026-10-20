package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestOmittedCompositorAdmissionConsumesDependencyBudget(t *testing.T) {
	t.Parallel()

	for _, exhausted := range []bool{false, true} {
		name := "available"
		if exhausted {
			name = "exhausted"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, component := omittedCompositorCompiler(t, "")
			if exhausted {
				c.dependencyWork = newWorkBudget(dependencyWorkBudget, 1)
				if err := c.dependencyWork.spend(1); err != nil {
					t.Fatal(err)
				}
			}
			_, err := c.compileModel(component.sourceNode(), component.ctx)
			if exhausted {
				expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
				var diagnostic *xsderrors.Error
				if !errors.As(err, &diagnostic) || diagnostic.Path() != "compositor.xsd" || diagnostic.Line() == 0 || diagnostic.Column() == 0 {
					t.Fatalf("admission diagnostic lacks source location: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = c.compileGlobals(); err != nil {
				t.Fatal(err)
			}
			if published, publishErr := c.publishSchema(); publishErr != nil || published == nil {
				t.Fatalf("publication with sufficient budget = %v, %v", published, publishErr)
			}
		})
	}
}

func TestOmittedCompositorAdmissionReusesAbsentModel(t *testing.T) {
	t.Parallel()

	c, component := omittedCompositorCompiler(t, "")
	if _, err := c.compileModel(component.sourceNode(), component.ctx); err != nil {
		t.Fatal(err)
	}
	c.dependencyWork = newWorkBudget(dependencyWorkBudget, 0)
	id, err := c.compileModel(component.sourceNode(), component.ctx)
	if err != nil || id != NoContentModel {
		t.Fatalf("reusing admitted absence with exhausted budget = %v, %v", id, err)
	}
}

func TestOmittedCompositorFailedAdmissionLeavesNoWork(t *testing.T) {
	t.Parallel()

	c, component := omittedCompositorCompiler(t, `<xs:element name="child"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="["/></xs:restriction></xs:simpleType></xs:element>`)
	available := c.dependencyWork
	c.dependencyWork = newWorkBudget(dependencyWorkBudget, 0)
	_, err := c.compileModel(component.sourceNode(), component.ctx)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)

	c.dependencyWork = available
	if err = c.completeComponentDefinitions(); err != nil {
		t.Fatalf("failed admission left descendant work: %v", err)
	}
	id, err := c.compileModel(component.sourceNode(), component.ctx)
	if err != nil || id != NoContentModel {
		t.Fatalf("retrying admission with available budget = %v, %v", id, err)
	}
	err = c.completeComponentDefinitions()
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaFacet)
}

func TestCompilerPublicationRequiresCompletedOmittedDescendants(t *testing.T) {
	t.Parallel()

	c, component := omittedCompositorCompiler(t, "")
	if err := c.compileGlobals(); err != nil {
		t.Fatal(err)
	}
	// Change only pending work; completed runtime metadata remains valid.
	c.deferredModelChildren = []schemaComponent{component}
	published, err := c.publishSchema()
	if published != nil {
		t.Fatal("published with uncompleted compositor descendants")
	}
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	c.deferredModelChildren = nil
	if published, err = c.publishSchema(); err != nil || published == nil {
		t.Fatalf("publication after restoring completed work = %v, %v", published, err)
	}
}

func omittedCompositorCompiler(t *testing.T, descendants string) (*compiler, schemaComponent) {
	t.Helper()
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	input := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:complexType name="Base"><xs:sequence minOccurs="0" maxOccurs="0">` + descendants + `</xs:sequence></xs:complexType><xs:element name="root" type="Base"/></xs:schema>`
	if err = c.loadOwned([]source.Source{source.Bytes("compositor.xsd", []byte(input))}); err != nil {
		t.Fatal(err)
	}
	if err = c.index(); err != nil {
		t.Fatal(err)
	}
	q, err := c.rt.internQName("", "Base")
	if err != nil {
		t.Fatal(err)
	}
	component := c.complexComponents[q]
	component.id = component.sourceNode().firstXS(sequenceChild).id
	return c, component
}
