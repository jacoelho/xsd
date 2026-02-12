package preprocessor

import "github.com/jacoelho/xsd/internal/objects"

// isIncludeNamespaceCompatible checks if target namespaces are compatible for include
// Rules according to XSD 1.0 spec:
// - If including schema has a target namespace, included schema must have the same namespace OR no namespace
// - If including schema has no target namespace, included schema must also have no target namespace
func (l *Loader) isIncludeNamespaceCompatible(includingNS, includedNS objects.NamespaceURI) bool {
	// same namespace - always compatible
	if includingNS == includedNS {
		return true
	}
	// including schema has namespace, included schema has no namespace - compatible
	if includingNS != "" && includedNS == "" {
		return true
	}
	// all other cases are incompatible
	return false
}
