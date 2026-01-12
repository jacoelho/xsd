package loader

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestResolveLocationAndGetLoaded(t *testing.T) {
	loader := NewLoader(Config{BasePath: "schemas"})
	schema := &parser.Schema{}

	abs := loader.resolveLocation("/abs/schema.xsd")
	if abs != "/abs/schema.xsd" {
		t.Fatalf("expected absolute path to remain unchanged, got %q", abs)
	}

	rel := loader.resolveLocation("a/b.xsd")
	if rel != "schemas/a/b.xsd" {
		t.Fatalf("expected base path join, got %q", rel)
	}

	loader.state.loaded[rel] = schema
	loaded, ok := loader.GetLoaded("a/b.xsd")
	if !ok || loaded != schema {
		t.Fatalf("expected GetLoaded to return cached schema")
	}
}

func TestIsNotFound(t *testing.T) {
	if !isNotFound(fs.ErrNotExist) {
		t.Fatalf("expected isNotFound to detect ErrNotExist")
	}
	if isNotFound(errors.New("other")) {
		t.Fatalf("expected non-ErrNotExist to be false")
	}
}

func TestDeepCopyModelGroup(t *testing.T) {
	original := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{
			&types.ElementDecl{Name: types.QName{Local: "a"}},
		},
	}

	clone := deepCopyModelGroup(original)
	if clone == original {
		t.Fatalf("expected a new model group instance")
	}
	if len(clone.Particles) != len(original.Particles) {
		t.Fatalf("expected copied particles length to match")
	}
	clone.Particles[0] = &types.ElementDecl{Name: types.QName{Local: "b"}}
	if original.Particles[0].(*types.ElementDecl).Name.Local != "a" {
		t.Fatalf("expected original particles to remain unchanged")
	}
}

func TestNormalizeAttributeForms(t *testing.T) {
	qualified := &types.AttributeDecl{Name: types.QName{Local: "q"}, Form: types.FormDefault}
	extAttr := &types.AttributeDecl{Name: types.QName{Local: "e"}, Form: types.FormDefault}
	ct := &types.ComplexType{}
	ct.SetAttributes([]*types.AttributeDecl{qualified})
	ct.SetContent(&types.ComplexContent{
		Extension: &types.Extension{Attributes: []*types.AttributeDecl{extAttr}},
	})

	normalizeAttributeForms(ct, parser.Qualified)
	if qualified.Form != types.FormQualified {
		t.Fatalf("expected qualified attribute to be FormQualified")
	}
	if extAttr.Form != types.FormQualified {
		t.Fatalf("expected extension attribute to be FormQualified")
	}

	restrAttr := &types.AttributeDecl{Name: types.QName{Local: "r"}, Form: types.FormDefault}
	ct = &types.ComplexType{}
	ct.SetAttributes([]*types.AttributeDecl{{Name: types.QName{Local: "u"}, Form: types.FormDefault}})
	ct.SetContent(&types.ComplexContent{
		Restriction: &types.Restriction{Attributes: []*types.AttributeDecl{restrAttr}},
	})
	normalizeAttributeForms(ct, parser.Unqualified)
	for _, attr := range []*types.AttributeDecl{ct.Attributes()[0], restrAttr} {
		if attr.Form != types.FormUnqualified {
			t.Fatalf("expected FormUnqualified, got %v", attr.Form)
		}
	}
}

func TestElementDeclEquivalent(t *testing.T) {
	elemA := &types.ElementDecl{
		Name:     types.QName{Local: "a"},
		Type:     types.GetBuiltin(types.TypeNameString),
		Form:     types.FormQualified,
		Fixed:    "x",
		HasFixed: true,
	}
	elemB := &types.ElementDecl{
		Name:     types.QName{Local: "a"},
		Type:     types.GetBuiltin(types.TypeNameString),
		Form:     types.FormQualified,
		Fixed:    "x",
		HasFixed: true,
	}

	if !elementDeclEquivalent(elemA, elemB) {
		t.Fatalf("expected equivalent element declarations")
	}

	elemB.Default = "y"
	if elementDeclEquivalent(elemA, elemB) {
		t.Fatalf("expected default mismatch to be non-equivalent")
	}
}
