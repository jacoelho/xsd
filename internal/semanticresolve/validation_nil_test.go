package semanticresolve

import "testing"

func TestValidateReferencesNilSchema(t *testing.T) {
	errs := ValidateReferences(nil)
	if len(errs) == 0 {
		t.Fatal("ValidateReferences(nil) returned no errors")
	}
}

func TestResolverResolveNilSchema(t *testing.T) {
	if err := NewResolver(nil).Resolve(); err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
}
