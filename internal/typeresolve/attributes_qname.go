package typeresolve

import (
	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
)

// EffectiveAttributeQName resolves an attribute's effective QName using form defaults.
func EffectiveAttributeQName(schema *parser.Schema, attr *model.AttributeDecl) model.QName {
	if attr == nil {
		return model.QName{}
	}
	if attr.IsReference {
		return attr.Name
	}
	form := attr.Form
	if form == model.FormDefault {
		if schema != nil && schema.AttributeFormDefault == parser.Qualified {
			form = model.FormQualified
		} else {
			form = model.FormUnqualified
		}
	}
	if form == model.FormQualified {
		ns := model.NamespaceEmpty
		if schema != nil {
			ns = schema.TargetNamespace
		}
		if attr.SourceNamespace != "" {
			ns = attr.SourceNamespace
		}
		return model.QName{Namespace: ns, Local: attr.Name.Local}
	}
	return model.QName{Namespace: model.NamespaceEmpty, Local: attr.Name.Local}
}
