package xpath

import (
	"strings"

	"github.com/jacoelho/xsd/internal/xml"
)

// Evaluator selects XML elements using the XPath subset allowed in XSD 1.0 identity constraints.
type Evaluator struct {
	doc  *xml.Document
	root xml.NodeID
}

// New creates a selector bound to a document root for namespace resolution.
func New(doc *xml.Document, root xml.NodeID) *Evaluator {
	return &Evaluator{doc: doc, root: root}
}

// Select evaluates an XPath selector without namespace context.
func (e *Evaluator) Select(root xml.NodeID, expr string) []xml.NodeID {
	return e.selectInternal(root, expr, nil)
}

// SelectWithNS evaluates an XPath selector with a namespace context.
func (e *Evaluator) SelectWithNS(root xml.NodeID, expr string, nsContext map[string]string) []xml.NodeID {
	return e.selectInternal(root, expr, nsContext)
}

func (e *Evaluator) selectInternal(root xml.NodeID, expr string, nsContext map[string]string) []xml.NodeID {
	if expr == "." || expr == "" {
		return []xml.NodeID{root}
	}

	// handle XPath union expressions (path1|path2|path3)
	// evaluate each path and combine results, removing duplicates
	if strings.Contains(expr, "|") {
		seen := make(map[xml.NodeID]bool)
		results := make([]xml.NodeID, 0)
		for part := range strings.SplitSeq(expr, "|") {
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

	if strings.Contains(normalized, "/") && normalized != "." {
		return e.evaluateMultiStepPathNS(root, normalized, nsContext)
	}

	if normalized == "child::*" || normalized == "*" {
		return e.doc.Children(root)
	}

	if normalized == "descendant::*" || normalized == "//*" {
		return e.collectAllDescendants(root, nil)
	}

	if normalized == "descendant-or-self::*" {
		results := []xml.NodeID{root}
		return e.collectAllDescendants(root, results)
	}

	elementName, axis := parseXPathPattern(normalized)
	localName, targetNSURI, namespaceSpecified, ok := resolveXPathName(elementName, nsContext)
	if !ok {
		return nil
	}

	var results []xml.NodeID
	switch axis {
	case "child":
		results = e.collectMatchingChildrenNS(root, localName, targetNSURI, namespaceSpecified, results)
	case "descendant":
		results = e.collectMatchingDescendantsNS(root, localName, targetNSURI, namespaceSpecified, results)
	case "descendant-or-self":
		if e.matchesElementNS(root, localName, targetNSURI, namespaceSpecified) {
			results = append(results, root)
		}
		results = e.collectMatchingDescendantsNS(root, localName, targetNSURI, namespaceSpecified, results)
	case "self":
		if e.matchesElementNS(root, localName, targetNSURI, namespaceSpecified) {
			results = append(results, root)
		}
	default:
		// default to child axis for backward compatibility
		results = e.collectMatchingChildrenNS(root, localName, targetNSURI, namespaceSpecified, results)
	}

	return results
}

// evaluateMultiStepPathNS evaluates a multi-step XPath path with namespace context.
func (e *Evaluator) evaluateMultiStepPathNS(root xml.NodeID, expr string, nsContext map[string]string) []xml.NodeID {
	steps := splitXPathSteps(expr)
	if len(steps) == 0 {
		return []xml.NodeID{root}
	}

	currentElements := []xml.NodeID{root}

	for _, step := range steps {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}

		nextElements := make([]xml.NodeID, 0)
		seen := make(map[xml.NodeID]bool)
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
		// handle ".//" (descendant-or-self axis) - keep it with the following element name
		if i+2 < len(expr) && expr[i:i+3] == ".//" {
			if current.Len() > 0 {
				steps = append(steps, current.String())
				current.Reset()
			}
			current.WriteString(".//")
			i += 3
			continue
		}

		// handle "//" (descendant axis) - keep it with the following element name
		if i+1 < len(expr) && expr[i:i+2] == "//" {
			if current.Len() > 0 {
				steps = append(steps, current.String())
				current.Reset()
			}
			current.WriteString("//")
			i += 2
			continue
		}

		if expr[i] == '/' {
			if current.Len() > 0 {
				steps = append(steps, current.String())
				current.Reset()
			}
			i++
			continue
		}

		current.WriteByte(expr[i])
		i++
	}

	if current.Len() > 0 {
		steps = append(steps, current.String())
	}

	return steps
}

// evaluateSingleStepNS evaluates a single XPath step with namespace context.
func (e *Evaluator) evaluateSingleStepNS(elem xml.NodeID, step string, nsContext map[string]string) []xml.NodeID {
	step = strings.TrimSpace(step)
	if step == "" {
		return []xml.NodeID{elem}
	}

	if step == "." {
		return []xml.NodeID{elem}
	}

	if step == "*" {
		return e.doc.Children(elem)
	}

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

	elementName, _ := parseXPathPattern(elementPart)
	if elementPart == "" || elementPart == "*" {
		switch axis {
		case "descendant-or-self":
			results := []xml.NodeID{elem}
			return e.collectAllDescendants(elem, results)
		case "descendant":
			return e.collectAllDescendants(elem, nil)
		default:
			return e.doc.Children(elem)
		}
	}

	localName, targetNSURI, namespaceSpecified, ok := resolveXPathName(elementName, nsContext)
	if !ok {
		return nil
	}

	var results []xml.NodeID
	switch axis {
	case "child":
		results = e.collectMatchingChildrenNS(elem, localName, targetNSURI, namespaceSpecified, results)
	case "descendant":
		results = e.collectMatchingDescendantsNS(elem, localName, targetNSURI, namespaceSpecified, results)
	case "descendant-or-self":
		if e.matchesElementNS(elem, localName, targetNSURI, namespaceSpecified) {
			results = append(results, elem)
		}
		results = e.collectMatchingDescendantsNS(elem, localName, targetNSURI, namespaceSpecified, results)
	default:
		// default to child axis
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
func (e *Evaluator) collectMatchingChildrenNS(elem xml.NodeID, localName, targetNSURI string, namespaceSpecified bool, results []xml.NodeID) []xml.NodeID {
	for _, child := range e.doc.Children(elem) {
		if localName == "*" || e.matchesElementNS(child, localName, targetNSURI, namespaceSpecified) {
			results = append(results, child)
		}
	}
	return results
}

// collectMatchingDescendantsNS collects all descendants matching the element name and namespace.
func (e *Evaluator) collectMatchingDescendantsNS(elem xml.NodeID, localName, targetNSURI string, namespaceSpecified bool, results []xml.NodeID) []xml.NodeID {
	for _, child := range e.doc.Children(elem) {
		if localName == "*" || e.matchesElementNS(child, localName, targetNSURI, namespaceSpecified) {
			results = append(results, child)
		}
		results = e.collectMatchingDescendantsNS(child, localName, targetNSURI, namespaceSpecified, results)
	}
	return results
}

// collectAllDescendants collects all descendant elements.
func (e *Evaluator) collectAllDescendants(elem xml.NodeID, results []xml.NodeID) []xml.NodeID {
	for _, child := range e.doc.Children(elem) {
		results = append(results, child)
		results = e.collectAllDescendants(child, results)
	}
	return results
}

// matchesElementNS checks if an element matches the given local name and namespace URI.
func (e *Evaluator) matchesElementNS(elem xml.NodeID, localName, targetNSURI string, namespaceSpecified bool) bool {
	if localName != "*" && e.doc.LocalName(elem) != localName {
		return false
	}
	if namespaceSpecified && e.doc.NamespaceURI(elem) != targetNSURI {
		return false
	}
	return true
}
