package schema_test

import (
	"slices"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestPublicationRejectsNormalizedExtensionBaseCorruption(t *testing.T) {
	const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Base">
    <xs:sequence minOccurs="0" maxOccurs="2"><xs:element name="a"/><xs:element name="c"/></xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent><xs:extension base="Base">
      <xs:sequence><xs:element name="b"/></xs:sequence>
    </xs:extension></xs:complexContent>
  </xs:complexType>
</xs:schema>`
	for _, mutation := range []struct {
		name  string
		apply func(derived, normalized *xsdSchema.ContentModel)
	}{
		{"dropped base", func(derived, _ *xsdSchema.ContentModel) {
			derived.Particles = derived.Particles[1:]
		}},
		{"reordered base", func(derived, _ *xsdSchema.ContentModel) {
			slices.Reverse(derived.Particles)
		}},
		{"changed minimum", func(derived, _ *xsdSchema.ContentModel) {
			derived.Particles[0].Occurs.Min = 1
		}},
		{"changed maximum", func(derived, _ *xsdSchema.ContentModel) {
			derived.Particles[0].Occurs.Max = 3
		}},
		{"changed compositor", func(_, normalized *xsdSchema.ContentModel) {
			normalized.Kind = xsdSchema.ModelChoice
		}},
		{"dropped child", func(_, normalized *xsdSchema.ContentModel) {
			normalized.Particles = normalized.Particles[:1]
		}},
		{"reordered children", func(_, normalized *xsdSchema.ContentModel) {
			slices.Reverse(normalized.Particles)
		}},
		{"changed child occurrence", func(_, normalized *xsdSchema.ContentModel) {
			normalized.Particles[0].Occurs.Min = 0
		}},
		{"repeated clone", func(_, normalized *xsdSchema.ContentModel) {
			normalized.Occurs.Max = 2
		}},
		{"changed mixed content", func(_, normalized *xsdSchema.ContentModel) {
			normalized.Mixed = true
		}},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			build := mutableSchemaBuild(t, source)
			if err := validateSchemaBuild(build); err != nil {
				t.Fatalf("ValidateSchema() before mutation error = %v", err)
			}
			derivedID := complexBuildTypeIDByName(t, build, "Derived")
			derived := &build.Models[build.ComplexTypes[derivedID].Content]
			normalized := &build.Models[derived.Particles[0].Model]
			// Mutate only the clone even if construction shares immutable child storage.
			normalized.Particles = slices.Clone(normalized.Particles)
			mutation.apply(derived, normalized)
			published, err := publishSchema(build)
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			if published != nil {
				t.Fatal("PublishSchema() returned a schema with corrupted inherited content")
			}
		})
	}
}
