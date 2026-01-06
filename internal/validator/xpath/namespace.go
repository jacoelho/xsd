package xpath

import (
	"strings"

	"github.com/jacoelho/xsd/internal/xml"
)

// NormalizeQName normalizes a QName value by resolving the namespace prefix to a URI.
// Returns the normalized form "{namespaceURI}local" for comparison.
func (e *Evaluator) NormalizeQName(value string, elem xml.Element) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	prefix := ""
	local := value
	if before, after, ok := strings.Cut(value, ":"); ok {
		prefix = before
		local = after
	}

	namespaceURI := e.resolveNamespacePrefix(prefix, elem)

	// Return normalized form: {namespaceURI}local.
	return "{" + namespaceURI + "}" + local
}

// resolveNamespacePrefix resolves a namespace prefix to a URI by walking up the element tree.
func (e *Evaluator) resolveNamespacePrefix(prefix string, elem xml.Element) string {
	// First check the element itself for namespace declarations.
	if ns := checkElementNamespace(prefix, elem); ns != "" {
		return ns
	}

	if e.root == nil {
		return ""
	}

	nsMap := make(map[string]string)
	e.collectNamespaces(e.root, elem, nsMap)

	if ns, ok := nsMap[prefix]; ok {
		return ns
	}

	// If prefix is empty, return empty namespace.
	if prefix == "" {
		return ""
	}

	// If not found, return empty (invalid QName).
	return ""
}

// checkElementNamespace checks namespace declarations on a single element.
func checkElementNamespace(prefix string, elem xml.Element) string {
	for _, attr := range elem.Attributes() {
		attrNS := attr.NamespaceURI()
		attrLocal := attr.LocalName()
		attrName := attr.Name()

		// Check for xmlns namespace (http://www.w3.org/2000/xmlns/ or "xmlns").
		if attrNS == "http://www.w3.org/2000/xmlns/" || attrNS == "xmlns" {
			if attrLocal == prefix {
				return attr.Value()
			}
		}
		if prefix == "" && attrName == "xmlns" {
			return attr.Value()
		}
		// Check for xmlns:prefix format.
		if prefix != "" && attrName == "xmlns:"+prefix {
			return attr.Value()
		}
	}
	return ""
}

// collectNamespaces collects namespace declarations along the path from root to target element.
func (e *Evaluator) collectNamespaces(root, target xml.Element, nsMap map[string]string) bool {
	// xmlns attributes can be stored in two ways:
	// 1. As attributes with namespace "http://www.w3.org/2000/xmlns/" and local name = prefix.
	// 2. As attributes with name "xmlns" or "xmlns:prefix".
	for _, attr := range root.Attributes() {
		attrNS := attr.NamespaceURI()
		attrLocal := attr.LocalName()
		attrName := attr.Name()

		// Check for xmlns namespace (http://www.w3.org/2000/xmlns/).
		if attrNS == "http://www.w3.org/2000/xmlns/" {
			// Prefixed namespace declaration: xmlns:prefix="uri".
			nsMap[attrLocal] = attr.Value()
		} else if attrNS == "xmlns" {
			// Alternative: namespace is stored as "xmlns" string.
			nsMap[attrLocal] = attr.Value()
		} else if attrName == "xmlns" {
			// Default namespace: xmlns="uri".
			nsMap[""] = attr.Value()
		} else if strings.HasPrefix(attrName, "xmlns:") {
			// Prefixed namespace: xmlns:prefix="uri".
			prefix := attrName[6:] // Skip "xmlns:"
			nsMap[prefix] = attr.Value()
		}
	}

	// If this is the target element, we're done.
	if root == target {
		return true
	}

	// Recursively search children.
	for _, child := range root.Children() {
		if e.collectNamespaces(child, target, nsMap) {
			return true
		}
	}

	return false
}
