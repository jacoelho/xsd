package schema

// restrictionModelView borrows semantic input and owns append-only comparison
// nodes. Readers borrow its slices without mutation. Temporary IDs never enter
// the build or its analysis caches.
type restrictionModelView struct {
	original []ContentModel
	added    []restrictionModel
}

func (m *restrictionModelView) model(id ContentModelID) (ContentModel, bool) {
	if uint64(id) < uint64(len(m.original)) {
		return m.original[id], true
	}
	index := uint64(id) - uint64(len(m.original))
	if index >= uint64(len(m.added)) {
		return ContentModel{}, false
	}
	return m.added[index].model, true
}

// A comparison node owns its order provenance. Whole generated choices need
// only the marker; flattening into an explicit choice materializes local spans.
type restrictionModel struct {
	order restrictionChoiceOrder
	model ContentModel
}

type restrictionChoiceOrder struct {
	spans     []restrictionChoiceSpan
	unordered bool
}

func (o restrictionChoiceOrder) ordered() bool {
	return !o.unordered && len(o.spans) == 0
}

func (m *restrictionModelView) choiceOrder(id ContentModelID) restrictionChoiceOrder {
	if uint64(id) < uint64(len(m.original)) {
		return restrictionChoiceOrder{}
	}
	return m.added[uint64(id)-uint64(len(m.original))].order
}

func (v contentRestrictionValidator) choiceOrder(id ContentModelID) restrictionChoiceOrder {
	if v.analysis.models == nil {
		return restrictionChoiceOrder{}
	}
	return v.analysis.models.choiceOrder(id)
}

// restrictionChoiceSpan preserves an unordered substitution-generated choice
// after normalization flattens it into an explicitly ordered parent choice.
// Positions outside these disjoint half-open spans retain source order.
type restrictionChoiceSpan struct {
	start int
	end   int
}

// A generated derived span may reuse a target: removing the order condition
// leaves a functional mapping, not an injective one. Earliest matching targets
// minimize the next explicit boundary, so no permutation search is needed.
func (v contentRestrictionValidator) validateChoiceSpans(base, derived []Particle, baseOrder, derivedOrder restrictionChoiceOrder) error {
	after, err := v.choiceTargetBoundaries(len(base), baseOrder)
	if err != nil {
		return err
	}
	lower, spanIndex := 0, 0
	for i := 0; i < len(derived); {
		end := i + 1
		if derivedOrder.unordered {
			end = len(derived)
		} else if spanIndex < len(derivedOrder.spans) && derivedOrder.spans[spanIndex].start == i {
			end = derivedOrder.spans[spanIndex].end
			spanIndex++
		}
		lower, err = v.matchDerivedChoiceSpan(base, derived[i:end], after, lower)
		if err != nil {
			return err
		}
		i = end
	}
	return nil
}

func (v contentRestrictionValidator) choiceTargetBoundaries(size int, order restrictionChoiceOrder) ([]int, error) {
	if err := v.spend(size); err != nil {
		return nil, err
	}
	after := make([]int, size)
	if order.unordered {
		if err := v.spend(size); err != nil {
			return nil, err
		}
		return after, nil
	}
	for i := range after {
		after[i] = i + 1
	}
	for _, span := range order.spans {
		if err := v.spend(span.end - span.start); err != nil {
			return nil, err
		}
		for i := span.start; i < span.end; i++ {
			after[i] = span.start
		}
	}
	return after, nil
}

func (v contentRestrictionValidator) matchDerivedChoiceSpan(base, derived []Particle, after []int, lower int) (int, error) {
	next := lower
	for _, particle := range derived {
		boundary, err := v.matchChoiceSpanBranch(base, after, lower, particle)
		if err != nil {
			return 0, err
		}
		next = max(next, boundary)
	}
	return next, nil
}

func (v contentRestrictionValidator) matchChoiceSpanBranch(base []Particle, after []int, start int, derived Particle) (int, error) {
	for i := start; i < len(base); i++ {
		allowed, err := v.choiceBranchRestricts(base[i], derived)
		if err != nil {
			return 0, err
		}
		if allowed {
			return after[i], nil
		}
	}
	return 0, contentRestrictionMismatch("choice restriction branch is not subset of base")
}
