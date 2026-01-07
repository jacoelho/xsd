package types

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

	return &AnyAttribute{
		Namespace:       intersectedNS.Constraint,
		NamespaceList:   intersectedNS.NamespaceList,
		ProcessContents: processContents,
		TargetNamespace: w1.TargetNamespace,
	}
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

	// MinOccurs: use maximum (more restrictive)
	minOccurs := max(w2.MinOccurs, w1.MinOccurs)

	// MaxOccurs: use minimum (more restrictive), but handle unbounded (-1)
	maxOccurs := w1.MaxOccurs
	if w2.MaxOccurs == UnboundedOccurs {
		// w2 is unbounded, use w1's limit
		// maxOccurs stays as w1.MaxOccurs
	} else if w1.MaxOccurs == UnboundedOccurs {
		// w1 is unbounded, use w2's limit
		maxOccurs = w2.MaxOccurs
	} else {
		// both bounded, use minimum
		if w2.MaxOccurs < maxOccurs {
			maxOccurs = w2.MaxOccurs
		}
	}

	return &AnyElement{
		Namespace:       intersectedNS.Constraint,
		NamespaceList:   intersectedNS.NamespaceList,
		ProcessContents: processContents,
		MinOccurs:       minOccurs,
		MaxOccurs:       maxOccurs,
		TargetNamespace: w1.TargetNamespace,
	}
}