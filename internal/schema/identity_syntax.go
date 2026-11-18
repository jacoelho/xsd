package schema

import (
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	selectorChild = vocab.XSDElemSelector
	fieldChild    = vocab.XSDElemField
)

// identityConstraintChildren selects the selector and fields of n, an
// identity constraint whose child order admitted one selector and at least one
// field.
func identityConstraintChildren(n *schemaNode) (identityConstraintSyntax, error) {
	var syntax identityConstraintSyntax
	for child := range n.xsdChildren() {
		switch child.local {
		case selectorChild:
			syntax.selector = child
		case fieldChild:
			syntax.fields = append(syntax.fields, child)
		}
	}
	if syntax.selector == nil || len(syntax.fields) == 0 {
		return identityConstraintSyntax{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("identity constraint child order admitted no selector or field"))
	}
	return syntax, nil
}
