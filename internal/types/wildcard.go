package types

import "slices"

// NamespaceConstraint represents a namespace constraint
type NamespaceConstraint int

const (
	// NSCAny allows any namespace.
	NSCAny NamespaceConstraint = iota
	// NSCOther allows any namespace except the target namespace.
	NSCOther
	// NSCTargetNamespace allows only the target namespace.
	NSCTargetNamespace
	// NSCLocal allows only no-namespace.
	NSCLocal
	// NSCList allows an explicit namespace list.
	NSCList
)

// ProcessContents defines how to process wildcard elements
type ProcessContents int

const (
	// Strict requires validation against a declaration.
	Strict ProcessContents = iota
	// Lax validates only if a declaration is found.
	Lax
	Skip
)

// AnyElement represents an <any> wildcard
type AnyElement struct {
	Namespace       NamespaceConstraint
	NamespaceList   []NamespaceURI
	ProcessContents ProcessContents
	MinOccurs       int
	MaxOccurs       int
	TargetNamespace NamespaceURI
}

// MinOcc implements Particle interface
func (a *AnyElement) MinOcc() int {
	return a.MinOccurs
}

// MaxOcc implements Particle interface
func (a *AnyElement) MaxOcc() int {
	return a.MaxOccurs
}

// AnyAttribute represents an <anyAttribute> wildcard
type AnyAttribute struct {
	Namespace       NamespaceConstraint
	NamespaceList   []NamespaceURI
	ProcessContents ProcessContents
	TargetNamespace NamespaceURI
}

// AllowsQName reports whether the anyAttribute wildcard allows the given QName.
func (a *AnyAttribute) AllowsQName(qname QName) bool {
	if a == nil {
		return false
	}
	return AllowsNamespace(a.Namespace, a.NamespaceList, a.TargetNamespace, qname.Namespace)
}

// NSCInvalid represents an invalid namespace constraint.
const NSCInvalid NamespaceConstraint = -1

type intersectedNamespace struct {
	Constraint    NamespaceConstraint
	NamespaceList []NamespaceURI
}

type wildcardConstraint struct {
	constraint NamespaceConstraint
	list       []NamespaceURI
	target     NamespaceURI
}

// AllowsNamespace reports whether a namespace is permitted by a wildcard constraint.
func AllowsNamespace(constraint NamespaceConstraint, list []NamespaceURI, targetNS NamespaceURI, ns NamespaceURI) bool {
	switch constraint {
	case NSCAny:
		return true
	case NSCLocal:
		return ns.IsEmpty()
	case NSCTargetNamespace:
		return ns == targetNS
	case NSCOther:
		return !ns.IsEmpty() && ns != targetNS
	case NSCList:
		return namespaceListContains(list, ns)
	default:
		return false
	}
}

func isWildcardSubset(a, b wildcardConstraint) bool {
	switch a.constraint {
	case NSCAny:
		return b.constraint == NSCAny
	case NSCOther:
		return b.constraint == NSCAny || (b.constraint == NSCOther && a.target == b.target)
	case NSCTargetNamespace:
		return AllowsNamespace(b.constraint, b.list, b.target, a.target)
	case NSCLocal:
		return AllowsNamespace(b.constraint, b.list, b.target, NamespaceEmpty)
	case NSCList:
		for _, ns := range a.list {
			if !AllowsNamespace(b.constraint, b.list, b.target, ns) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func intersectWildcards(a, b wildcardConstraint) (wildcardConstraint, bool) {
	if a.constraint == NSCAny {
		return b, true
	}
	if b.constraint == NSCAny {
		return a, true
	}
	if isWildcardSubset(a, b) {
		return a, true
	}
	if isWildcardSubset(b, a) {
		return b, true
	}

	switch {
	case a.constraint == NSCList && b.constraint == NSCList:
		result := intersectNamespaceLists(a.list, b.list)
		if len(result) == 0 {
			return wildcardConstraint{}, false
		}
		return wildcardConstraint{constraint: NSCList, list: result, target: a.target}, true
	case a.constraint == NSCList && b.constraint == NSCOther:
		result := filterNamespaceList(a.list, b)
		if len(result) == 0 {
			return wildcardConstraint{}, false
		}
		return wildcardConstraint{constraint: NSCList, list: result, target: a.target}, true
	case b.constraint == NSCList && a.constraint == NSCOther:
		result := filterNamespaceList(b.list, a)
		if len(result) == 0 {
			return wildcardConstraint{}, false
		}
		return wildcardConstraint{constraint: NSCList, list: result, target: b.target}, true
	default:
		return wildcardConstraint{}, false
	}
}

func intersectNamespaceConstraints(ns1 NamespaceConstraint, list1 []NamespaceURI, targetNS1 NamespaceURI, ns2 NamespaceConstraint, list2 []NamespaceURI, targetNS2 NamespaceURI) intersectedNamespace {
	intersection, ok := intersectWildcards(
		wildcardConstraint{constraint: ns1, list: list1, target: targetNS1},
		wildcardConstraint{constraint: ns2, list: list2, target: targetNS2},
	)
	if !ok {
		return intersectedNamespace{Constraint: NSCInvalid, NamespaceList: nil}
	}

	return intersectedNamespace{Constraint: intersection.constraint, NamespaceList: intersection.list}
}

func namespaceListContains(list []NamespaceURI, target NamespaceURI) bool {
	return slices.Contains(list, target)
}

func filterNamespaceList(list []NamespaceURI, constraint wildcardConstraint) []NamespaceURI {
	filtered := make([]NamespaceURI, 0, len(list))
	for _, ns := range list {
		if AllowsNamespace(constraint.constraint, constraint.list, constraint.target, ns) {
			filtered = append(filtered, ns)
		}
	}
	return filtered
}

func intersectNamespaceLists(list1, list2 []NamespaceURI) []NamespaceURI {
	result := make([]NamespaceURI, 0)
	for _, ns1 := range list1 {
		if namespaceListContains(list2, ns1) {
			result = append(result, ns1)
		}
	}
	return result
}

// unionNamespaceConstraints unions two namespace constraints according to cos-aw-union
func unionNamespaceConstraints(ns1 NamespaceConstraint, list1 []NamespaceURI, targetNS1 NamespaceURI, ns2 NamespaceConstraint, list2 []NamespaceURI, targetNS2 NamespaceURI, resultTargetNS NamespaceURI) intersectedNamespace {
	if ns1 == NSCTargetNamespace {
		ns1 = NSCList
		list1 = []NamespaceURI{targetNS1}
	}
	if ns2 == NSCTargetNamespace {
		ns2 = NSCList
		list2 = []NamespaceURI{targetNS2}
	}
	if ns1 == NSCLocal {
		ns1 = NSCList
		list1 = []NamespaceURI{NamespaceEmpty}
	}
	if ns2 == NSCLocal {
		ns2 = NSCList
		list2 = []NamespaceURI{NamespaceEmpty}
	}

	// If either O1 or O2 is ##any, ##any must be the value
	if ns1 == NSCAny || ns2 == NSCAny {
		return intersectedNamespace{Constraint: NSCAny, NamespaceList: nil}
	}

	// If both are sets (NSCList), the union of those sets must be the value
	if ns1 == NSCList && ns2 == NSCList {
		result := unionLists(list1, list2)
		return intersectedNamespace{Constraint: NSCList, NamespaceList: result}
	}

	// Handle ##other (negation of targetNamespace) with list
	if ns1 == NSCOther && ns2 == NSCList {
		return unionOtherWithList(list2, targetNS1, resultTargetNS)
	}
	if ns2 == NSCOther && ns1 == NSCList {
		return unionOtherWithList(list1, targetNS2, resultTargetNS)
	}

	if ns1 == NSCOther && ns2 == NSCOther {
		if targetNS1 == targetNS2 && targetNS1 == resultTargetNS {
			return intersectedNamespace{Constraint: NSCOther, NamespaceList: nil}
		}
		return intersectedNamespace{Constraint: NSCInvalid, NamespaceList: nil}
	}

	// Default: not expressible
	return intersectedNamespace{Constraint: NSCInvalid, NamespaceList: nil}
}

// unionOtherWithList handles union of ##other with a list according to spec rules
func unionOtherWithList(list []NamespaceURI, otherTargetNS NamespaceURI, resultTargetNS NamespaceURI) intersectedNamespace {
	// According to spec: if set includes both the negated namespace name and absent, then ##any
	hasTargetNS := false
	hasEmpty := false
	for _, ns := range list {
		if ns == otherTargetNS {
			hasTargetNS = true
		}
		if ns.IsEmpty() {
			hasEmpty = true
		}
	}

	if hasTargetNS && hasEmpty {
		return intersectedNamespace{Constraint: NSCAny, NamespaceList: nil}
	}

	// If set includes negated namespace but not absent, return not and absent
	if hasTargetNS && !hasEmpty {
		// Treat as ##any (##other plus target namespace covers all namespaces)
		return intersectedNamespace{Constraint: NSCAny, NamespaceList: nil}
	}

	// If set includes absent but not negated namespace, union is not expressible
	if hasEmpty && !hasTargetNS {
		return intersectedNamespace{Constraint: NSCInvalid, NamespaceList: nil}
	}

	// If set doesn't include either, return ##other
	if !hasTargetNS && !hasEmpty {
		if otherTargetNS == resultTargetNS {
			return intersectedNamespace{Constraint: NSCOther, NamespaceList: nil}
		}
		return intersectedNamespace{Constraint: NSCInvalid, NamespaceList: nil}
	}

	// Should not reach here
	return intersectedNamespace{Constraint: NSCInvalid, NamespaceList: nil}
}

// unionLists creates the union of two namespace lists (removes duplicates)
func unionLists(list1, list2 []NamespaceURI) []NamespaceURI {
	seen := make(map[NamespaceURI]bool)
	result := make([]NamespaceURI, 0)

	for _, ns := range list1 {
		if !seen[ns] {
			seen[ns] = true
			result = append(result, ns)
		}
	}

	for _, ns := range list2 {
		if !seen[ns] {
			seen[ns] = true
			result = append(result, ns)
		}
	}

	return result
}
