package globaldecl

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// Handlers routes resolved global declarations by kind.
type Handlers struct {
	Element        func(model.QName, *model.ElementDecl) error
	Type           func(model.QName, model.Type) error
	Attribute      func(model.QName, *model.AttributeDecl) error
	AttributeGroup func(model.QName, *model.AttributeGroup) error
	Group          func(model.QName, *model.ModelGroup) error
	Notation       func(model.QName, *model.NotationDecl) error
	Unknown        func(parser.GlobalDeclKind, model.QName) error
}

// Dispatch routes one declaration to a typed callback.
func Dispatch(schema *parser.Schema, decl parser.GlobalDecl, handlers Handlers) error {
	switch decl.Kind {
	case parser.GlobalDeclElement:
		if handlers.Element == nil {
			return nil
		}
		return handlers.Element(decl.Name, schema.ElementDecls[decl.Name])
	case parser.GlobalDeclType:
		if handlers.Type == nil {
			return nil
		}
		return handlers.Type(decl.Name, schema.TypeDefs[decl.Name])
	case parser.GlobalDeclAttribute:
		if handlers.Attribute == nil {
			return nil
		}
		return handlers.Attribute(decl.Name, schema.AttributeDecls[decl.Name])
	case parser.GlobalDeclAttributeGroup:
		if handlers.AttributeGroup == nil {
			return nil
		}
		return handlers.AttributeGroup(decl.Name, schema.AttributeGroups[decl.Name])
	case parser.GlobalDeclGroup:
		if handlers.Group == nil {
			return nil
		}
		return handlers.Group(decl.Name, schema.Groups[decl.Name])
	case parser.GlobalDeclNotation:
		if handlers.Notation == nil {
			return nil
		}
		return handlers.Notation(decl.Name, schema.NotationDecls[decl.Name])
	default:
		if handlers.Unknown != nil {
			return handlers.Unknown(decl.Kind, decl.Name)
		}
		return fmt.Errorf("unknown global declaration kind %d", decl.Kind)
	}
}

// ForEach dispatches all global declarations in source order.
func ForEach(schema *parser.Schema, handlers Handlers) error {
	for _, decl := range schema.GlobalDecls {
		if err := Dispatch(schema, decl, handlers); err != nil {
			return err
		}
	}
	return nil
}

// ForEachElement dispatches global element declarations in source order.
func ForEachElement(schema *parser.Schema, fn func(model.QName, *model.ElementDecl) error) error {
	return ForEach(schema, Handlers{Element: fn})
}

// ForEachType dispatches global type declarations in source order.
func ForEachType(schema *parser.Schema, fn func(model.QName, model.Type) error) error {
	return ForEach(schema, Handlers{Type: fn})
}

// ForEachAttribute dispatches global attribute declarations in source order.
func ForEachAttribute(schema *parser.Schema, fn func(model.QName, *model.AttributeDecl) error) error {
	return ForEach(schema, Handlers{Attribute: fn})
}

// ForEachAttributeGroup dispatches global attributeGroup declarations in source order.
func ForEachAttributeGroup(schema *parser.Schema, fn func(model.QName, *model.AttributeGroup) error) error {
	return ForEach(schema, Handlers{AttributeGroup: fn})
}

// ForEachGroup dispatches global group declarations in source order.
func ForEachGroup(schema *parser.Schema, fn func(model.QName, *model.ModelGroup) error) error {
	return ForEach(schema, Handlers{Group: fn})
}

// ForEachNotation dispatches global notation declarations in source order.
func ForEachNotation(schema *parser.Schema, fn func(model.QName, *model.NotationDecl) error) error {
	return ForEach(schema, Handlers{Notation: fn})
}
