package runtime

// SessionPlan carries sizing hints for reusable validation-session buffers.
type SessionPlan struct {
	MaxAttrs           int
	MaxAttrUses        int
	MaxModelWords      int
	MaxIdentityFields  int
	NameHint           int
	NameBytesHint      int
	NamespaceHint      int
	NamespaceBytesHint int
}

// NewSessionPlan derives validation-session buffer hints from immutable runtime tables.
func NewSessionPlan(schema *Schema) SessionPlan {
	if schema == nil {
		return SessionPlan{}
	}
	symbols := schema.SymbolsTable()
	namespaces := schema.NamespaceTable()
	plan := SessionPlan{
		NameHint:           symbols.Count(),
		NameBytesHint:      len(symbols.LocalBlob),
		NamespaceHint:      namespaces.Count(),
		NamespaceBytesHint: len(namespaces.Blob),
	}
	for _, ct := range schema.ComplexTypeTable() {
		plan.MaxAttrUses = max(plan.MaxAttrUses, int(ct.Attrs.Len))
	}
	plan.MaxAttrs = max(plan.MaxAttrUses+4, 8)
	plan.MaxModelWords = maxSessionModelWords(schema)
	for _, ic := range schema.IdentityConstraints() {
		plan.MaxIdentityFields = max(plan.MaxIdentityFields, int(ic.FieldLen))
	}
	return plan
}

func maxSessionModelWords(schema *Schema) int {
	maxWords := 0
	models := schema.ModelBundle()
	for _, nfa := range models.NFA {
		maxWords = max(maxWords, 2*int(nfa.Start.Len))
	}
	for _, all := range models.All {
		maxWords = max(maxWords, allModelWords(all))
	}

	types := schema.TypeTable()
	memo := make(map[TypeID]int, len(types))
	visiting := make(map[TypeID]bool, len(types))
	for _, elemID := range schema.GlobalElementIDs() {
		elem, ok := sessionPlanElement(schema, elemID)
		if !ok {
			continue
		}
		maxWords = max(maxWords, sessionPlanTypeWords(schema, elem.Type, memo, visiting))
	}
	return maxWords
}

func sessionPlanTypeWords(schema *Schema, id TypeID, memo map[TypeID]int, visiting map[TypeID]bool) int {
	typ, ok := schema.Type(id)
	if !ok {
		return 0
	}
	if words, ok := memo[id]; ok {
		return words
	}
	if visiting[id] {
		return 0
	}

	ct, ok := schema.ComplexType(typ.Complex.ID)
	if typ.Kind != TypeComplex || !ok {
		return 0
	}

	visiting[id] = true
	words := sessionPlanModelWords(schema, ct.Model) + maxSessionChildModelWords(schema, ct.Model, memo, visiting)
	visiting[id] = false
	memo[id] = words
	return words
}

func maxSessionChildModelWords(schema *Schema, ref ModelRef, memo map[TypeID]int, visiting map[TypeID]bool) int {
	maxWords := 0
	forEachModelElement(schema, ref, func(elemID ElemID) {
		elem, ok := sessionPlanElement(schema, elemID)
		if !ok {
			return
		}
		maxWords = max(maxWords, sessionPlanTypeWords(schema, elem.Type, memo, visiting))
	})
	return maxWords
}

func forEachModelElement(schema *Schema, ref ModelRef, yield func(ElemID)) {
	if schema == nil || yield == nil {
		return
	}
	switch ref.Kind {
	case ModelDFA:
		model, ok := schema.DFAModelByRef(ref)
		if !ok {
			return
		}
		for _, tr := range model.Transitions {
			yield(tr.Elem)
		}
	case ModelNFA:
		model, ok := schema.NFAModelByRef(ref)
		if !ok {
			return
		}
		for _, matcher := range model.Matchers {
			if matcher.Kind == PosExact {
				yield(matcher.Elem)
			}
		}
	case ModelAll:
		model, ok := schema.AllModelByRef(ref)
		if !ok {
			return
		}
		for _, member := range model.Members {
			yield(member.Elem)
		}
	}
}

func sessionPlanModelWords(schema *Schema, ref ModelRef) int {
	if schema == nil {
		return 0
	}
	switch ref.Kind {
	case ModelNFA:
		model, ok := schema.NFAModelByRef(ref)
		if !ok {
			return 0
		}
		return 2 * int(model.Start.Len)
	case ModelAll:
		model, ok := schema.AllModelByRef(ref)
		if !ok {
			return 0
		}
		return allModelWords(*model)
	default:
		return 0
	}
}

func allModelWords(model AllModel) int {
	return (len(model.Members) + 63) / 64
}

func sessionPlanElement(schema *Schema, id ElemID) (Element, bool) {
	return schema.Element(id)
}
