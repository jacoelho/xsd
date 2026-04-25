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
	plan := SessionPlan{
		NameHint:           schema.Symbols.Count(),
		NameBytesHint:      len(schema.Symbols.LocalBlob),
		NamespaceHint:      schema.Namespaces.Count(),
		NamespaceBytesHint: len(schema.Namespaces.Blob),
	}
	for _, ct := range schema.ComplexTypes {
		plan.MaxAttrUses = max(plan.MaxAttrUses, int(ct.Attrs.Len))
	}
	plan.MaxAttrs = max(plan.MaxAttrUses+4, 8)
	for _, nfa := range schema.Models.NFA {
		plan.MaxModelWords = max(plan.MaxModelWords, int(nfa.Start.Len))
	}
	for _, all := range schema.Models.All {
		plan.MaxModelWords = max(plan.MaxModelWords, (len(all.Members)+63)/64)
	}
	for _, ic := range schema.ICs {
		plan.MaxIdentityFields = max(plan.MaxIdentityFields, int(ic.FieldLen))
	}
	return plan
}
