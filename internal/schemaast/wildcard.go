package schemaast

import (
	"slices"
)

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
	// NSCNotAbsent allows any namespace-qualified name (excludes no-namespace).
	NSCNotAbsent
)

// NamespaceTargetPlaceholder marks a namespace list entry that represents ##targetNamespace.
// It is resolved against the wildcard's TargetNamespace at validation time.
const NamespaceTargetPlaceholder NamespaceURI = "##targetNamespace"

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
	TargetNamespace NamespaceURI
	NamespaceList   []NamespaceURI
	MinOccurs       Occurs
	MaxOccurs       Occurs
	Namespace       NamespaceConstraint
	ProcessContents ProcessContents
}

// MinOcc implements Particle interface
func (a *AnyElement) MinOcc() Occurs {
	return a.MinOccurs
}

// MaxOcc implements Particle interface
func (a *AnyElement) MaxOcc() Occurs {
	return a.MaxOccurs
}

// AnyAttribute represents an <anyAttribute> wildcard
type AnyAttribute struct {
	TargetNamespace NamespaceURI
	NamespaceList   []NamespaceURI
	Namespace       NamespaceConstraint
	ProcessContents ProcessContents
}

// NSCInvalid represents an invalid namespace constraint.
const NSCInvalid NamespaceConstraint = -1

type intersectedNamespace struct {
	NamespaceList []NamespaceURI
	Constraint    NamespaceConstraint
}

type wildcardConstraint struct {
	target     NamespaceURI
	list       []NamespaceURI
	constraint NamespaceConstraint
}

// AllowsNamespace reports whether a namespace is permitted by a wildcard constraint.
func AllowsNamespace(constraint NamespaceConstraint, list []NamespaceURI, targetNS, ns NamespaceURI) bool {
	switch constraint {
	case NSCAny:
		return true
	case NSCLocal:
		return ns == NamespaceEmpty
	case NSCTargetNamespace:
		return ns == targetNS
	case NSCOther:
		return ns != targetNS && ns != NamespaceEmpty
	case NSCNotAbsent:
		return ns != NamespaceEmpty
	case NSCList:
		for _, allowed := range list {
			if resolveNamespaceToken(allowed, targetNS) == ns {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// ProcessContentsStrongerOrEqual reports whether derived is as strict as base.
func ProcessContentsStrongerOrEqual(derived, base ProcessContents) bool {
	switch base {
	case Strict:
		return derived == Strict
	case Lax:
		return derived == Lax || derived == Strict
	case Skip:
		return true
	default:
		return false
	}
}

// NamespaceConstraintSubset reports whether the first wildcard namespace constraint
// is a subset of the second constraint.
func NamespaceConstraintSubset(
	derivedConstraint NamespaceConstraint,
	derivedList []NamespaceURI,
	derivedTargetNS NamespaceURI,
	baseConstraint NamespaceConstraint,
	baseList []NamespaceURI,
	baseTargetNS NamespaceURI,
) bool {
	return namespaceConstraintSubset(
		wildcardConstraint{
			constraint: derivedConstraint,
			list:       derivedList,
			target:     derivedTargetNS,
		},
		wildcardConstraint{
			constraint: baseConstraint,
			list:       baseList,
			target:     baseTargetNS,
		},
	)
}

func namespaceConstraintSubset(derived, base wildcardConstraint) bool {
	switch derived.constraint {
	case NSCAny:
		return base.constraint == NSCAny
	case NSCOther:
		if base.constraint == NSCAny {
			return true
		}
		if base.constraint == NSCOther && derived.target == base.target {
			return true
		}
		return base.constraint == NSCNotAbsent
	case NSCNotAbsent:
		switch base.constraint {
		case NSCAny, NSCNotAbsent:
			return true
		case NSCOther:
			return base.target == NamespaceEmpty
		default:
			return false
		}
	case NSCTargetNamespace:
		return AllowsNamespace(base.constraint, base.list, base.target, derived.target)
	case NSCLocal:
		return AllowsNamespace(base.constraint, base.list, base.target, NamespaceEmpty)
	case NSCList:
		for _, ns := range derived.list {
			if !AllowsNamespace(base.constraint, base.list, base.target, resolveNamespaceToken(ns, derived.target)) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func resolveNamespaceToken(ns, targetNS NamespaceURI) NamespaceURI {
	if ns == NamespaceTargetPlaceholder {
		return targetNS
	}
	return ns
}

func resolvedNamespaceList(list []NamespaceURI, targetNS NamespaceURI) []NamespaceURI {
	if len(list) == 0 {
		return nil
	}
	seen := make(map[NamespaceURI]bool, len(list))
	out := make([]NamespaceURI, 0, len(list))
	for _, ns := range list {
		resolved := resolveNamespaceToken(ns, targetNS)
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		out = append(out, resolved)
	}
	return out
}

func isWildcardSubset(a, b wildcardConstraint) bool {
	return namespaceConstraintSubset(a, b)
}

func intersectWildcards(a, b wildcardConstraint) (wildcardConstraint, bool) {
	intersection, ok, empty := intersectWildcardsDetailed(a, b)
	if !ok || empty {
		return wildcardConstraint{}, false
	}
	return intersection, true
}

func intersectWildcardsDetailed(a, b wildcardConstraint) (wildcardConstraint, bool, bool) {
	if result, handled := intersectWildcardTrivialCases(a, b); handled {
		return result.constraint, result.expressible, result.empty
	}
	if result, handled := intersectTargetOrLocalWithList(a, b); handled {
		return result.constraint, result.expressible, result.empty
	}
	if result, handled := intersectListPairs(a, b); handled {
		return result.constraint, result.expressible, result.empty
	}
	if result, handled := intersectOtherPairs(a, b); handled {
		return result.constraint, result.expressible, result.empty
	}
	if result, handled := intersectLocalAndNotAbsent(a, b); handled {
		return result.constraint, result.expressible, result.empty
	}
	if result, handled := intersectTargetPairs(a, b); handled {
		return result.constraint, result.expressible, result.empty
	}
	return wildcardConstraint{}, false, false
}

type wildcardIntersectionResult struct {
	constraint  wildcardConstraint
	expressible bool
	empty       bool
}

func intersectWildcardTrivialCases(a, b wildcardConstraint) (wildcardIntersectionResult, bool) {
	switch {
	case a.constraint == NSCAny:
		return wildcardIntersectionResult{constraint: b, expressible: true}, true
	case b.constraint == NSCAny:
		return wildcardIntersectionResult{constraint: a, expressible: true}, true
	case isWildcardSubset(a, b):
		return wildcardIntersectionResult{constraint: a, expressible: true}, true
	case isWildcardSubset(b, a):
		return wildcardIntersectionResult{constraint: b, expressible: true}, true
	default:
		return wildcardIntersectionResult{}, false
	}
}

func intersectTargetOrLocalWithList(a, b wildcardConstraint) (wildcardIntersectionResult, bool) {
	switch {
	case a.constraint == NSCTargetNamespace && b.constraint == NSCList:
		return intersectListMembership(b, a, a.target), true
	case b.constraint == NSCTargetNamespace && a.constraint == NSCList:
		return intersectListMembership(a, b, b.target), true
	case a.constraint == NSCLocal && b.constraint == NSCList:
		return intersectListMembership(b, a, NamespaceEmpty), true
	case b.constraint == NSCLocal && a.constraint == NSCList:
		return intersectListMembership(a, b, NamespaceEmpty), true
	default:
		return wildcardIntersectionResult{}, false
	}
}

func intersectListPairs(a, b wildcardConstraint) (wildcardIntersectionResult, bool) {
	switch {
	case a.constraint == NSCList && b.constraint == NSCList:
		result := intersectNamespaceLists(a.list, b.list, a.target, b.target)
		return listIntersectionResult(result, a.target), true
	case a.constraint == NSCList && (b.constraint == NSCOther || b.constraint == NSCNotAbsent):
		return filteredListIntersection(a, b), true
	case b.constraint == NSCList && (a.constraint == NSCOther || a.constraint == NSCNotAbsent):
		return filteredListIntersection(b, a), true
	default:
		return wildcardIntersectionResult{}, false
	}
}

func intersectOtherPairs(a, b wildcardConstraint) (wildcardIntersectionResult, bool) {
	switch {
	case a.constraint == NSCOther && b.constraint == NSCOther:
		return intersectOtherWithOther(a, b), true
	case a.constraint == NSCOther && b.constraint == NSCTargetNamespace:
		return intersectOtherWithTarget(a, b), true
	case b.constraint == NSCOther && a.constraint == NSCTargetNamespace:
		return intersectOtherWithTarget(b, a), true
	case a.constraint == NSCOther && b.constraint == NSCLocal:
		return emptyWildcardIntersection(), true
	case b.constraint == NSCOther && a.constraint == NSCLocal:
		return emptyWildcardIntersection(), true
	default:
		return wildcardIntersectionResult{}, false
	}
}

func intersectLocalAndNotAbsent(a, b wildcardConstraint) (wildcardIntersectionResult, bool) {
	if (a.constraint == NSCNotAbsent && b.constraint == NSCLocal) || (b.constraint == NSCNotAbsent && a.constraint == NSCLocal) {
		return emptyWildcardIntersection(), true
	}
	return wildcardIntersectionResult{}, false
}

func intersectTargetPairs(a, b wildcardConstraint) (wildcardIntersectionResult, bool) {
	switch {
	case a.constraint == NSCTargetNamespace && b.constraint == NSCTargetNamespace:
		if a.target == b.target {
			return wildcardIntersectionResult{constraint: a, expressible: true}, true
		}
		return emptyWildcardIntersection(), true
	case a.constraint == NSCTargetNamespace && b.constraint == NSCNotAbsent:
		return intersectTargetWithNotAbsent(a), true
	case b.constraint == NSCTargetNamespace && a.constraint == NSCNotAbsent:
		return intersectTargetWithNotAbsent(b), true
	default:
		return wildcardIntersectionResult{}, false
	}
}

func intersectListMembership(listConstraint, matched wildcardConstraint, namespace NamespaceURI) wildcardIntersectionResult {
	if AllowsNamespace(listConstraint.constraint, listConstraint.list, listConstraint.target, namespace) {
		return wildcardIntersectionResult{constraint: matched, expressible: true}
	}
	return emptyWildcardIntersection()
}

func filteredListIntersection(listConstraint, filter wildcardConstraint) wildcardIntersectionResult {
	result := filterNamespaceList(listConstraint.list, listConstraint.target, filter)
	return listIntersectionResult(result, listConstraint.target)
}

func listIntersectionResult(list []NamespaceURI, target NamespaceURI) wildcardIntersectionResult {
	if len(list) == 0 {
		return emptyWildcardIntersection()
	}
	return wildcardIntersectionResult{
		constraint:  wildcardConstraint{constraint: NSCList, list: list, target: target},
		expressible: true,
	}
}

func intersectOtherWithOther(a, b wildcardConstraint) wildcardIntersectionResult {
	switch {
	case a.target == b.target:
		return wildcardIntersectionResult{constraint: a, expressible: true}
	case a.target == NamespaceEmpty:
		return wildcardIntersectionResult{constraint: b, expressible: true}
	case b.target == NamespaceEmpty:
		return wildcardIntersectionResult{constraint: a, expressible: true}
	default:
		return wildcardIntersectionResult{}
	}
}

func intersectOtherWithTarget(other, target wildcardConstraint) wildcardIntersectionResult {
	if target.target == NamespaceEmpty || target.target == other.target {
		return emptyWildcardIntersection()
	}
	return wildcardIntersectionResult{constraint: target, expressible: true}
}

func intersectTargetWithNotAbsent(target wildcardConstraint) wildcardIntersectionResult {
	if target.target == NamespaceEmpty {
		return emptyWildcardIntersection()
	}
	return wildcardIntersectionResult{constraint: target, expressible: true}
}

func emptyWildcardIntersection() wildcardIntersectionResult {
	return wildcardIntersectionResult{expressible: true, empty: true}
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

func namespaceListContains(list []NamespaceURI, target, listTargetNS NamespaceURI) bool {
	for _, ns := range list {
		if resolveNamespaceToken(ns, listTargetNS) == target {
			return true
		}
	}
	return false
}

func filterNamespaceList(list []NamespaceURI, listTargetNS NamespaceURI, constraint wildcardConstraint) []NamespaceURI {
	filtered := make([]NamespaceURI, 0, len(list))
	for _, ns := range list {
		resolved := resolveNamespaceToken(ns, listTargetNS)
		if AllowsNamespace(constraint.constraint, constraint.list, constraint.target, resolved) {
			filtered = append(filtered, resolved)
		}
	}
	return filtered
}

func intersectNamespaceLists(list1, list2 []NamespaceURI, targetNS1, targetNS2 NamespaceURI) []NamespaceURI {
	result := make([]NamespaceURI, 0)
	for _, ns1 := range list1 {
		resolved1 := resolveNamespaceToken(ns1, targetNS1)
		if namespaceListContains(list2, resolved1, targetNS2) {
			result = append(result, resolved1)
		}
	}
	return result
}

// unionNamespaceConstraints unions two namespace constraints according to cos-aw-union
func unionNamespaceConstraints(ns1 NamespaceConstraint, list1 []NamespaceURI, targetNS1 NamespaceURI, ns2 NamespaceConstraint, list2 []NamespaceURI, targetNS2, resultTargetNS NamespaceURI) intersectedNamespace {
	if ns1 == NSCList {
		list1 = resolvedNamespaceList(list1, targetNS1)
	}
	if ns2 == NSCList {
		list2 = resolvedNamespaceList(list2, targetNS2)
	}
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

	// if either O1 or O2 is ##any, ##any must be the value
	if ns1 == NSCAny || ns2 == NSCAny {
		return intersectedNamespace{Constraint: NSCAny, NamespaceList: nil}
	}

	// if both are sets (NSCList), the union of those sets must be the value
	if ns1 == NSCList && ns2 == NSCList {
		result := unionLists(list1, list2)
		return intersectedNamespace{Constraint: NSCList, NamespaceList: result}
	}

	// handle ##other (negation of targetNamespace) with list
	if ns1 == NSCOther && ns2 == NSCList {
		return unionOtherWithList(list2, targetNS1, resultTargetNS)
	}
	if ns2 == NSCOther && ns1 == NSCList {
		return unionOtherWithList(list1, targetNS2, resultTargetNS)
	}

	// handle notAbsent with list (notAbsent + local => any, otherwise notAbsent)
	if ns1 == NSCNotAbsent && ns2 == NSCList {
		return unionNotAbsentWithList(list2)
	}
	if ns2 == NSCNotAbsent && ns1 == NSCList {
		return unionNotAbsentWithList(list1)
	}

	// handle notAbsent with other/other-like
	if ns1 == NSCNotAbsent && ns2 == NSCOther {
		return intersectedNamespace{Constraint: NSCNotAbsent, NamespaceList: nil}
	}
	if ns2 == NSCNotAbsent && ns1 == NSCOther {
		return intersectedNamespace{Constraint: NSCNotAbsent, NamespaceList: nil}
	}
	if ns1 == NSCNotAbsent && ns2 == NSCNotAbsent {
		return intersectedNamespace{Constraint: NSCNotAbsent, NamespaceList: nil}
	}

	if ns1 == NSCOther && ns2 == NSCOther {
		if targetNS1 == resultTargetNS && targetNS2 == resultTargetNS {
			return intersectedNamespace{Constraint: NSCOther, NamespaceList: nil}
		}
		return intersectedNamespace{Constraint: NSCNotAbsent, NamespaceList: nil}
	}

	// default: not expressible
	return intersectedNamespace{Constraint: NSCInvalid, NamespaceList: nil}
}

// unionOtherWithList handles union of ##other with a list according to spec rules
func unionOtherWithList(list []NamespaceURI, otherTargetNS, resultTargetNS NamespaceURI) intersectedNamespace {
	if otherTargetNS != resultTargetNS {
		return intersectedNamespace{Constraint: NSCInvalid, NamespaceList: nil}
	}
	hasTarget := false
	hasLocal := false
	for _, ns := range list {
		resolved := resolveNamespaceToken(ns, resultTargetNS)
		if resolved == otherTargetNS {
			hasTarget = true
		}
		if resolved == "" {
			hasLocal = true
		}
		if hasTarget && hasLocal {
			break
		}
	}
	switch {
	case hasTarget && hasLocal:
		return intersectedNamespace{Constraint: NSCAny, NamespaceList: nil}
	case hasTarget && !hasLocal:
		return intersectedNamespace{Constraint: NSCNotAbsent, NamespaceList: nil}
	case hasLocal && !hasTarget:
		return intersectedNamespace{Constraint: NSCInvalid, NamespaceList: nil}
	default:
		return intersectedNamespace{Constraint: NSCOther, NamespaceList: nil}
	}
}

func unionNotAbsentWithList(list []NamespaceURI) intersectedNamespace {
	if slices.Contains(list, "") {
		return intersectedNamespace{Constraint: NSCAny, NamespaceList: nil}
	}
	return intersectedNamespace{Constraint: NSCNotAbsent, NamespaceList: nil}
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

// UnionAnyAttribute unions two AnyAttribute wildcards according to XSD 1.0 spec (cos-aw-union)
// The result represents the union of namespaces that match either wildcard
// Returns nil if union is not expressible
func UnionAnyAttribute(w1, w2 *AnyAttribute) *AnyAttribute {
	if w1 == nil {
		return w2
	}
	if w2 == nil {
		return w1
	}

	// union namespace constraints
	unionedNS := unionNamespaceConstraints(
		w1.Namespace, w1.NamespaceList, w1.TargetNamespace,
		w2.Namespace, w2.NamespaceList, w2.TargetNamespace,
		w1.TargetNamespace,
	)
	if unionedNS.Constraint == NSCInvalid {
		// union is not expressible
		return nil
	}

	// ProcessContents: according to spec (line 3332-3333), use the complete wildcard's processContents
	// for union (extension case), w1 is the complete wildcard, w2 is the base wildcard
	processContents := w1.ProcessContents
	return &AnyAttribute{
		Namespace:       unionedNS.Constraint,
		NamespaceList:   unionedNS.NamespaceList,
		ProcessContents: processContents,
		TargetNamespace: w1.TargetNamespace,
	}
}

// IntersectAnyAttribute intersects two AnyAttribute wildcards according to XSD 1.0 spec
// The result represents the set of namespaces that match both wildcards
// Returns nil if intersection is empty (no namespaces match both)
func IntersectAnyAttribute(w1, w2 *AnyAttribute) *AnyAttribute {
	result, expressible, empty := IntersectAnyAttributeDetailed(w1, w2)
	if !expressible || empty {
		return nil
	}
	return result
}

// IntersectAnyAttributeDetailed intersects two AnyAttribute wildcards and reports expressibility.
// If expressible is true and empty is true, the intersection is empty.
func IntersectAnyAttributeDetailed(w1, w2 *AnyAttribute) (*AnyAttribute, bool, bool) {
	if w1 == nil {
		return w2, true, false
	}
	if w2 == nil {
		return w1, true, false
	}

	// intersect namespace constraints
	constraint, expressible, empty := intersectWildcardsDetailed(
		wildcardConstraint{constraint: w1.Namespace, list: w1.NamespaceList, target: w1.TargetNamespace},
		wildcardConstraint{constraint: w2.Namespace, list: w2.NamespaceList, target: w2.TargetNamespace},
	)
	if !expressible || empty {
		return nil, expressible, empty
	}

	// ProcessContents: use most restrictive (strict > lax > skip)
	processContents := w1.ProcessContents
	if w2.ProcessContents == Strict || (w2.ProcessContents == Lax && w1.ProcessContents == Skip) {
		processContents = w2.ProcessContents
	}

	return &AnyAttribute{
		Namespace:       constraint.constraint,
		NamespaceList:   constraint.list,
		ProcessContents: processContents,
		TargetNamespace: w1.TargetNamespace,
	}, true, false
}

// IntersectAnyElement intersects two AnyElement wildcards according to XSD 1.0 spec
func IntersectAnyElement(w1, w2 *AnyElement) *AnyElement {
	if w1 == nil {
		return w2
	}
	if w2 == nil {
		return w1
	}

	// intersect namespace constraints
	intersectedNS := intersectNamespaceConstraints(
		w1.Namespace, w1.NamespaceList, w1.TargetNamespace,
		w2.Namespace, w2.NamespaceList, w2.TargetNamespace,
	)
	if intersectedNS.Constraint == NSCInvalid {
		// intersection is empty
		return nil
	}

	// ProcessContents: use most restrictive (strict > lax > skip)
	processContents := w1.ProcessContents
	if w2.ProcessContents == Strict || (w2.ProcessContents == Lax && w1.ProcessContents == Skip) {
		processContents = w2.ProcessContents
	}

	// MinOccurs: use maximum (more restrictive).
	minOccurs := MaxOccurs(w2.MinOccurs, w1.MinOccurs)

	// MaxOccurs: use minimum (more restrictive), treating unbounded as infinity.
	maxOccurs := MinOccurs(w2.MaxOccurs, w1.MaxOccurs)

	return &AnyElement{
		Namespace:       intersectedNS.Constraint,
		NamespaceList:   intersectedNS.NamespaceList,
		ProcessContents: processContents,
		MinOccurs:       minOccurs,
		MaxOccurs:       maxOccurs,
		TargetNamespace: w1.TargetNamespace,
	}
}
