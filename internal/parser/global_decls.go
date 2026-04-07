package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

// GlobalDeclHandlers routes resolved global declarations by kind.
type GlobalDeclHandlers struct {
	Element        func(model.QName, *model.ElementDecl) error
	Type           func(model.QName, model.Type) error
	Attribute      func(model.QName, *model.AttributeDecl) error
	AttributeGroup func(model.QName, *model.AttributeGroup) error
	Group          func(model.QName, *model.ModelGroup) error
	Notation       func(model.QName, *model.NotationDecl) error
	Unknown        func(GlobalDeclKind, model.QName) error
}

func callDeclHandler[T any](fn func(model.QName, T) error, name model.QName, value T) error {
	if fn == nil {
		return nil
	}
	return fn(name, value)
}

func dispatchGlobalDecl(graph *SchemaGraph, decl GlobalDecl, handlers GlobalDeclHandlers) error {
	switch decl.Kind {
	case GlobalDeclElement:
		return callDeclHandler(handlers.Element, decl.Name, graph.ElementDecls[decl.Name])
	case GlobalDeclType:
		return callDeclHandler(handlers.Type, decl.Name, graph.TypeDefs[decl.Name])
	case GlobalDeclAttribute:
		return callDeclHandler(handlers.Attribute, decl.Name, graph.AttributeDecls[decl.Name])
	case GlobalDeclAttributeGroup:
		return callDeclHandler(handlers.AttributeGroup, decl.Name, graph.AttributeGroups[decl.Name])
	case GlobalDeclGroup:
		return callDeclHandler(handlers.Group, decl.Name, graph.Groups[decl.Name])
	case GlobalDeclNotation:
		return callDeclHandler(handlers.Notation, decl.Name, graph.NotationDecls[decl.Name])
	default:
		if handlers.Unknown != nil {
			return handlers.Unknown(decl.Kind, decl.Name)
		}
		return fmt.Errorf("unknown global declaration kind %d", decl.Kind)
	}
}

// ForEachGlobalDecl dispatches all global declarations in source order.
func ForEachGlobalDecl(graph *SchemaGraph, handlers GlobalDeclHandlers) error {
	for _, decl := range graph.GlobalDecls {
		if err := dispatchGlobalDecl(graph, decl, handlers); err != nil {
			return err
		}
	}
	return nil
}
