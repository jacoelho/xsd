package schema

// This file contains the compiler-facing child selection of admitted source
// nodes. Which children each element admits, in which order and at most how
// often, is decided as each child starts, and which it requires at its end
// tag; see childOrderFor.

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
	return contentDerivationSource{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant(container+" child order admitted no derivation", nil))
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
	return nil, withSchemaCompileLocation(n, xsderrors.InternalInvariant("top-level group child order admitted no model group", nil))
}

func checkSchemaQNameParts(n *schemaNode, lexical string) (qNameParts, error) {
	parts, err := parseQNameParts(lexical)
	if err != nil {
		return qNameParts{}, withSchemaCompileLocation(n, err)
	}
	return parts, nil
}
