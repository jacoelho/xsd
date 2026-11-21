package schema

import (
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
