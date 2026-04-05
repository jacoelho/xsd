package semantics

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func TestCanonicalizeDefaultFixedUnionQNameMemberUsesContext(t *testing.T) {
	c := newCompiler(nil)
	c.registry = &analysis.Registry{}
	c.builtinTypeIDs = make(map[model.TypeName]runtime.TypeID)
	for i, name := range BuiltinTypeNames() {
		c.builtinTypeIDs[name] = runtime.TypeID(i + 1)
	}
	union := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "QNameUnion"},
		Union: &model.UnionType{},
		MemberTypes: []model.Type{
			model.GetBuiltin(model.TypeNameQName),
			model.GetBuiltin(model.TypeNameString),
		},
	}
	ctx := map[string]string{"p": "urn:test"}
	want, err := value.CanonicalQName([]byte("p:name"), mapResolver(ctx), nil)
	if err != nil {
		t.Fatalf("CanonicalQName() error = %v", err)
	}

	got, memberID, keyRef, err := c.canonicalizeDefaultFixed("p:name", union, ctx)
	if err != nil {
		t.Fatalf("canonicalizeDefaultFixed() error = %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("canonical bytes = %q, want %q", got, want)
	}
	if memberID == 0 {
		t.Fatal("member validator id = 0, want compiled QName member validator")
	}
	if keyRef.Kind != runtime.VKQName || !keyRef.Ref.Present {
		t.Fatalf("key ref = %#v, want bytes key", keyRef)
	}
}

func TestCanonicalizeNormalizedDefaultWithMemberSelectsFirstMatchingUnionMember(t *testing.T) {
	c := newCompiler(nil)
	union := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "Union"},
		Union: &model.UnionType{},
		MemberTypes: []model.Type{
			model.GetBuiltin(model.TypeNameInt),
			model.GetBuiltin(model.TypeNameString),
		},
	}

	got, member, err := c.canonicalizeNormalizedDefaultWithMember("007", "007", union, nil)
	if err != nil {
		t.Fatalf("canonicalizeNormalizedDefaultWithMember() error = %v", err)
	}
	if string(got) != "7" {
		t.Fatalf("canonical bytes = %q, want %q", got, "7")
	}
	if member == nil || member.Name().Local != "int" {
		t.Fatalf("member = %v, want xs:int", member)
	}
}

func TestCanonicalizeNormalizedDefaultWithMemberRejectsUnionMismatch(t *testing.T) {
	c := newCompiler(nil)
	union := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "Union"},
		Union: &model.UnionType{},
		MemberTypes: []model.Type{
			model.GetBuiltin(model.TypeNameInt),
			model.GetBuiltin(model.TypeNameDate),
		},
	}

	_, _, err := c.canonicalizeNormalizedDefaultWithMember("nope", "nope", union, nil)
	if err == nil || !strings.Contains(err.Error(), "union value does not match any member type") {
		t.Fatalf("canonicalizeNormalizedDefaultWithMember() error = %v, want union mismatch", err)
	}
}
