package globaldecl

import (
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestForEachDispatchesInOrder(t *testing.T) {
	s := parser.NewSchema()
	elemName := model.QName{Namespace: "urn:test", Local: "e"}
	typeName := model.QName{Namespace: "urn:test", Local: "t"}
	attrName := model.QName{Namespace: "urn:test", Local: "a"}
	s.ElementDecls[elemName] = &model.ElementDecl{Name: elemName}
	s.TypeDefs[typeName] = &model.SimpleType{QName: typeName}
	s.AttributeDecls[attrName] = &model.AttributeDecl{Name: attrName}
	s.GlobalDecls = []parser.GlobalDecl{
		{Kind: parser.GlobalDeclElement, Name: elemName},
		{Kind: parser.GlobalDeclType, Name: typeName},
		{Kind: parser.GlobalDeclAttribute, Name: attrName},
	}

	var got []string
	err := ForEach(s, Handlers{
		Element: func(name model.QName, _ *model.ElementDecl) error {
			got = append(got, "element:"+name.Local)
			return nil
		},
		Type: func(name model.QName, _ model.Type) error {
			got = append(got, "type:"+name.Local)
			return nil
		},
		Attribute: func(name model.QName, _ *model.AttributeDecl) error {
			got = append(got, "attribute:"+name.Local)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ForEach() error = %v", err)
	}
	want := []string{"element:e", "type:t", "attribute:a"}
	if len(got) != len(want) {
		t.Fatalf("ForEach() count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ForEach() order[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestForEachTypeFiltersKinds(t *testing.T) {
	s := parser.NewSchema()
	typeA := model.QName{Namespace: "urn:test", Local: "A"}
	typeB := model.QName{Namespace: "urn:test", Local: "B"}
	elem := model.QName{Namespace: "urn:test", Local: "E"}
	s.TypeDefs[typeA] = &model.SimpleType{QName: typeA}
	s.TypeDefs[typeB] = &model.SimpleType{QName: typeB}
	s.ElementDecls[elem] = &model.ElementDecl{Name: elem}
	s.GlobalDecls = []parser.GlobalDecl{
		{Kind: parser.GlobalDeclType, Name: typeA},
		{Kind: parser.GlobalDeclElement, Name: elem},
		{Kind: parser.GlobalDeclType, Name: typeB},
	}

	var got []string
	err := ForEachType(s, func(name model.QName, typ model.Type) error {
		if typ == nil {
			t.Fatalf("type %s missing", name)
		}
		got = append(got, name.Local)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachType() error = %v", err)
	}
	want := []string{"A", "B"}
	if len(got) != len(want) {
		t.Fatalf("ForEachType() count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ForEachType() order[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDispatchUnknownKindWithoutHandlerErrors(t *testing.T) {
	s := parser.NewSchema()
	decl := parser.GlobalDecl{
		Kind: parser.GlobalDeclKind(255),
		Name: model.QName{Namespace: "urn:test", Local: "x"},
	}
	if err := Dispatch(s, decl, Handlers{}); err == nil {
		t.Fatalf("Dispatch() expected error for unknown kind without handler")
	}
}

func TestDispatchUnknownKindUsesHandler(t *testing.T) {
	s := parser.NewSchema()
	decl := parser.GlobalDecl{
		Kind: parser.GlobalDeclKind(255),
		Name: model.QName{Namespace: "urn:test", Local: "x"},
	}
	called := false
	err := Dispatch(s, decl, Handlers{
		Unknown: func(kind parser.GlobalDeclKind, name model.QName) error {
			called = true
			if kind != decl.Kind {
				return fmt.Errorf("kind = %d, want %d", kind, decl.Kind)
			}
			if name != decl.Name {
				return fmt.Errorf("name = %s, want %s", name, decl.Name)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if !called {
		t.Fatalf("unknown handler was not called")
	}
}

func TestDispatchMissingDeclarationPassesNilValue(t *testing.T) {
	s := parser.NewSchema()
	name := model.QName{Namespace: "urn:test", Local: "missingType"}
	decl := parser.GlobalDecl{
		Kind: parser.GlobalDeclType,
		Name: name,
	}
	called := false
	err := Dispatch(s, decl, Handlers{
		Type: func(gotName model.QName, typ model.Type) error {
			called = true
			if gotName != name {
				return fmt.Errorf("name = %s, want %s", gotName, name)
			}
			if typ != nil {
				return fmt.Errorf("type = %T, want nil", typ)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if !called {
		t.Fatalf("type handler was not called")
	}
}
