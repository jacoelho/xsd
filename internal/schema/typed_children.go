package schema

// This file contains the compiler-facing child selection of admitted source
// nodes. Which children each element admits, in which order and at most how
// often, is decided as each child starts; see childOrderFor.

import (
	"github.com/jacoelho/xsd/xsderrors"
)

type contentDerivationSource struct {
	node *schemaNode
	kind contentDerivationKind
}

// contentDerivationOf selects the extension or restriction child of n, an
// xs:simpleContent or xs:complexContent named container. childOrderFor admits
// at most one; a missing one is reported here.
func contentDerivationOf(n *schemaNode, container string) (contentDerivationSource, error) {
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

// topLevelGroupModel selects the model group of n, a top-level xs:group.
// childOrderFor admits at most one; a missing one is reported here.
func topLevelGroupModel(n *schemaNode) (*schemaNode, error) {
	for child := range n.xsdChildren() {
		switch child.local {
		case sequenceChild, choiceChild, allChild:
			return child, nil
		}
	}
	return nil, schemaCompileAt(n, xsderrors.CodeSchemaContentModel, topLevelGroupModelGroupCount)
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

func checkElementTypeSource(n *schemaNode) error {
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
