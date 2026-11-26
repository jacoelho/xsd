package schema

import "testing"

func TestRuntimePublicationCloneHelpersDoNotAliasMutableState(t *testing.T) {
	valueConstraintIdentity := constraintIdentity{
		ResolvedNames: []resolvedValueName{{Lexical: "p:item"}},
	}
	clonedValueConstraintIdentity := cloneValueConstraintIdentity(valueConstraintIdentity)
	valueConstraintIdentity.ResolvedNames[0].Lexical = "p:other"
	if clonedValueConstraintIdentity.ResolvedNames[0].Lexical != "p:item" {
		t.Fatalf("CloneValueConstraintIdentity aliased resolved-name slice: %#v", clonedValueConstraintIdentity.ResolvedNames)
	}
}
