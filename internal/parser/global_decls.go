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

// DispatchGlobalDecl routes one declaration from a schema graph to a typed callback.
func DispatchGlobalDecl(graph *SchemaGraph, decl GlobalDecl, handlers GlobalDeclHandlers) error {
	switch decl.Kind {
	case GlobalDeclElement:
		if handlers.Element == nil {
			return nil
		}
		return handlers.Element(decl.Name, graph.ElementDecls[decl.Name])
	case GlobalDeclType:
		if handlers.Type == nil {
			return nil
		}
		return handlers.Type(decl.Name, graph.TypeDefs[decl.Name])
	case GlobalDeclAttribute:
		if handlers.Attribute == nil {
			return nil
		}
		return handlers.Attribute(decl.Name, graph.AttributeDecls[decl.Name])
	case GlobalDeclAttributeGroup:
		if handlers.AttributeGroup == nil {
			return nil
		}
		return handlers.AttributeGroup(decl.Name, graph.AttributeGroups[decl.Name])
	case GlobalDeclGroup:
		if handlers.Group == nil {
			return nil
		}
		return handlers.Group(decl.Name, graph.Groups[decl.Name])
	case GlobalDeclNotation:
		if handlers.Notation == nil {
			return nil
		}
		return handlers.Notation(decl.Name, graph.NotationDecls[decl.Name])
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
		if err := DispatchGlobalDecl(graph, decl, handlers); err != nil {
			return err
		}
	}
	return nil
}

// ForEachGlobalElement dispatches global element declarations in source order.
func ForEachGlobalElement(graph *SchemaGraph, fn func(model.QName, *model.ElementDecl) error) error {
	return ForEachGlobalDecl(graph, GlobalDeclHandlers{Element: fn})
}

// ForEachGlobalType dispatches global type declarations in source order.
func ForEachGlobalType(graph *SchemaGraph, fn func(model.QName, model.Type) error) error {
	return ForEachGlobalDecl(graph, GlobalDeclHandlers{Type: fn})
}

// ForEachGlobalAttribute dispatches global attribute declarations in source order.
func ForEachGlobalAttribute(graph *SchemaGraph, fn func(model.QName, *model.AttributeDecl) error) error {
	return ForEachGlobalDecl(graph, GlobalDeclHandlers{Attribute: fn})
}

// ForEachGlobalAttributeGroup dispatches global attributeGroup declarations in source order.
func ForEachGlobalAttributeGroup(graph *SchemaGraph, fn func(model.QName, *model.AttributeGroup) error) error {
	return ForEachGlobalDecl(graph, GlobalDeclHandlers{AttributeGroup: fn})
}

// ForEachGlobalGroup dispatches global group declarations in source order.
func ForEachGlobalGroup(graph *SchemaGraph, fn func(model.QName, *model.ModelGroup) error) error {
	return ForEachGlobalDecl(graph, GlobalDeclHandlers{Group: fn})
}

// ForEachGlobalNotation dispatches global notation declarations in source order.
func ForEachGlobalNotation(graph *SchemaGraph, fn func(model.QName, *model.NotationDecl) error) error {
	return ForEachGlobalDecl(graph, GlobalDeclHandlers{Notation: fn})
}
