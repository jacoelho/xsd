package schema

// modelHasNoParticles reports whether modelID is structurally empty.
func modelHasNoParticles(rt *schemaBuild, modelID contentModelID) bool {
	if modelID == noContentModel {
		return true
	}
	model, ok := contentModelByID(rt.Models, modelID)
	if !ok {
		return false
	}
	switch model.Kind {
	case modelEmpty:
		return true
	case modelSequence, modelChoice, modelAll:
		return len(model.Particles) == 0
	case modelAny:
		return false
	default:
	}
	return false
}

// sequenceChoiceRange reports the range for a sequence that restricts a choice.
func sequenceChoiceRange(model contentModelNode) occurrence {
	particleCount := saturatingUint32(len(model.Particles))
	if model.Occurs.Unbounded {
		return occurrence{Min: saturatingMul(model.Occurs.Min, particleCount), Unbounded: true}
	}
	return occurrence{Min: saturatingMul(model.Occurs.Min, particleCount), Max: saturatingMul(model.Occurs.Max, particleCount)}
}

// addOccurrenceRanges saturates sequence ranges before applying occurrence limits.
func addOccurrenceRanges(a, b occurrence) occurrence {
	return occurrence{Min: saturatingAdd(a.Min, b.Min), Max: saturatingAdd(a.Max, b.Max), Unbounded: a.Unbounded || b.Unbounded}
}

// unionOccurrenceRanges returns the range accepted by either occurrence range.
func unionOccurrenceRanges(a, b occurrence) occurrence {
	minOccurs := min(b.Min, a.Min)
	if a.Unbounded || b.Unbounded {
		return occurrence{Min: minOccurs, Unbounded: true}
	}
	maxOccurs := max(b.Max, a.Max)
	return occurrence{Min: minOccurs, Max: maxOccurs}
}

// multiplyOccurrence applies occurrence constraints to an accepted range.
func multiplyOccurrence(a, b occurrence) occurrence {
	minOccurs := saturatingMul(a.Min, b.Min)
	if a.Unbounded || b.Unbounded {
		return occurrence{Min: minOccurs, Unbounded: true}
	}
	return occurrence{Min: minOccurs, Max: saturatingMul(a.Max, b.Max)}
}

// occurrenceRangeSubset reports whether derived is a subset of base.
func occurrenceRangeSubset(derived, base occurrence) bool {
	if derived.Min < base.Min {
		return false
	}
	if base.Unbounded {
		return true
	}
	if derived.Unbounded {
		return false
	}
	return derived.Max <= base.Max
}

func saturatingUint32(n int) uint32 {
	if n < 0 || uint64(n) > uint64(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(n)
}

func saturatingAdd(a, b uint32) uint32 {
	if ^uint32(0)-a < b {
		return ^uint32(0)
	}
	return a + b
}

func saturatingMul(a, b uint32) uint32 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > ^uint32(0)/b {
		return ^uint32(0)
	}
	return a * b
}
