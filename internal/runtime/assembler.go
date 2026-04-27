package runtime

import "fmt"

// Assembler is the mutable construction boundary for runtime schema tables.
type Assembler struct {
	schema *Schema
	sealed bool
}

func NewAssembler(base *Schema) *Assembler {
	return &Assembler{schema: base}
}

func NewSchemaAssembler() *Assembler {
	return NewAssembler(&Schema{})
}

func (a *Assembler) Seal() (*Schema, error) {
	if a == nil {
		return nil, fmt.Errorf("runtime assembler missing")
	}
	if a.sealed {
		return nil, fmt.Errorf("runtime assembler already sealed")
	}
	if a.schema == nil {
		return nil, fmt.Errorf("runtime assembler missing schema")
	}
	a.sealed = true
	return a.schema, nil
}

func (a *Assembler) SetSymbols(v SymbolsTable) error {
	return a.set(func(s *Schema) { s.symbols = v })
}

func (a *Assembler) SetNamespaces(v NamespaceTable) error {
	return a.set(func(s *Schema) { s.namespaces = v })
}

func (a *Assembler) SetGlobalTypes(v []TypeID) error {
	return a.set(func(s *Schema) { s.globalTypes = v })
}

func (a *Assembler) SetGlobalType(sym SymbolID, id TypeID) error {
	s, err := a.mutable()
	if err != nil {
		return err
	}
	if sym == 0 || int(sym) >= len(s.globalTypes) {
		return fmt.Errorf("global type symbol %d out of range", sym)
	}
	s.globalTypes[sym] = id
	return nil
}

func (a *Assembler) SetGlobalElements(v []ElemID) error {
	return a.set(func(s *Schema) { s.globalElements = v })
}

func (a *Assembler) SetGlobalElement(sym SymbolID, id ElemID) error {
	s, err := a.mutable()
	if err != nil {
		return err
	}
	if sym == 0 || int(sym) >= len(s.globalElements) {
		return fmt.Errorf("global element symbol %d out of range", sym)
	}
	s.globalElements[sym] = id
	return nil
}

func (a *Assembler) SetGlobalAttributes(v []AttrID) error {
	return a.set(func(s *Schema) { s.globalAttributes = v })
}

func (a *Assembler) SetGlobalAttribute(sym SymbolID, id AttrID) error {
	s, err := a.mutable()
	if err != nil {
		return err
	}
	if sym == 0 || int(sym) >= len(s.globalAttributes) {
		return fmt.Errorf("global attribute symbol %d out of range", sym)
	}
	s.globalAttributes[sym] = id
	return nil
}

func (a *Assembler) SetTypes(v []Type) error {
	return a.set(func(s *Schema) { s.types = v })
}

func (a *Assembler) SetType(id TypeID, v Type) error {
	s, err := a.mutable()
	if err != nil {
		return err
	}
	if id == 0 || int(id) >= len(s.types) {
		return fmt.Errorf("type %d out of range", id)
	}
	s.types[id] = v
	return nil
}

func (a *Assembler) AppendType(v Type) (TypeID, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := TypeID(len(s.types))
	s.types = append(s.types, v)
	return id, nil
}

func (a *Assembler) SetAncestors(v TypeAncestors) error {
	return a.set(func(s *Schema) { s.ancestors = v })
}

func (a *Assembler) AppendAncestor(id TypeID, mask DerivationMethod) error {
	s, err := a.mutable()
	if err != nil {
		return err
	}
	s.ancestors.IDs = append(s.ancestors.IDs, id)
	s.ancestors.Masks = append(s.ancestors.Masks, mask)
	return nil
}

func (a *Assembler) SetComplexTypes(v []ComplexType) error {
	return a.set(func(s *Schema) { s.complexTypes = v })
}

func (a *Assembler) SetComplexType(id uint32, v ComplexType) error {
	s, err := a.mutable()
	if err != nil {
		return err
	}
	if id == 0 || int(id) >= len(s.complexTypes) {
		return fmt.Errorf("complex type %d out of range", id)
	}
	s.complexTypes[id] = v
	return nil
}

func (a *Assembler) AppendComplexType(v ComplexType) (uint32, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := uint32(len(s.complexTypes))
	s.complexTypes = append(s.complexTypes, v)
	return id, nil
}

func (a *Assembler) SetElements(v []Element) error {
	return a.set(func(s *Schema) { s.elements = v })
}

func (a *Assembler) SetElement(id ElemID, v Element) error {
	s, err := a.mutable()
	if err != nil {
		return err
	}
	if id == 0 || int(id) >= len(s.elements) {
		return fmt.Errorf("element %d out of range", id)
	}
	s.elements[id] = v
	return nil
}

func (a *Assembler) AppendElement(v Element) (ElemID, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := ElemID(len(s.elements))
	s.elements = append(s.elements, v)
	return id, nil
}

func (a *Assembler) SetAttributes(v []Attribute) error {
	return a.set(func(s *Schema) { s.attributes = v })
}

func (a *Assembler) SetAttribute(id AttrID, v Attribute) error {
	s, err := a.mutable()
	if err != nil {
		return err
	}
	if id == 0 || int(id) >= len(s.attributes) {
		return fmt.Errorf("attribute %d out of range", id)
	}
	s.attributes[id] = v
	return nil
}

func (a *Assembler) AppendAttribute(v Attribute) (AttrID, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := AttrID(len(s.attributes))
	s.attributes = append(s.attributes, v)
	return id, nil
}

func (a *Assembler) SetAttrIndex(v ComplexAttrIndex) error {
	return a.set(func(s *Schema) { s.attrIndex = v })
}

func (a *Assembler) AppendAttrUse(v AttrUse) (uint32, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := uint32(len(s.attrIndex.Uses))
	s.attrIndex.Uses = append(s.attrIndex.Uses, v)
	return id, nil
}

func (a *Assembler) AppendAttrHashTable(v AttrHashTable) (uint32, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := uint32(len(s.attrIndex.HashTables))
	s.attrIndex.HashTables = append(s.attrIndex.HashTables, v)
	return id, nil
}

func (a *Assembler) SetValidators(v ValidatorsBundle) error {
	return a.set(func(s *Schema) { s.validators = v })
}

func (a *Assembler) SetFacets(v []FacetInstr) error {
	return a.set(func(s *Schema) { s.facets = v })
}

func (a *Assembler) AppendFacet(v FacetInstr) (uint32, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := uint32(len(s.facets))
	s.facets = append(s.facets, v)
	return id, nil
}

func (a *Assembler) SetPatterns(v []Pattern) error {
	return a.set(func(s *Schema) { s.patterns = v })
}

func (a *Assembler) AppendPattern(v Pattern) (PatternID, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := PatternID(len(s.patterns))
	s.patterns = append(s.patterns, v)
	return id, nil
}

func (a *Assembler) SetEnums(v EnumTable) error {
	return a.set(func(s *Schema) { s.enums = v })
}

func (a *Assembler) SetValues(v ValueBlob) error {
	return a.set(func(s *Schema) { s.values = v })
}

func (a *Assembler) AppendValueBytes(v []byte) (ValueRef, error) {
	s, err := a.mutable()
	if err != nil {
		return ValueRef{}, err
	}
	off := uint32(len(s.values.Blob))
	s.values.Blob = append(s.values.Blob, v...)
	return ValueRef{Off: off, Len: uint32(len(v)), Present: true}, nil
}

func (a *Assembler) SetNotations(v []SymbolID) error {
	return a.set(func(s *Schema) { s.notations = v })
}

func (a *Assembler) AppendNotation(v SymbolID) error {
	s, err := a.mutable()
	if err != nil {
		return err
	}
	s.notations = append(s.notations, v)
	return nil
}

func (a *Assembler) SetModels(v ModelsBundle) error {
	return a.set(func(s *Schema) { s.models = v })
}

func (a *Assembler) AppendDFAModel(v DFAModel) (ModelRef, error) {
	s, err := a.mutable()
	if err != nil {
		return ModelRef{}, err
	}
	id := uint32(len(s.models.DFA))
	s.models.DFA = append(s.models.DFA, v)
	return ModelRef{Kind: ModelDFA, ID: id}, nil
}

func (a *Assembler) AppendNFAModel(v NFAModel) (ModelRef, error) {
	s, err := a.mutable()
	if err != nil {
		return ModelRef{}, err
	}
	id := uint32(len(s.models.NFA))
	s.models.NFA = append(s.models.NFA, v)
	return ModelRef{Kind: ModelNFA, ID: id}, nil
}

func (a *Assembler) AppendAllModel(v AllModel) (ModelRef, error) {
	s, err := a.mutable()
	if err != nil {
		return ModelRef{}, err
	}
	id := uint32(len(s.models.All))
	s.models.All = append(s.models.All, v)
	return ModelRef{Kind: ModelAll, ID: id}, nil
}

func (a *Assembler) AppendAllSubstitution(v ElemID) (uint32, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := uint32(len(s.models.AllSubst))
	s.models.AllSubst = append(s.models.AllSubst, v)
	return id, nil
}

func (a *Assembler) SetWildcards(v []WildcardRule) error {
	return a.set(func(s *Schema) { s.wildcards = v })
}

func (a *Assembler) AppendWildcard(v WildcardRule) (WildcardID, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := WildcardID(len(s.wildcards))
	s.wildcards = append(s.wildcards, v)
	return id, nil
}

func (a *Assembler) SetWildcardNS(v []NamespaceID) error {
	return a.set(func(s *Schema) { s.wildcardNS = v })
}

func (a *Assembler) AppendWildcardNS(v NamespaceID) (uint32, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := uint32(len(s.wildcardNS))
	s.wildcardNS = append(s.wildcardNS, v)
	return id, nil
}

func (a *Assembler) SetIdentityConstraints(v []IdentityConstraint) error {
	return a.set(func(s *Schema) { s.identityConstraints = v })
}

func (a *Assembler) AppendIdentityConstraint(v IdentityConstraint) (ICID, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := ICID(len(s.identityConstraints))
	s.identityConstraints = append(s.identityConstraints, v)
	return id, nil
}

func (a *Assembler) SetElementIdentityConstraints(v []ICID) error {
	return a.set(func(s *Schema) { s.elementIdentityConstraints = v })
}

func (a *Assembler) AppendElementIdentityConstraint(v ICID) (uint32, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := uint32(len(s.elementIdentityConstraints))
	s.elementIdentityConstraints = append(s.elementIdentityConstraints, v)
	return id, nil
}

func (a *Assembler) SetIdentitySelectors(v []PathID) error {
	return a.set(func(s *Schema) { s.identitySelectors = v })
}

func (a *Assembler) AppendIdentitySelector(v PathID) (uint32, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := uint32(len(s.identitySelectors))
	s.identitySelectors = append(s.identitySelectors, v)
	return id, nil
}

func (a *Assembler) SetIdentityFields(v []PathID) error {
	return a.set(func(s *Schema) { s.identityFields = v })
}

func (a *Assembler) AppendIdentityField(v PathID) (uint32, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := uint32(len(s.identityFields))
	s.identityFields = append(s.identityFields, v)
	return id, nil
}

func (a *Assembler) SetPaths(v []PathProgram) error {
	return a.set(func(s *Schema) { s.paths = v })
}

func (a *Assembler) AppendPath(v PathProgram) (PathID, error) {
	s, err := a.mutable()
	if err != nil {
		return 0, err
	}
	id := PathID(len(s.paths))
	s.paths = append(s.paths, v)
	return id, nil
}

func (a *Assembler) SetPredefinedSymbols(v PredefinedSymbols) error {
	return a.set(func(s *Schema) { s.predef = v })
}

func (a *Assembler) SetPredefinedNamespaces(v PredefinedNamespaces) error {
	return a.set(func(s *Schema) { s.predefNS = v })
}

func (a *Assembler) SetBuiltin(v BuiltinIDs) error {
	return a.set(func(s *Schema) { s.builtin = v })
}

func (a *Assembler) SetRootPolicy(v RootPolicy) error {
	return a.set(func(s *Schema) { s.rootPolicy = v })
}

func (a *Assembler) SetBuildHash(v uint64) error {
	return a.set(func(s *Schema) { s.buildHash = v })
}

func (a *Assembler) set(apply func(*Schema)) error {
	s, err := a.mutable()
	if err != nil {
		return err
	}
	apply(s)
	return nil
}

func (a *Assembler) mutable() (*Schema, error) {
	if a == nil {
		return nil, fmt.Errorf("runtime assembler missing")
	}
	if a.sealed {
		return nil, fmt.Errorf("runtime assembler already sealed")
	}
	if a.schema == nil {
		return nil, fmt.Errorf("runtime assembler missing schema")
	}
	return a.schema, nil
}
