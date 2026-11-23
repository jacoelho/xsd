package schema

import "github.com/jacoelho/xsd/internal/diagnostictext"

// WildcardDescription describes the elements a wildcard expectation admits.
// It is the one wording of a wildcard's namespace constraint in schema and
// instance diagnostics; the absent namespace is written "".
func (e ContentExpectation) WildcardDescription() string {
	switch e.WildcardMode {
	case WildcardAny:
		return "any element"
	case WildcardLocal:
		return "an element with no namespace"
	case WildcardOther:
		return "an element in a non-empty namespace other than " + diagnostictext.Quote(e.ExcludedNamespace)
	case WildcardTargetNamespace, WildcardList:
		if e.MoreNamespaces {
			return "an element in namespace " + diagnostictext.QuoteTruncatedList(e.Namespaces)
		}
		return "an element in namespace " + diagnostictext.QuoteList(e.Namespaces)
	default:
		return "an element matching the namespace wildcard"
	}
}

// wildcardExpectation returns the expectation of an admitted wildcard, holding
// at most diagnostictext.MaxAlternatives of its namespaces, the first in text
// order. Admitted namespace lists are in ID order, so the shown namespaces are
// selected without sorting or copying the whole list.
func wildcardExpectation(names *nameTable, wildcard wildcardRuntime) ContentExpectation {
	expectation := ContentExpectation{IsWildcard: true, WildcardMode: wildcard.Mode}
	switch wildcard.Mode {
	case WildcardOther:
		expectation.ExcludedNamespace = names.Namespace(wildcard.OtherThan)
	case WildcardTargetNamespace, WildcardList:
		expectation.Namespaces, expectation.MoreNamespaces = firstNamespacesInTextOrder(names, wildcard.Namespaces)
	case WildcardAny, WildcardLocal:
	}
	return expectation
}

func firstNamespacesInTextOrder(names *nameTable, ids []namespaceID) ([]string, bool) {
	shown := make([]string, 0, min(len(ids), diagnostictext.MaxAlternatives))
	for _, id := range ids {
		uri := names.Namespace(id)
		at := len(shown)
		for at > 0 && compareNamespaceURI(uri, shown[at-1]) < 0 {
			at--
		}
		if at == diagnostictext.MaxAlternatives {
			continue
		}
		if len(shown) < diagnostictext.MaxAlternatives {
			shown = append(shown, "")
		}
		copy(shown[at+1:], shown[at:len(shown)-1])
		shown[at] = uri
	}
	return shown, len(shown) < len(ids)
}
