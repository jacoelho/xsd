package schema_test

import (
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
)

func TestValueConstraintIdentityClonesResolvedNames(t *testing.T) {
	t.Parallel()

	vc := &xsdSchema.ValueConstraint{
		ResolvedNames: []xsdSchema.ResolvedValueName{{Lexical: "p:item"}},
	}
	identity := xsdSchema.NewValueConstraintIdentity(vc)
	identity.ResolvedNames[0].Lexical = "p:other"
	if vc.ResolvedNames[0].Lexical != "p:item" {
		t.Fatalf("NewValueConstraintIdentity returned table-backed resolved names: %#v", vc.ResolvedNames)
	}
}
