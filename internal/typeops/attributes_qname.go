package typeops

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// EffectiveAttributeQName resolves an attribute's effective QName using form defaults.
func EffectiveAttributeQName(schema *parser.Schema, attr *types.AttributeDecl) types.QName {
	if attr == nil {
		return types.QName{}
	}
	if attr.IsReference {
		return attr.Name
	}
	form := attr.Form
	if form == types.FormDefault {
		if schema != nil && schema.AttributeFormDefault == parser.Qualified {
			form = types.FormQualified
		} else {
			form = types.FormUnqualified
		}
	}
	if form == types.FormQualified {
		ns := types.NamespaceEmpty
		if schema != nil {
			ns = schema.TargetNamespace
		}
		if !attr.SourceNamespace.IsEmpty() {
			ns = attr.SourceNamespace
		}
		return types.QName{Namespace: ns, Local: attr.Name.Local}
	}
	return types.QName{Namespace: types.NamespaceEmpty, Local: attr.Name.Local}
}
