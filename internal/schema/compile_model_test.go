package schema

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestComplexTypeReachableModelsRejectsMissingModel(t *testing.T) {
	t.Parallel()
	const document = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="G"><xs:sequence><xs:element name="a"/></xs:sequence></xs:group>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:group ref="G"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	for _, tt := range []struct {
		name    string
		corrupt func(c *compiler, root contentModelID)
	}{
		{name: "complex-type root", corrupt: func(c *compiler, root contentModelID) {
			for i := range c.rt.ComplexTypes {
				if c.rt.ComplexTypes[i].Content == root {
					c.rt.ComplexTypes[i].Content = noContentModel - 1
				}
			}
		}},
		{name: "nested particle", corrupt: func(c *compiler, root contentModelID) {
			c.rt.Models[root].Particles[0].Model = noContentModel - 1
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, root := newEmptyComplexTypePublicationCompiler(t, document)
			if particles := c.rt.Models[root].Particles; len(particles) != 1 || particles[0].Kind != particleModel {
				t.Fatalf("fixture: root particles = %+v, want one model particle", particles)
			}
			tt.corrupt(c, root)
			_, err := c.complexTypeReachableModels()
			expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestComplexTypeReachableModelsChargesEachModelAndParticleItReads(t *testing.T) {
	t.Parallel()
	const references, particles = 40, 25
	const (
		rootSequenceWork   = 1 + references
		repeatedGroupWork  = references * (1 + particles)
		anyTypeContentWork = 1 + 1
		walkWork           = rootSequenceWork + repeatedGroupWork + anyTypeContentWork
	)
	var document strings.Builder
	document.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:group name="G"><xs:sequence>`)
	for i := range particles {
		fmt.Fprintf(&document, `<xs:element name="e%d"/>`, i)
	}
	document.WriteString(`</xs:sequence></xs:group><xs:element name="root"><xs:complexType><xs:sequence>`)
	for i := range references {
		fmt.Fprintf(&document, `<xs:element name="w%d"><xs:complexType><xs:group ref="G" maxOccurs="2"/></xs:complexType></xs:element>`+"\n", i)
	}
	document.WriteString(`</xs:sequence></xs:complexType></xs:element></xs:schema>`)
	c, _ := newEmptyComplexTypePublicationCompiler(t, document.String())
	c.contentWork = newWorkBudget(contentModelWorkBudget, walkWork)
	if _, err := c.complexTypeReachableModels(); err != nil {
		t.Fatalf("walk with %d steps: %v", walkWork, err)
	}
	c.contentWork = newWorkBudget(contentModelWorkBudget, walkWork-1)
	_, err := c.complexTypeReachableModels()
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	var diag *xsderrors.Error
	if errors.As(err, &diag); diag.Line() < 2 {
		t.Fatalf("limit located at line %d, want the line of a referring complex type: %v", diag.Line(), err)
	}
}
