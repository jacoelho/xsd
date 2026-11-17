package schema

import (
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	selectorChild = vocab.XSDElemSelector
	fieldChild    = vocab.XSDElemField
)

// checkIdentityConstraintChildren selects the selector and fields of n, an
// identity constraint whose child order was admitted as each child started,
// and checks what is required of them.
func checkIdentityConstraintChildren(n *schemaNode) (identityConstraintSyntax, error) {
	var syntax identityConstraintSyntax
	for child := range n.xsdChildren() {
		switch child.local {
		case selectorChild:
			syntax.selector = child
		case fieldChild:
			syntax.fields = append(syntax.fields, child)
		}
	}
	if syntax.selector == nil {
		return identityConstraintSyntax{}, schemaCompileAt(n, xsderrors.CodeSchemaIdentity, "identity constraint missing selector")
	}
	if len(syntax.fields) == 0 {
		return identityConstraintSyntax{}, schemaCompileAt(n, xsderrors.CodeSchemaIdentity, "identity constraint missing fields")
	}
	return syntax, nil
}
