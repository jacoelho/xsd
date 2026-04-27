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
	namespaces := s.NamespaceTable()
	symbols := s.SymbolsTable()
	validators := s.ValidatorBundle()
	enums := s.EnumTable()

	digestNamespaces(w, &namespaces)
	digestSymbols(w, &symbols)

	digestPredef(w, s.PredefinedSymbols(), s.PredefinedNamespaces(), s.BuiltinTypes(), s.RootPolicyValue())
	digestGlobalIndices(w, s.GlobalTypeIDs(), s.GlobalElementIDs(), s.GlobalAttributeIDs())

	digestTypes(w, s.TypeTable())
	digestAncestors(w, s.AncestorTable())
	digestComplexTypes(w, s.ComplexTypeTable())
	digestElements(w, s.ElementTable())
	digestAttributes(w, s.AttributeTable())
	digestAttrIndex(w, s.AttributeIndex())

	digestValidators(w, &validators)
	digestFacets(w, s.FacetTable())
	digestPatterns(w, s.PatternTable())
	digestEnums(w, &enums)
	digestValues(w, s.ValueBlob())
	digestSymbolIDs(w, s.NotationSymbols())

	digestModels(w, s.ModelBundle())
	digestWildcards(w, s.WildcardTable(), s.WildcardNamespaces())

	digestIdentity(w, s.IdentityConstraints(), s.ElementIdentityConstraints(), s.IdentitySelectors(), s.IdentityFields(), s.PathPrograms())
}
