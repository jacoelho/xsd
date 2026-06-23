package compile

import (
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	selectorChild = vocab.XSDElemSelector
	fieldChild    = vocab.XSDElemField
)

// IdentityConstraintChild is the raw child syntax needed to validate an
// xs:unique, xs:key, or xs:keyref declaration without importing schema nodes.
type IdentityConstraintChild struct {
	Local    string
	XPath    string
	Children []string
	HasXPath bool
}

// IdentityConstraintSyntax identifies the selector and fields by index in the
// input slice after identity constraint child syntax has been validated.
type IdentityConstraintSyntax struct {
	Fields   []int
	Selector int
}

// IdentityConstraintSyntaxError identifies the offending identity syntax node.
// ChildIndex is -1 for the identity declaration itself. NestedChildIndex is -1
// unless the issue belongs to a child of the selector or field node.
type IdentityConstraintSyntaxError struct {
	Code             xsderrors.Code
	Message          string
	ChildIndex       int
	NestedChildIndex int
}

func (e *IdentityConstraintSyntaxError) Error() string {
	if e == nil {
		return nilErrorString
	}
	return e.Message
}

// ValidateIdentityConstraintChildren validates xs:unique/xs:key/xs:keyref
// child syntax and returns indexes for the selector and field children.
func ValidateIdentityConstraintChildren(children []IdentityConstraintChild) (IdentityConstraintSyntax, error) {
	syntax := IdentityConstraintSyntax{Selector: -1}
	seenAnnotation := false
	for i, child := range children {
		switch child.Local {
		case annotationChild:
			if seenAnnotation {
				return syntax, identitySyntaxError(i, -1, xsderrors.CodeSchemaContentModel, "identity constraint can contain at most one annotation")
			}
			if syntax.Selector >= 0 || len(syntax.Fields) != 0 {
				return syntax, identitySyntaxError(i, -1, xsderrors.CodeSchemaContentModel, "identity constraint annotation must be first")
			}
			seenAnnotation = true
		case selectorChild:
			if syntax.Selector >= 0 {
				return syntax, identitySyntaxError(i, -1, xsderrors.CodeSchemaContentModel, "identity constraint can contain at most one selector")
			}
			if len(syntax.Fields) != 0 {
				return syntax, identitySyntaxError(i, -1, xsderrors.CodeSchemaContentModel, "identity constraint selector must precede fields")
			}
			if err := validateIdentityXPathChild(i, child, selectorChild); err != nil {
				return syntax, err
			}
			syntax.Selector = i
		case fieldChild:
			if syntax.Selector < 0 {
				return syntax, identitySyntaxError(i, -1, xsderrors.CodeSchemaContentModel, "identity constraint field requires selector")
			}
			if err := validateIdentityXPathChild(i, child, fieldChild); err != nil {
				return syntax, err
			}
			syntax.Fields = append(syntax.Fields, i)
		default:
			return syntax, identitySyntaxError(i, -1, xsderrors.CodeSchemaContentModel, "invalid identity constraint child "+child.Local)
		}
	}
	if syntax.Selector < 0 {
		return syntax, identitySyntaxError(-1, -1, xsderrors.CodeSchemaIdentity, "identity constraint missing selector")
	}
	if len(syntax.Fields) == 0 {
		return syntax, identitySyntaxError(-1, -1, xsderrors.CodeSchemaIdentity, "identity constraint missing fields")
	}
	return syntax, nil
}

func validateIdentityXPathChild(index int, child IdentityConstraintChild, label string) error {
	if !child.HasXPath {
		return identitySyntaxError(index, -1, xsderrors.CodeSchemaIdentity, label+" missing xpath")
	}
	if trimIdentityXMLWhitespace(child.XPath) == "" {
		return identitySyntaxError(index, -1, xsderrors.CodeSchemaIdentity, label+" xpath is empty")
	}
	seenAnnotation := false
	for i, nested := range child.Children {
		if nested != annotationChild {
			return identitySyntaxError(index, i, xsderrors.CodeSchemaContentModel, label+" can contain only annotation")
		}
		if seenAnnotation {
			return identitySyntaxError(index, i, xsderrors.CodeSchemaContentModel, label+" can contain at most one annotation")
		}
		seenAnnotation = true
	}
	return nil
}

func identitySyntaxError(child, nested int, code xsderrors.Code, msg string) error {
	return &IdentityConstraintSyntaxError{
		Code:             code,
		Message:          msg,
		ChildIndex:       child,
		NestedChildIndex: nested,
	}
}
