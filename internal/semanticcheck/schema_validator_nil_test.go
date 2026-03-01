package semanticcheck

import "testing"

func TestValidateStructureNilSchema(t *testing.T) {
	errs := ValidateStructure(nil)
	if len(errs) == 0 {
		t.Fatal("ValidateStructure(nil) returned no errors")
	}
}
