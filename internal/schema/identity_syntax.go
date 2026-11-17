package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"
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
		default:
			continue
		}
		if err := checkIdentityXPathChild(child); err != nil {
			return identityConstraintSyntax{}, err
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

func checkIdentityXPathChild(n *schemaNode) error {
	source := semanticSource[*schemaIdentityXPathSource](&n.semantic)
	if source == nil || !source.XPath.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaIdentity, diagnostictext.Quote(n.local)+" missing xpath")
	}
	if lex.TrimXMLWhitespaceString(source.XPath.Value) == "" {
		return schemaCompileAt(n, xsderrors.CodeSchemaIdentity, diagnostictext.Quote(n.local)+" xpath is empty")
	}
	return nil
}
