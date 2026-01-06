package validator

import (
	"github.com/jacoelho/xsd/internal/grammar"
	internal "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// resolveElementQName resolves an element's QName from the XML instance.
// Uses the element's actual namespace from the XML, not what elementFormDefault would suggest.
// Local elements with form="unqualified" are indexed with empty namespace, so we need to
// use the actual namespace from the XML to match them correctly.
func (r *validationRun) resolveElementQName(elem xml.Element) types.QName {
	return types.QName{
		Namespace: types.NamespaceURI(elem.NamespaceURI()),
		Local:     elem.LocalName(),
	}
}

// effectiveElementQName returns the QName that should be used for matching
// an element in an XML instance, taking elementFormDefault into account.
func (r *validationRun) effectiveElementQName(elem *grammar.CompiledElement) types.QName {
	if elem.Original == nil {
		return elem.QName
	}

	switch elem.Original.Form {
	case types.FormQualified:
		return elem.QName
	case types.FormUnqualified:
		return types.QName{Namespace: "", Local: elem.QName.Local}
	default: // FormDefault - use schema's elementFormDefault
		if r.validator.grammar.ElementFormDefault == internal.Qualified {
			return elem.QName
		}
		return types.QName{Namespace: "", Local: elem.QName.Local}
	}
}
