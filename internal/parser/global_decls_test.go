package parser

import (
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestForEachGlobalDeclDispatchesInOrder(t *testing.T) {
	s := NewSchema()
	elemName := model.QName{Namespace: "urn:test", Local: "e"}
	typeName := model.QName{Namespace: "urn:test", Local: "t"}
	attrName := model.QName{Namespace: "urn:test", Local: "a"}
	s.ElementDecls[elemName] = &model.ElementDecl{Name: elemName}
	s.TypeDefs[typeName] = &model.SimpleType{QName: typeName}
	s.AttributeDecls[attrName] = &model.AttributeDecl{Name: attrName}
	s.GlobalDecls = []GlobalDecl{
		{Kind: GlobalDeclElement, Name: elemName},
		{Kind: GlobalDeclType, Name: typeName},
		{Kind: GlobalDeclAttribute, Name: attrName},
	}

	var got []string
	err := ForEachGlobalDecl(&s.SchemaGraph, GlobalDeclHandlers{
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
		t.Fatalf("ForEachGlobalDecl() error = %v", err)
	}
	want := []string{"element:e", "type:t", "attribute:a"}
	if len(got) != len(want) {
		t.Fatalf("ForEachGlobalDecl() count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ForEachGlobalDecl() order[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestForEachGlobalDeclTypeHandlerFiltersKinds(t *testing.T) {
	s := NewSchema()
	typeA := model.QName{Namespace: "urn:test", Local: "A"}
	typeB := model.QName{Namespace: "urn:test", Local: "B"}
	elem := model.QName{Namespace: "urn:test", Local: "E"}
	s.TypeDefs[typeA] = &model.SimpleType{QName: typeA}
	s.TypeDefs[typeB] = &model.SimpleType{QName: typeB}
	s.ElementDecls[elem] = &model.ElementDecl{Name: elem}
	s.GlobalDecls = []GlobalDecl{
		{Kind: GlobalDeclType, Name: typeA},
		{Kind: GlobalDeclElement, Name: elem},
		{Kind: GlobalDeclType, Name: typeB},
	}

	var got []string
	err := ForEachGlobalDecl(&s.SchemaGraph, GlobalDeclHandlers{
		Type: func(name model.QName, typ model.Type) error {
			if typ == nil {
				t.Fatalf("type %s missing", name)
			}
			got = append(got, name.Local)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ForEachGlobalDecl() error = %v", err)
	}
	want := []string{"A", "B"}
	if len(got) != len(want) {
		t.Fatalf("ForEachGlobalDecl() count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ForEachGlobalDecl() order[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestForEachGlobalDeclUnknownKindWithoutHandlerErrors(t *testing.T) {
	s := NewSchema()
	s.GlobalDecls = []GlobalDecl{{
		Kind: GlobalDeclKind(255),
		Name: model.QName{Namespace: "urn:test", Local: "x"},
	}}
	if err := ForEachGlobalDecl(&s.SchemaGraph, GlobalDeclHandlers{}); err == nil {
		t.Fatalf("ForEachGlobalDecl() expected error for unknown kind without handler")
	}
}

func TestForEachGlobalDeclUnknownKindUsesHandler(t *testing.T) {
	s := NewSchema()
	decl := GlobalDecl{
		Kind: GlobalDeclKind(255),
		Name: model.QName{Namespace: "urn:test", Local: "x"},
	}
	s.GlobalDecls = []GlobalDecl{decl}
	called := false
	err := ForEachGlobalDecl(&s.SchemaGraph, GlobalDeclHandlers{
		Unknown: func(kind GlobalDeclKind, name model.QName) error {
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
		t.Fatalf("ForEachGlobalDecl() error = %v", err)
	}
	if !called {
		t.Fatalf("unknown handler was not called")
	}
}

func TestForEachGlobalDeclMissingDeclarationPassesNilValue(t *testing.T) {
	s := NewSchema()
	name := model.QName{Namespace: "urn:test", Local: "missingType"}
	s.GlobalDecls = []GlobalDecl{{
		Kind: GlobalDeclType,
		Name: name,
	}}
	called := false
	err := ForEachGlobalDecl(&s.SchemaGraph, GlobalDeclHandlers{
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
		t.Fatalf("ForEachGlobalDecl() error = %v", err)
	}
	if !called {
		t.Fatalf("type handler was not called")
	}
}
