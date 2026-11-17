package schema

// This file contains the compiler-facing grammar checks. Parser admission
// uses the *Syntax variants in compile_children.go; compilation runs against
// compact semantic nodes and does not reconstruct XML nodes.

import (
	"errors"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/xsderrors"
)

type contentDerivationSource struct {
	node *schemaNode
	kind contentDerivationKind
}

func checkOrderedXSDChildren(n *schemaNode, order childOrder) error {
	state := childOrderState{maxLevelSeen: -1}
	index := 0
	for child := range n.xsdChildren() {
		if err := state.accept(index, child.local, order); err != nil {
			issue, ok := errors.AsType[*childOrderError](err)
			if !ok {
				return err
			}
			target := child
			if issue.Index < 0 {
				target = n
			}
			return schemaCompileAt(target, xsderrors.CodeSchemaContentModel, issue.Message)
		}
		index++
	}
	return nil
}

func checkComplexTypeChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, complexTypeChildOrder)
}

func checkComplexContentSyntax(n *schemaNode) (contentDerivationSource, error) {
	return checkContentDerivation(n, complexContent, complexContentChildOrder)
}

func checkSimpleContentSyntax(n *schemaNode) (contentDerivationSource, error) {
	return checkContentDerivation(n, simpleContent, simpleContentChildOrder)
}

func checkContentDerivation(n *schemaNode, container string, order childOrder) (contentDerivationSource, error) {
	if err := checkOrderedXSDChildren(n, order); err != nil {
		return contentDerivationSource{}, err
	}
	for child := range n.xsdChildren() {
		switch child.local {
		case extensionChild:
			return contentDerivationSource{node: child, kind: contentDerivationExtension}, nil
		case restrictionChild:
			return contentDerivationSource{node: child, kind: contentDerivationRestriction}, nil
		}
	}
	return contentDerivationSource{}, schemaCompileAt(n, xsderrors.CodeSchemaContentModel, container+" missing extension or restriction")
}

func checkSimpleContentRestrictionChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, simpleContentRestrictionChildOrder)
}

func checkSimpleContentExtensionChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, simpleContentExtensionChildOrder)
}

func checkComplexContentRestrictionChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, complexContentRestrictionChildOrder)
}

func checkComplexContentExtensionChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, complexContentExtensionChildOrder)
}

func checkAttributeGroupDeclarationChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, attributeGroupDeclarationChildOrder)
}

func checkTopLevelGroupChildren(n *schemaNode) (*schemaNode, error) {
	var model *schemaNode
	for child := range n.xsdChildren() {
		switch child.local {
		case annotationChild:
		case sequenceChild, choiceChild, allChild:
			if model != nil {
				return nil, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "top-level group must contain exactly one model group")
			}
			model = child
		case groupChild:
			return nil, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "top-level group cannot contain group ref")
		default:
			return nil, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "invalid top-level group child "+diagnostictext.Quote(child.local))
		}
	}
	if model == nil {
		return nil, schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "top-level group must contain exactly one model group")
	}
	return model, nil
}

func checkNotationDeclaration(n *schemaNode) error {
	source, ok := n.semantic.kind.(*schemaNotationSource)
	if !ok || source == nil {
		return withSchemaCompileLocation(n, xsderrors.InternalInvariant("notation node has no typed notation source"))
	}
	if !source.Global.Name.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "notation missing name")
	}
	if !source.Public.Present && !source.System.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "notation requires public or system")
	}
	return nil
}

func checkAttributeDeclarationChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, attributeDeclarationChildOrder)
}

func checkElementDeclarationChildren(n *schemaNode) error {
	if err := checkOrderedXSDChildren(n, elementDeclarationChildOrder); err != nil {
		return err
	}
	if semanticSource[*schemaElementSource](&n.semantic).Type.Lexical.Present && hasAnonymousTypedElementType(n) {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "element cannot have both type and anonymous type")
	}
	return nil
}

func hasAnonymousTypedElementType(n *schemaNode) bool {
	for child := range n.xsdChildren() {
		if child.local == simpleTypeChild || child.local == complexTypeChild {
			return true
		}
	}
	return false
}

func checkSchemaQNameParts(n *schemaNode, lexical string) (qNameParts, error) {
	parts, err := parseQNameParts(lexical)
	if err != nil {
		return qNameParts{}, withSchemaCompileLocation(n, err)
	}
	return parts, nil
}
