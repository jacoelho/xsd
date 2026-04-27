package runtime

// SymbolsTable returns the interned symbol table.
func (s *Schema) SymbolsTable() SymbolsTable {
	if s == nil {
		return SymbolsTable{}
	}
	return s.symbols
}

// NamespaceTable returns the interned namespace table.
func (s *Schema) NamespaceTable() NamespaceTable {
	if s == nil {
		return NamespaceTable{}
	}
	return s.namespaces
}

func (s *Schema) GlobalTypeIDs() []TypeID {
	if s == nil {
		return nil
	}
	return s.globalTypes
}

func (s *Schema) GlobalElementIDs() []ElemID {
	if s == nil {
		return nil
	}
	return s.globalElements
}

func (s *Schema) GlobalAttributeIDs() []AttrID {
	if s == nil {
		return nil
	}
	return s.globalAttributes
}

func (s *Schema) TypeTable() []Type {
	if s == nil {
		return nil
	}
	return s.types
}

func (s *Schema) AncestorTable() TypeAncestors {
	if s == nil {
		return TypeAncestors{}
	}
	return s.ancestors
}

func (s *Schema) ComplexTypeTable() []ComplexType {
	if s == nil {
		return nil
	}
	return s.complexTypes
}

func (s *Schema) ElementTable() []Element {
	if s == nil {
		return nil
	}
	return s.elements
}

func (s *Schema) AttributeTable() []Attribute {
	if s == nil {
		return nil
	}
	return s.attributes
}

func (s *Schema) AttributeIndex() ComplexAttrIndex {
	if s == nil {
		return ComplexAttrIndex{}
	}
	return s.attrIndex
}

func (s *Schema) ValidatorBundle() ValidatorsBundle {
	if s == nil {
		return ValidatorsBundle{}
	}
	return s.validators
}

func (s *Schema) FacetTable() []FacetInstr {
	if s == nil {
		return nil
	}
	return s.facets
}

func (s *Schema) PatternTable() []Pattern {
	if s == nil {
		return nil
	}
	return s.patterns
}

func (s *Schema) EnumTable() EnumTable {
	if s == nil {
		return EnumTable{}
	}
	return s.enums
}

func (s *Schema) ValueBlob() ValueBlob {
	if s == nil {
		return ValueBlob{}
	}
	return s.values
}

func (s *Schema) NotationSymbols() []SymbolID {
	if s == nil {
		return nil
	}
	return s.notations
}

func (s *Schema) ModelBundle() ModelsBundle {
	if s == nil {
		return ModelsBundle{}
	}
	return s.models
}

func (s *Schema) WildcardTable() []WildcardRule {
	if s == nil {
		return nil
	}
	return s.wildcards
}

func (s *Schema) WildcardNamespaces() []NamespaceID {
	if s == nil {
		return nil
	}
	return s.wildcardNS
}

func (s *Schema) IdentityConstraints() []IdentityConstraint {
	if s == nil {
		return nil
	}
	return s.identityConstraints
}

func (s *Schema) ElementIdentityConstraints() []ICID {
	if s == nil {
		return nil
	}
	return s.elementIdentityConstraints
}

func (s *Schema) IdentitySelectors() []PathID {
	if s == nil {
		return nil
	}
	return s.identitySelectors
}

func (s *Schema) IdentityFields() []PathID {
	if s == nil {
		return nil
	}
	return s.identityFields
}

func (s *Schema) PathPrograms() []PathProgram {
	if s == nil {
		return nil
	}
	return s.paths
}

func (s *Schema) PredefinedSymbols() PredefinedSymbols {
	if s == nil {
		return PredefinedSymbols{}
	}
	return s.predef
}

func (s *Schema) KnownSymbols() PredefinedSymbols {
	return s.PredefinedSymbols()
}

func (s *Schema) PredefinedNamespaces() PredefinedNamespaces {
	if s == nil {
		return PredefinedNamespaces{}
	}
	return s.predefNS
}

func (s *Schema) KnownNamespaces() PredefinedNamespaces {
	return s.PredefinedNamespaces()
}

func (s *Schema) BuiltinTypes() BuiltinIDs {
	if s == nil {
		return BuiltinIDs{}
	}
	return s.builtin
}

func (s *Schema) BuiltinTypeIDs() BuiltinIDs {
	return s.BuiltinTypes()
}

func (s *Schema) RootPolicyValue() RootPolicy {
	if s == nil {
		return 0
	}
	return s.rootPolicy
}

func (s *Schema) RootMode() RootPolicy {
	return s.RootPolicyValue()
}

func (s *Schema) BuildHashValue() uint64 {
	if s == nil {
		return 0
	}
	return s.buildHash
}

func (s *Schema) BuildFingerprint() uint64 {
	return s.BuildHashValue()
}

func (s *Schema) Type(id TypeID) (Type, bool) {
	i, ok := checkedID(id, s.typeCount())
	if !ok {
		return Type{}, false
	}
	return s.types[i], true
}

func (s *Schema) ElementRef(id ElemID) (*Element, bool) {
	i, ok := checkedID(id, s.elementCount())
	if !ok {
		return nil, false
	}
	return &s.elements[i], true
}

func (s *Schema) Element(id ElemID) (Element, bool) {
	i, ok := checkedID(id, s.elementCount())
	if !ok {
		return Element{}, false
	}
	return s.elements[i], true
}

func (s *Schema) Attribute(id AttrID) (Attribute, bool) {
	i, ok := checkedID(id, s.attributeCount())
	if !ok {
		return Attribute{}, false
	}
	return s.attributes[i], true
}

func (s *Schema) ComplexType(id uint32) (ComplexType, bool) {
	i, ok := checkedID32(id, s.complexTypeCount())
	if !ok {
		return ComplexType{}, false
	}
	return s.complexTypes[i], true
}

func (s *Schema) ValidatorMeta(id ValidatorID) (ValidatorMeta, bool) {
	if s == nil {
		return ValidatorMeta{}, false
	}
	i, ok := checkedID(id, len(s.validators.Meta))
	if !ok {
		return ValidatorMeta{}, false
	}
	return s.validators.Meta[i], true
}

func (s *Schema) DFAModelByRef(ref ModelRef) (*DFAModel, bool) {
	if s == nil || ref.Kind != ModelDFA {
		return nil, false
	}
	i, ok := checkedID32(ref.ID, len(s.models.DFA))
	if !ok {
		return nil, false
	}
	return &s.models.DFA[i], true
}

func (s *Schema) NFAModelByRef(ref ModelRef) (*NFAModel, bool) {
	if s == nil || ref.Kind != ModelNFA {
		return nil, false
	}
	i, ok := checkedID32(ref.ID, len(s.models.NFA))
	if !ok {
		return nil, false
	}
	return &s.models.NFA[i], true
}

func (s *Schema) AllModelByRef(ref ModelRef) (*AllModel, bool) {
	if s == nil || ref.Kind != ModelAll {
		return nil, false
	}
	i, ok := checkedID32(ref.ID, len(s.models.All))
	if !ok {
		return nil, false
	}
	return &s.models.All[i], true
}

func (s *Schema) Wildcard(id WildcardID) (WildcardRule, bool) {
	i, ok := checkedID(id, s.wildcardCount())
	if !ok {
		return WildcardRule{}, false
	}
	return s.wildcards[i], true
}

func (s *Schema) IdentityConstraint(id ICID) (IdentityConstraint, bool) {
	i, ok := checkedID(id, s.identityConstraintCount())
	if !ok {
		return IdentityConstraint{}, false
	}
	return s.identityConstraints[i], true
}

func (s *Schema) Path(id PathID) (PathProgram, bool) {
	i, ok := checkedID(id, s.pathCount())
	if !ok {
		return PathProgram{}, false
	}
	return s.paths[i], true
}

func (s *Schema) GlobalType(sym SymbolID) (TypeID, bool) {
	i, ok := checkedID(sym, s.globalTypeCount())
	if !ok {
		return 0, false
	}
	id := s.globalTypes[i]
	return id, id != 0
}

func (s *Schema) GlobalElement(sym SymbolID) (ElemID, bool) {
	i, ok := checkedID(sym, s.globalElementCount())
	if !ok {
		return 0, false
	}
	id := s.globalElements[i]
	return id, id != 0
}

func (s *Schema) GlobalAttribute(sym SymbolID) (AttrID, bool) {
	i, ok := checkedID(sym, s.globalAttributeCount())
	if !ok {
		return 0, false
	}
	id := s.globalAttributes[i]
	return id, id != 0
}

func (s *Schema) NamespaceLookup(ns []byte) NamespaceID {
	if s == nil {
		return 0
	}
	return s.namespaces.Lookup(ns)
}

func (s *Schema) NamespaceBytes(id NamespaceID) []byte {
	if s == nil {
		return nil
	}
	return s.namespaces.Bytes(id)
}

func (s *Schema) SymbolLookup(nsID NamespaceID, local []byte) SymbolID {
	if s == nil {
		return 0
	}
	return s.symbols.Lookup(nsID, local)
}

func (s *Schema) SymbolLocalBytes(id SymbolID) []byte {
	if s == nil {
		return nil
	}
	return s.symbols.LocalBytes(id)
}

func (s *Schema) SymbolNamespace(id SymbolID) (NamespaceID, bool) {
	i, ok := checkedID(id, s.symbolSlotCount())
	if !ok {
		return 0, false
	}
	ns := s.symbols.NS[i]
	return ns, ns != 0
}

func (s *Schema) SymbolBytes(id SymbolID) (NamespaceID, []byte, bool) {
	ns, ok := s.SymbolNamespace(id)
	if !ok {
		return 0, nil, false
	}
	local := s.SymbolLocalBytes(id)
	if local == nil {
		return 0, nil, false
	}
	return ns, local, true
}

func (s *Schema) AttributeUses(ref AttrIndexRef) []AttrUse {
	if s == nil || ref.Len == 0 {
		return nil
	}
	start, end, ok := checkedSpan(ref.Off, ref.Len, len(s.attrIndex.Uses))
	if !ok || start == len(s.attrIndex.Uses) {
		return nil
	}
	return s.attrIndex.Uses[start:end]
}

func (s *Schema) AttributeHashTable(id uint32) (AttrHashTable, bool) {
	if s == nil || uint64(id) >= uint64(len(s.attrIndex.HashTables)) {
		return AttrHashTable{}, false
	}
	return s.attrIndex.HashTables[id], true
}

func (s *Schema) AncestorIDs(off, length uint32) []TypeID {
	if s == nil {
		return nil
	}
	start, end, ok := checkedSpan(off, length, len(s.ancestors.IDs))
	if !ok {
		return nil
	}
	return s.ancestors.IDs[start:end]
}

func (s *Schema) AncestorMasks(off, length uint32) []DerivationMethod {
	if s == nil {
		return nil
	}
	start, end, ok := checkedSpan(off, length, len(s.ancestors.Masks))
	if !ok {
		return nil
	}
	return s.ancestors.Masks[start:end]
}

func (s *Schema) FacetProgram(ref FacetProgramRef) []FacetInstr {
	if s == nil {
		return nil
	}
	start, end, ok := checkedSpan(ref.Off, ref.Len, len(s.facets))
	if !ok {
		return nil
	}
	return s.facets[start:end]
}

func (s *Schema) Value(ref ValueRef) []byte {
	if s == nil || !ref.Present {
		return nil
	}
	start, end, ok := checkedSpan(ref.Off, ref.Len, len(s.values.Blob))
	if !ok {
		return nil
	}
	return s.values.Blob[start:end]
}

func (s *Schema) AllSubstitutions(off, length uint32) []ElemID {
	if s == nil {
		return nil
	}
	start, end, ok := checkedSpan(off, length, len(s.models.AllSubst))
	if !ok {
		return nil
	}
	return s.models.AllSubst[start:end]
}

func (s *Schema) WildcardNamespaceSpan(ref NSConstraint) []NamespaceID {
	if s == nil {
		return nil
	}
	start, end, ok := checkedSpan(ref.Off, ref.Len, len(s.wildcardNS))
	if !ok {
		return nil
	}
	return s.wildcardNS[start:end]
}

func (s *Schema) ElementIdentityConstraintIDs(elem Element) []ICID {
	if s == nil {
		return nil
	}
	start, end, ok := checkedSpan(elem.ICOff, elem.ICLen, len(s.elementIdentityConstraints))
	if !ok {
		return nil
	}
	return s.elementIdentityConstraints[start:end]
}

func (s *Schema) IdentitySelectorPathIDs(ic IdentityConstraint) []PathID {
	if s == nil {
		return nil
	}
	start, end, ok := checkedSpan(ic.SelectorOff, ic.SelectorLen, len(s.identitySelectors))
	if !ok {
		return nil
	}
	return s.identitySelectors[start:end]
}

func (s *Schema) IdentityFieldPathIDs(ic IdentityConstraint) []PathID {
	if s == nil {
		return nil
	}
	start, end, ok := checkedSpan(ic.FieldOff, ic.FieldLen, len(s.identityFields))
	if !ok {
		return nil
	}
	return s.identityFields[start:end]
}

func (s *Schema) SymbolCount() int {
	if s == nil {
		return 0
	}
	return s.symbols.Count()
}

func (s *Schema) NamespaceCount() int {
	if s == nil {
		return 0
	}
	return s.namespaces.Count()
}

func (s *Schema) GlobalTypeCount() int      { return s.globalTypeCount() }
func (s *Schema) GlobalElementCount() int   { return s.globalElementCount() }
func (s *Schema) GlobalAttributeCount() int { return s.globalAttributeCount() }
func (s *Schema) TypeCount() int            { return s.typeCount() }
func (s *Schema) AncestorIDCount() int {
	if s == nil {
		return 0
	}
	return len(s.ancestors.IDs)
}
func (s *Schema) AncestorMaskCount() int {
	if s == nil {
		return 0
	}
	return len(s.ancestors.Masks)
}
func (s *Schema) ComplexTypeCount() int { return s.complexTypeCount() }
func (s *Schema) ElementCount() int     { return s.elementCount() }
func (s *Schema) AttributeCount() int   { return s.attributeCount() }
func (s *Schema) AttributeUseCount() int {
	if s == nil {
		return 0
	}
	return len(s.attrIndex.Uses)
}
func (s *Schema) AttributeHashTableCount() int {
	if s == nil {
		return 0
	}
	return len(s.attrIndex.HashTables)
}
func (s *Schema) ValidatorCount() int {
	if s == nil {
		return 0
	}
	return len(s.validators.Meta)
}
func (s *Schema) FacetCount() int {
	if s == nil {
		return 0
	}
	return len(s.facets)
}
func (s *Schema) PatternCount() int {
	if s == nil {
		return 0
	}
	return len(s.patterns)
}
func (s *Schema) ValueByteCount() int {
	if s == nil {
		return 0
	}
	return len(s.values.Blob)
}
func (s *Schema) NotationCount() int {
	if s == nil {
		return 0
	}
	return len(s.notations)
}
func (s *Schema) DFAModelCount() int {
	if s == nil {
		return 0
	}
	return len(s.models.DFA)
}
func (s *Schema) NFAModelCount() int {
	if s == nil {
		return 0
	}
	return len(s.models.NFA)
}
func (s *Schema) AllModelCount() int {
	if s == nil {
		return 0
	}
	return len(s.models.All)
}
func (s *Schema) AllSubstitutionCount() int {
	if s == nil {
		return 0
	}
	return len(s.models.AllSubst)
}
func (s *Schema) WildcardCount() int { return s.wildcardCount() }
func (s *Schema) WildcardNamespaceCount() int {
	if s == nil {
		return 0
	}
	return len(s.wildcardNS)
}
func (s *Schema) IdentityConstraintCount() int { return s.identityConstraintCount() }
func (s *Schema) ElementIdentityConstraintCount() int {
	if s == nil {
		return 0
	}
	return len(s.elementIdentityConstraints)
}
func (s *Schema) IdentitySelectorCount() int {
	if s == nil {
		return 0
	}
	return len(s.identitySelectors)
}
func (s *Schema) IdentityFieldCount() int {
	if s == nil {
		return 0
	}
	return len(s.identityFields)
}
func (s *Schema) PathCount() int { return s.pathCount() }

func (s *Schema) globalTypeCount() int {
	if s == nil {
		return 0
	}
	return len(s.globalTypes)
}

func (s *Schema) globalElementCount() int {
	if s == nil {
		return 0
	}
	return len(s.globalElements)
}

func (s *Schema) globalAttributeCount() int {
	if s == nil {
		return 0
	}
	return len(s.globalAttributes)
}

func (s *Schema) typeCount() int {
	if s == nil {
		return 0
	}
	return len(s.types)
}

func (s *Schema) complexTypeCount() int {
	if s == nil {
		return 0
	}
	return len(s.complexTypes)
}

func (s *Schema) elementCount() int {
	if s == nil {
		return 0
	}
	return len(s.elements)
}

func (s *Schema) attributeCount() int {
	if s == nil {
		return 0
	}
	return len(s.attributes)
}

func (s *Schema) wildcardCount() int {
	if s == nil {
		return 0
	}
	return len(s.wildcards)
}

func (s *Schema) identityConstraintCount() int {
	if s == nil {
		return 0
	}
	return len(s.identityConstraints)
}

func (s *Schema) pathCount() int {
	if s == nil {
		return 0
	}
	return len(s.paths)
}

func (s *Schema) symbolSlotCount() int {
	if s == nil {
		return 0
	}
	return len(s.symbols.NS)
}

func checkedID[T ~uint32](id T, size int) (int, bool) {
	return checkedID32(uint32(id), size)
}

func checkedID32(id uint32, size int) (int, bool) {
	if id == 0 || uint64(id) >= uint64(size) {
		return 0, false
	}
	return int(id), true
}

func checkedSpan(off, length uint32, size int) (int, int, bool) {
	end := uint64(off) + uint64(length)
	if end > uint64(size) {
		return 0, 0, false
	}
	return int(off), int(end), true
}
