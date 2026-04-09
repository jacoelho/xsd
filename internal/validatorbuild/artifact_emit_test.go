package validatorbuild

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestCompileUnionValidatorKeepsMemberCountsAligned(t *testing.T) {
	c := newArtifactCompiler(nil)
	c.registry = &analysis.Registry{}
	c.builtinTypeIDs = make(map[model.TypeName]runtime.TypeID)
	for i, name := range analysis.BuiltinTypeNames() {
		c.builtinTypeIDs[name] = runtime.TypeID(i + 1)
	}
	union := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "Union"},
		Union: &model.UnionType{},
		MemberTypes: []model.Type{
			model.GetBuiltin(model.TypeNameString),
			model.GetBuiltin(model.TypeNameInt),
		},
	}

	id, err := c.compileType(union)
	if err != nil {
		t.Fatalf("compileType(union) error = %v", err)
	}
	if id == 0 {
		t.Fatal("union validator id = 0")
	}
	if len(c.bundle.Union) != 1 {
		t.Fatalf("union validator count = %d, want 1", len(c.bundle.Union))
	}
	uv := c.bundle.Union[0]
	if uv.MemberLen != 2 {
		t.Fatalf("union member len = %d, want 2", uv.MemberLen)
	}
	off := int(uv.MemberOff)
	end := off + int(uv.MemberLen)
	if len(c.bundle.UnionMembers[off:end]) != len(c.bundle.UnionMemberTypes[off:end]) {
		t.Fatalf("member validators = %d, member types = %d", len(c.bundle.UnionMembers[off:end]), len(c.bundle.UnionMemberTypes[off:end]))
	}
}

func TestAddUnionValidatorRejectsMemberTypeCountMismatch(t *testing.T) {
	c := newArtifactCompiler(nil)
	member := c.addAtomicValidator(runtime.VString, runtime.WSCollapse, runtime.FacetProgramRef{}, runtime.StringAny, 0)

	_, err := c.addUnionValidator(runtime.WSCollapse, runtime.FacetProgramRef{}, []runtime.ValidatorID{member}, nil, "urn:test:Broken", 0)
	if err == nil || !strings.Contains(err.Error(), "union member type count mismatch") {
		t.Fatalf("addUnionValidator() error = %v, want count mismatch", err)
	}
}
