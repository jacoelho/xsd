package xpath

import (
	"strings"

	"github.com/jacoelho/xsd/internal/xml"
)

// Evaluator selects XML elements using the XPath subset allowed in XSD 1.0 identity constraints.
type Evaluator struct {
	root xml.Element
}

// New creates a selector bound to a document root for namespace resolution.
func New(root xml.Element) *Evaluator {
	return &Evaluator{root: root}
}

// Select evaluates an XPath selector without namespace context.
func (e *Evaluator) Select(root xml.Element, expr string) []xml.Element {
	return e.selectInternal(root, expr, nil)
}

// SelectWithNS evaluates an XPath selector with a namespace context.
func (e *Evaluator) SelectWithNS(root xml.Element, expr string, nsContext map[string]string) []xml.Element {
	return e.selectInternal(root, expr, nsContext)
}

func (e *Evaluator) selectInternal(root xml.Element, expr string, nsContext map[string]string) []xml.Element {
	// Handle "." (current element)
	if expr == "." || expr == "" {
		return []xml.Element{root}
	}

	// Handle XPath union expressions (path1|path2|path3)
	// Evaluate each path and combine results, removing duplicates.
	if strings.Contains(expr, "|") {
		parts := strings.Split(expr, "|")
		seen := make(map[xml.Element]bool)
		results := make([]xml.Element, 0)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			partResults := e.selectInternal(root, part, nsContext)
			for _, elem := range partResults {
				if !seen[elem] {
					seen[elem] = true
					results = append(results, elem)
				}
			}
		}
		return results
	}

	normalized := strings.TrimSpace(expr)

	// Handle multi-step paths (e.g., ".//p:keys/p:a/*/p:b").
	if strings.Contains(normalized, "/") && normalized != "." {
		return e.evaluateMultiStepPathNS(root, normalized, nsContext)
	}

	// Handle "child::*" or just "*" (all direct children).
	if normalized == "child::*" || normalized == "*" {
		return root.Children()
	}

	// Handle "descendant::*" or "//*" (all descendants).
	if normalized == "descendant::*" || normalized == "//*" {
		return e.collectAllDescendants(root, nil)
	}

	// Handle "descendant-or-self::*" (element and all descendants).
	if normalized == "descendant-or-self::*" {
		results := []xml.Element{root}
		return e.collectAllDescendants(root, results)
	}

	elementName, axis := parseXPathPattern(normalized)
	localName, targetNSURI, namespaceSpecified, ok := resolveXPathName(elementName, nsContext)
	if !ok {
		return nil
	}

	var results []xml.Element
	switch axis {
	case "child":
		// Direct children only.
		results = e.collectMatchingChildrenNS(root, localName, targetNSURI, namespaceSpecified, results)
	case "descendant":
		// All descendants.
		results = e.collectMatchingDescendantsNS(root, localName, targetNSURI, namespaceSpecified, results)
	case "descendant-or-self":
		// Element itself and all descendants.
		if e.matchesElementNS(root, localName, targetNSURI, namespaceSpecified) {
			results = append(results, root)
		}
		results = e.collectMatchingDescendantsNS(root, localName, targetNSURI, namespaceSpecified, results)
	case "self":
		if e.matchesElementNS(root, localName, targetNSURI, namespaceSpecified) {
			results = append(results, root)
		}
	default:
		// Default to child axis for backward compatibility.
		results = e.collectMatchingChildrenNS(root, localName, targetNSURI, namespaceSpecified, results)
	}

	return results
}

// evaluateMultiStepPathNS evaluates a multi-step XPath path with namespace context.
func (e *Evaluator) evaluateMultiStepPathNS(root xml.Element, expr string, nsContext map[string]string) []xml.Element {
	steps := splitXPathSteps(expr)
	if len(steps) == 0 {
		return []xml.Element{root}
	}

	currentElements := []xml.Element{root}

	// Evaluate each step relative to the results of the previous step.
	for _, step := range steps {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}

		// Evaluate this step for all current elements.
		nextElements := make([]xml.Element, 0)
		seen := make(map[xml.Element]bool)
		for _, elem := range currentElements {
			stepResults := e.evaluateSingleStepNS(elem, step, nsContext)
			for _, result := range stepResults {
				if !seen[result] {
					seen[result] = true
					nextElements = append(nextElements, result)
				}
			}
		}
		currentElements = nextElements
	}

	return currentElements
}

// splitXPathSteps splits an XPath into individual steps, handling axes like .// and //.
func splitXPathSteps(expr string) []string {
	var steps []string
	var current strings.Builder
	i := 0

	for i < len(expr) {
		// Handle ".//" (descendant-or-self axis) - keep it with the following element name.
		if i+2 < len(expr) && expr[i:i+3] == ".//" {
			if current.Len() > 0 {
				steps = append(steps, current.String())
				current.Reset()
			}
			current.WriteString(".//")
			i += 3
			// Continue to read the element name that follows.
			continue
		}

		// Handle "//" (descendant axis) - keep it with the following element name.
		if i+1 < len(expr) && expr[i:i+2] == "//" {
			if current.Len() > 0 {
				steps = append(steps, current.String())
				current.Reset()
			}
			current.WriteString("//")
			i += 2
			// Continue to read the element name that follows.
			continue
		}

		// Handle "/" (child axis separator).
		if expr[i] == '/' {
			if current.Len() > 0 {
				steps = append(steps, current.String())
				current.Reset()
			}
			i++
			continue
		}

		// Regular character.
		current.WriteByte(expr[i])
		i++
	}

	if current.Len() > 0 {
		steps = append(steps, current.String())
	}

	return steps
}

// evaluateSingleStepNS evaluates a single XPath step with namespace context.
func (e *Evaluator) evaluateSingleStepNS(elem xml.Element, step string, nsContext map[string]string) []xml.Element {
	step = strings.TrimSpace(step)
	if step == "" {
		return []xml.Element{elem}
	}

	// Handle "." (current element).
	if step == "." {
		return []xml.Element{elem}
	}

	// Handle "*" (wildcard - all children).
	if step == "*" {
		return elem.Children()
	}

	// Handle step with axis prefix (e.g., ".//p:keys", "//p:keys").
	var axis string
	var elementPart string
	if strings.HasPrefix(step, ".//") {
		axis = "descendant-or-self"
		elementPart = step[3:]
	} else if strings.HasPrefix(step, "//") {
		axis = "descendant"
		elementPart = step[2:]
	} else {
		axis = "child"
		elementPart = step
	}

	// Extract element name from element part (handle namespace prefixes).
	elementName, _ := parseXPathPattern(elementPart)
	if elementPart == "" || elementPart == "*" {
		// No element name specified - return all descendants/children.
		switch axis {
		case "descendant-or-self":
			results := []xml.Element{elem}
			return e.collectAllDescendants(elem, results)
		case "descendant":
			return e.collectAllDescendants(elem, nil)
		default:
			return elem.Children()
		}
	}

	localName, targetNSURI, namespaceSpecified, ok := resolveXPathName(elementName, nsContext)
	if !ok {
		return nil
	}

	var results []xml.Element
	switch axis {
	case "child":
		results = e.collectMatchingChildrenNS(elem, localName, targetNSURI, namespaceSpecified, results)
	case "descendant":
		results = e.collectMatchingDescendantsNS(elem, localName, targetNSURI, namespaceSpecified, results)
	case "descendant-or-self":
		// Check if current element matches.
		if e.matchesElementNS(elem, localName, targetNSURI, namespaceSpecified) {
			results = append(results, elem)
		}
		results = e.collectMatchingDescendantsNS(elem, localName, targetNSURI, namespaceSpecified, results)
	default:
		// Default to child axis.
		results = e.collectMatchingChildrenNS(elem, localName, targetNSURI, namespaceSpecified, results)
	}

	return results
}

// parseXPathPattern extracts element name and axis from an XPath pattern.
func parseXPathPattern(expr string) (elementName string, axis string) {
	var namePart string
	var axisPart string

	if strings.HasPrefix(expr, "child::") {
		namePart = expr[7:]
		axisPart = "child"
	} else if strings.HasPrefix(expr, "descendant::") {
		namePart = expr[12:]
		axisPart = "descendant"
	} else if strings.HasPrefix(expr, "descendant-or-self::") {
		namePart = expr[19:]
		axisPart = "descendant-or-self"
	} else if strings.HasPrefix(expr, "//") {
		namePart = expr[2:]
		axisPart = "descendant"
	} else if strings.HasPrefix(expr, "self::") {
		namePart = expr[6:]
		axisPart = "self"
	} else {
		namePart = expr
		axisPart = "child"
	}
	return namePart, axisPart
}

func resolveXPathName(name string, nsContext map[string]string) (localName, targetNSURI string, namespaceSpecified bool, ok bool) {
	if name == "*" {
		return "*", "", false, true
	}

	prefix, localName, hasPrefix := SplitQName(name)
	if hasPrefix {
		if nsContext == nil {
			return localName, "", true, false
		}
		nsURI, found := nsContext[prefix]
		if !found {
			return localName, "", true, false
		}
		return localName, nsURI, true, true
	}

	if nsContext != nil {
		return localName, "", true, true
	}
	return localName, "", false, true
}

// SplitQName splits a QName into prefix and local parts.
func SplitQName(name string) (prefix, local string, hasPrefix bool) {
	if idx := strings.Index(name, ":"); idx >= 0 && idx < len(name)-1 {
		return name[:idx], name[idx+1:], true
	}
	return "", name, false
}

// collectMatchingChildrenNS collects direct children matching the element name and namespace.
func (e *Evaluator) collectMatchingChildrenNS(elem xml.Element, localName, targetNSURI string, namespaceSpecified bool, results []xml.Element) []xml.Element {
	for _, child := range elem.Children() {
		if localName == "*" || e.matchesElementNS(child, localName, targetNSURI, namespaceSpecified) {
			results = append(results, child)
		}
	}
	return results
}

// collectMatchingDescendantsNS collects all descendants matching the element name and namespace.
func (e *Evaluator) collectMatchingDescendantsNS(elem xml.Element, localName, targetNSURI string, namespaceSpecified bool, results []xml.Element) []xml.Element {
	for _, child := range elem.Children() {
		if localName == "*" || e.matchesElementNS(child, localName, targetNSURI, namespaceSpecified) {
			results = append(results, child)
		}
		results = e.collectMatchingDescendantsNS(child, localName, targetNSURI, namespaceSpecified, results)
	}
	return results
}

// collectAllDescendants collects all descendant elements.
func (e *Evaluator) collectAllDescendants(elem xml.Element, results []xml.Element) []xml.Element {
	for _, child := range elem.Children() {
		results = append(results, child)
		results = e.collectAllDescendants(child, results)
	}
	return results
}

// matchesElementNS checks if an element matches the given local name and namespace URI.
func (e *Evaluator) matchesElementNS(elem xml.Element, localName, targetNSURI string, namespaceSpecified bool) bool {
	if localName != "*" && elem.LocalName() != localName {
		return false
	}
	if namespaceSpecified && elem.NamespaceURI() != targetNSURI {
		return false
	}
	return true
}
