package runtime

// FingerprintWriter receives the canonical runtime schema fingerprint stream.
type FingerprintWriter interface {
	WriteU8(uint8)
	WriteU32(uint32)
	WriteU64(uint64)
	WriteBool(bool)
	WriteBytes([]byte)
}

// WriteFingerprint writes the canonical schema fingerprint stream to w.
func WriteFingerprint(w FingerprintWriter, s *Schema) {
	if w == nil || s == nil {
		return
	}
	digestNamespaces(w, &s.Namespaces)
	digestSymbols(w, &s.Symbols)

	digestPredef(w, s.Predef, s.PredefNS, s.Builtin, s.RootPolicy)
	digestGlobalIndices(w, s.GlobalTypes, s.GlobalElements, s.GlobalAttributes)

	digestTypes(w, s.Types)
	digestAncestors(w, s.Ancestors)
	digestComplexTypes(w, s.ComplexTypes)
	digestElements(w, s.Elements)
	digestAttributes(w, s.Attributes)
	digestAttrIndex(w, s.AttrIndex)

	digestValidators(w, &s.Validators)
	digestFacets(w, s.Facets)
	digestPatterns(w, s.Patterns)
	digestEnums(w, &s.Enums)
	digestValues(w, s.Values)
	digestSymbolIDs(w, s.Notations)

	digestModels(w, s.Models)
	digestWildcards(w, s.Wildcards, s.WildcardNS)

	digestIdentity(w, s.ICs, s.ElemICs, s.ICSelectors, s.ICFields, s.Paths)
}
