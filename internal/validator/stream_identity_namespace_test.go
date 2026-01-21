package validator

import (
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

func TestKeyRefUsesReferQNameNamespace(t *testing.T) {
	run := &streamRun{validationRun: &validationRun{}}
	scope := &identityScope{
		keyTables: map[types.QName]map[string]string{
			{Namespace: "urn:a", Local: "id"}: {"val": "/a"},
		},
	}
	keyref := &types.IdentityConstraint{
		Name:       "ref",
		Type:       types.KeyRefConstraint,
		ReferQName: types.QName{Namespace: "urn:b", Local: "id"},
	}
	compiled := &grammar.CompiledConstraint{Original: keyref}
	scope.constraints = []*constraintState{{constraint: compiled}}
	scope.keyRefs = []keyRefEntry{{
		constraint: compiled,
		value:      "val",
		display:    "val",
		path:       "/root",
		line:       1,
		column:     1,
	}}

	run.finalizeKeyRefs(scope)

	if !hasViolationCode(run.violations, errors.ErrIdentityKeyRefFailed) {
		t.Fatalf("expected keyref resolution failure, got %v", run.violations)
	}
}
