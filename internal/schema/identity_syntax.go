package schema

import (
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	selectorChild = vocab.XSDElemSelector
	fieldChild    = vocab.XSDElemField
)

//nolint:gocognit // One ordered traversal preserves child-error precedence and selects the validated source nodes.
func checkIdentityConstraintChildren(n *schemaNode) (identityConstraintSyntax, error) {
	var syntax identityConstraintSyntax
	seenAnnotation := false
	for child := range n.xsdChildren() {
		switch child.local {
		case annotationChild:
			if seenAnnotation {
				return identityConstraintSyntax{}, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "identity constraint can contain at most one annotation")
			}
			if syntax.selector != nil || len(syntax.fields) != 0 {
				return identityConstraintSyntax{}, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "identity constraint annotation must be first")
			}
			seenAnnotation = true
		case selectorChild:
			if syntax.selector != nil {
				return identityConstraintSyntax{}, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "identity constraint can contain at most one selector")
			}
			if err := checkIdentityXPathChild(child); err != nil {
				return identityConstraintSyntax{}, err
			}
			syntax.selector = child
		case fieldChild:
			if syntax.selector == nil {
				return identityConstraintSyntax{}, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "identity constraint field requires selector")
			}
			if err := checkIdentityXPathChild(child); err != nil {
				return identityConstraintSyntax{}, err
			}
			syntax.fields = append(syntax.fields, child)
		default:
			return identityConstraintSyntax{}, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "invalid identity constraint child "+child.local)
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
		return schemaCompileAt(n, xsderrors.CodeSchemaIdentity, n.local+" missing xpath")
	}
	if lex.TrimXMLWhitespaceString(source.XPath.Value) == "" {
		return schemaCompileAt(n, xsderrors.CodeSchemaIdentity, n.local+" xpath is empty")
	}
	seenAnnotation := false
	for child := range n.xsdChildren() {
		if child.local != annotationChild {
			return schemaCompileAt(child, xsderrors.CodeSchemaContentModel, n.local+" can contain only annotation")
		}
		if seenAnnotation {
			return schemaCompileAt(child, xsderrors.CodeSchemaContentModel, n.local+" can contain at most one annotation")
		}
		seenAnnotation = true
	}
	return nil
}
