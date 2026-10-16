package schema

import "testing"

func TestSemanticSourcePointerProjection(t *testing.T) {
	t.Parallel()

	t.Run("complex type", func(t *testing.T) {
		t.Parallel()
		checkSemanticSourcePointerProjection(t, semanticSource[*schemaComplexTypeSource], &schemaComplexTypeSource{})
	})
	t.Run("facet", func(t *testing.T) {
		t.Parallel()
		checkSemanticSourcePointerProjection(t, semanticSource[*schemaFacetSource], &schemaFacetSource{})
	})
	t.Run("element", func(t *testing.T) {
		t.Parallel()
		checkSemanticSourcePointerProjection(t, semanticSource[*schemaElementSource], &schemaElementSource{})
	})
	t.Run("attribute", func(t *testing.T) {
		t.Parallel()
		checkSemanticSourcePointerProjection(t, semanticSource[*schemaAttributeSource], &schemaAttributeSource{})
	})
	t.Run("attribute group", func(t *testing.T) {
		t.Parallel()
		checkSemanticSourcePointerProjection(t, semanticSource[*schemaAttributeGroupSource], &schemaAttributeGroupSource{})
	})
	t.Run("model", func(t *testing.T) {
		t.Parallel()
		checkSemanticSourcePointerProjection(t, semanticSource[*schemaModelSource], &schemaModelSource{})
	})
	t.Run("identity", func(t *testing.T) {
		t.Parallel()
		checkSemanticSourcePointerProjection(t, semanticSource[*schemaIdentitySource], &schemaIdentitySource{})
	})
	t.Run("identity XPath", func(t *testing.T) {
		t.Parallel()
		checkSemanticSourcePointerProjection(t, semanticSource[*schemaIdentityXPathSource], &schemaIdentityXPathSource{})
	})
	t.Run("wildcard", func(t *testing.T) {
		t.Parallel()
		checkSemanticSourcePointerProjection(t, semanticSource[*schemaWildcardSource], &schemaWildcardSource{})
	})
	t.Run("group", func(t *testing.T) {
		t.Parallel()
		checkSemanticSourcePointerProjection(t, semanticSource[*schemaGroupSource], &schemaGroupSource{})
	})
}

func checkSemanticSourcePointerProjection[T interface {
	schemaSemanticKind
	comparable
}](t *testing.T, read func(*schemaSemanticSource) T, pointer T) {
	t.Helper()

	var nilPointer T
	var nilReference *schemaReferenceSource
	for _, test := range []struct {
		name   string
		source *schemaSemanticSource
		want   T
	}{
		{name: "nil source", want: nilPointer},
		{name: "nil kind", source: &schemaSemanticSource{}, want: nilPointer},
		{name: "wrong kind", source: &schemaSemanticSource{kind: &schemaReferenceSource{}}, want: nilPointer},
		{name: "wrong typed nil", source: &schemaSemanticSource{kind: nilReference}, want: nilPointer},
		{name: "matching typed nil", source: &schemaSemanticSource{kind: nilPointer}, want: nilPointer},
		{name: "pointer identity", source: &schemaSemanticSource{kind: pointer}, want: pointer},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := read(test.source)
			if got != test.want {
				t.Fatalf("projection = %p; want original pointer %p", any(got), any(test.want))
			}
		})
	}
}
