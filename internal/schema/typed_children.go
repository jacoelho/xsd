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
// xs:simpleContent or xs:complexContent named container, whose child order
// admitted exactly one.
func contentDerivationOf(n *schemaNode, container string) (contentDerivationSource, error) {
	for child := range n.xsdChildren() {
		switch child.local {
		case extensionChild:
			return contentDerivationSource{node: child, kind: contentDerivationExtension}, nil
		case restrictionChild:
			return contentDerivationSource{node: child, kind: contentDerivationRestriction}, nil
		}
	}
	return contentDerivationSource{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant(container+" child order admitted no derivation"))
}

// topLevelGroupModel selects the model group of n, a top-level xs:group,
// whose child order admitted exactly one.
func topLevelGroupModel(n *schemaNode) (*schemaNode, error) {
	for child := range n.xsdChildren() {
		switch child.local {
		case sequenceChild, choiceChild, allChild:
			return child, nil
		}
	}
	return nil, withSchemaCompileLocation(n, xsderrors.InternalInvariant("top-level group child order admitted no model group"))
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

func checkSchemaQNameParts(n *schemaNode, lexical string) (qNameParts, error) {
	parts, err := parseQNameParts(lexical)
	if err != nil {
		return qNameParts{}, withSchemaCompileLocation(n, err)
	}
	return parts, nil
}
