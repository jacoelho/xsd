package schema

// RemoveProhibitedAttributeUses removes prohibited uses from the final
// attribute-use set while preserving the order of admitted uses.
func RemoveProhibitedAttributeUses(uses []AttributeUse) []AttributeUse {
	out := uses[:0]
	for _, use := range uses {
		if !use.Prohibited {
			out = append(out, use)
		}
	}
	return out
}
