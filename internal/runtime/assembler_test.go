package runtime

import (
	"reflect"
	"testing"
)

func TestAssemblerSealMatchesDirectSchema(t *testing.T) {
	direct, directNS, directSym := assemblerBaseSchema(t)
	populateDirectSchema(direct, directNS, directSym)

	base, ns, sym := assemblerBaseSchema(t)
	assembler := NewAssembler(base)
	populateAssembledSchema(t, assembler, ns, sym)
	assembled, err := assembler.Seal()
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if !reflect.DeepEqual(assembled, direct) {
		t.Fatalf("assembled schema mismatch\nassembled=%#v\ndirect=%#v", assembled, direct)
	}
}

func TestAssemblerRejectsRepeatSealAndMutationAfterSeal(t *testing.T) {
	base, _, _ := assemblerBaseSchema(t)
	assembler := NewAssembler(base)
	if _, err := assembler.Seal(); err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := assembler.Seal(); err == nil {
		t.Fatalf("expected repeat Seal error")
	}
	if err := assembler.SetRootPolicy(RootAny); err == nil {
		t.Fatalf("expected mutation after Seal error")
	}
	if _, err := NewAssembler(nil).Seal(); err == nil {
		t.Fatalf("expected nil schema Seal error")
	}
}

func assemblerBaseSchema(t *testing.T) (*Schema, NamespaceID, SymbolID) {
	t.Helper()

	builder := NewBuilder()
	ns := mustInternNamespace(t, builder, []byte("urn:test"))
	sym := mustInternSymbol(t, builder, ns, []byte("root"))
	schema, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return schema, ns, sym
}

func populateDirectSchema(schema *Schema, ns NamespaceID, sym SymbolID) {
	schema.globalTypes = make([]TypeID, int(sym)+1)
	schema.globalElements = make([]ElemID, int(sym)+1)
	schema.globalAttributes = make([]AttrID, int(sym)+1)
	schema.globalTypes[sym] = 1
	schema.globalElements[sym] = 1
	schema.globalAttributes[sym] = 1
	schema.types = []Type{{}, {Name: sym, Kind: TypeSimple, Validator: 1}}
	schema.ancestors = TypeAncestors{IDs: []TypeID{1}, Masks: []DerivationMethod{DerRestriction}}
	schema.complexTypes = []ComplexType{{}, {Content: ContentElementOnly}}
	schema.elements = []Element{{}, {Name: sym, Type: 1, ICOff: 0, ICLen: 1}}
	schema.attributes = []Attribute{{}, {Name: sym, Validator: 1}}
	schema.attrIndex = ComplexAttrIndex{
		Uses:       []AttrUse{{Name: sym, Validator: 1}},
		HashTables: []AttrHashTable{{Hash: []uint64{1}, Slot: []uint32{1}}},
	}
	schema.validators = ValidatorsBundle{Meta: []ValidatorMeta{{}, {Kind: VString}}}
	schema.facets = []FacetInstr{{Op: FLength, Arg0: 3}}
	schema.patterns = []Pattern{{Source: []byte("x")}}
	schema.enums = EnumTable{Off: []uint32{0}, Len: []uint32{1}}
	schema.values = ValueBlob{Blob: []byte("abc")}
	schema.notations = []SymbolID{sym}
	schema.models = ModelsBundle{
		DFA:      []DFAModel{{}, {Start: 7}},
		NFA:      []NFAModel{{}, {Start: BitsetRef{Len: 1}}},
		All:      []AllModel{{}, {Members: []AllMember{{Elem: 1}}}},
		AllSubst: []ElemID{1},
	}
	schema.wildcards = []WildcardRule{{}, {PC: PCSkip}}
	schema.wildcardNS = []NamespaceID{ns}
	schema.identityConstraints = []IdentityConstraint{{}, {Name: sym, SelectorOff: 0, SelectorLen: 1, FieldOff: 0, FieldLen: 1}}
	schema.elementIdentityConstraints = []ICID{1}
	schema.identitySelectors = []PathID{1}
	schema.identityFields = []PathID{1}
	schema.paths = []PathProgram{{}, {Ops: []PathOp{{Op: OpSelf}}}}
	schema.builtin = BuiltinIDs{AnyType: 1, AnySimpleType: 1}
	schema.rootPolicy = RootAny
	schema.buildHash = 0x55
}

func populateAssembledSchema(t *testing.T, assembler *Assembler, ns NamespaceID, sym SymbolID) {
	t.Helper()

	globalTypes := make([]TypeID, int(sym)+1)
	globalElements := make([]ElemID, int(sym)+1)
	globalAttributes := make([]AttrID, int(sym)+1)
	globalTypes[sym] = 1
	globalElements[sym] = 1
	globalAttributes[sym] = 1
	mustSet(t, assembler.SetGlobalTypes(globalTypes))
	mustSet(t, assembler.SetGlobalElements(globalElements))
	mustSet(t, assembler.SetGlobalAttributes(globalAttributes))

	mustSet(t, assembler.SetTypes([]Type{{}}))
	typeID, err := assembler.AppendType(Type{Name: sym, Kind: TypeSimple, Validator: 1})
	mustSet(t, err)
	if typeID != 1 {
		t.Fatalf("AppendType ID = %d, want 1", typeID)
	}
	mustSet(t, assembler.AppendAncestor(1, DerRestriction))
	mustSet(t, assembler.SetComplexTypes([]ComplexType{{}}))
	complexID, err := assembler.AppendComplexType(ComplexType{Content: ContentElementOnly})
	mustSet(t, err)
	if complexID != 1 {
		t.Fatalf("AppendComplexType ID = %d, want 1", complexID)
	}
	mustSet(t, assembler.SetElements([]Element{{}}))
	elemID, err := assembler.AppendElement(Element{Name: sym, Type: 1, ICOff: 0, ICLen: 1})
	mustSet(t, err)
	if elemID != 1 {
		t.Fatalf("AppendElement ID = %d, want 1", elemID)
	}
	mustSet(t, assembler.SetAttributes([]Attribute{{}}))
	attrID, err := assembler.AppendAttribute(Attribute{Name: sym, Validator: 1})
	mustSet(t, err)
	if attrID != 1 {
		t.Fatalf("AppendAttribute ID = %d, want 1", attrID)
	}

	useOff, err := assembler.AppendAttrUse(AttrUse{Name: sym, Validator: 1})
	mustSet(t, err)
	if useOff != 0 {
		t.Fatalf("AppendAttrUse offset = %d, want 0", useOff)
	}
	hashOff, err := assembler.AppendAttrHashTable(AttrHashTable{Hash: []uint64{1}, Slot: []uint32{1}})
	mustSet(t, err)
	if hashOff != 0 {
		t.Fatalf("AppendAttrHashTable offset = %d, want 0", hashOff)
	}

	mustSet(t, assembler.SetValidators(ValidatorsBundle{Meta: []ValidatorMeta{{}, {Kind: VString}}}))
	facetOff, err := assembler.AppendFacet(FacetInstr{Op: FLength, Arg0: 3})
	mustSet(t, err)
	if facetOff != 0 {
		t.Fatalf("AppendFacet offset = %d, want 0", facetOff)
	}
	patternID, err := assembler.AppendPattern(Pattern{Source: []byte("x")})
	mustSet(t, err)
	if patternID != 0 {
		t.Fatalf("AppendPattern ID = %d, want 0", patternID)
	}
	mustSet(t, assembler.SetEnums(EnumTable{Off: []uint32{0}, Len: []uint32{1}}))
	valueRef, err := assembler.AppendValueBytes([]byte("abc"))
	mustSet(t, err)
	if valueRef.Off != 0 || valueRef.Len != 3 || !valueRef.Present {
		t.Fatalf("AppendValueBytes ref = %+v, want present 0/3", valueRef)
	}
	mustSet(t, assembler.AppendNotation(sym))

	mustSet(t, assembler.SetModels(ModelsBundle{
		DFA: []DFAModel{{}},
		NFA: []NFAModel{{}},
		All: []AllModel{{}},
	}))
	dfaRef, err := assembler.AppendDFAModel(DFAModel{Start: 7})
	mustSet(t, err)
	if dfaRef != (ModelRef{Kind: ModelDFA, ID: 1}) {
		t.Fatalf("AppendDFAModel ref = %+v, want DFA/1", dfaRef)
	}
	nfaRef, err := assembler.AppendNFAModel(NFAModel{Start: BitsetRef{Len: 1}})
	mustSet(t, err)
	if nfaRef != (ModelRef{Kind: ModelNFA, ID: 1}) {
		t.Fatalf("AppendNFAModel ref = %+v, want NFA/1", nfaRef)
	}
	allRef, err := assembler.AppendAllModel(AllModel{Members: []AllMember{{Elem: 1}}})
	mustSet(t, err)
	if allRef != (ModelRef{Kind: ModelAll, ID: 1}) {
		t.Fatalf("AppendAllModel ref = %+v, want All/1", allRef)
	}
	allSubstOff, err := assembler.AppendAllSubstitution(1)
	mustSet(t, err)
	if allSubstOff != 0 {
		t.Fatalf("AppendAllSubstitution offset = %d, want 0", allSubstOff)
	}

	mustSet(t, assembler.SetWildcards([]WildcardRule{{}}))
	wildcardID, err := assembler.AppendWildcard(WildcardRule{PC: PCSkip})
	mustSet(t, err)
	if wildcardID != 1 {
		t.Fatalf("AppendWildcard ID = %d, want 1", wildcardID)
	}
	wildcardNSOff, err := assembler.AppendWildcardNS(ns)
	mustSet(t, err)
	if wildcardNSOff != 0 {
		t.Fatalf("AppendWildcardNS offset = %d, want 0", wildcardNSOff)
	}

	mustSet(t, assembler.SetIdentityConstraints([]IdentityConstraint{{}}))
	icID, err := assembler.AppendIdentityConstraint(IdentityConstraint{Name: sym, SelectorOff: 0, SelectorLen: 1, FieldOff: 0, FieldLen: 1})
	mustSet(t, err)
	if icID != 1 {
		t.Fatalf("AppendIdentityConstraint ID = %d, want 1", icID)
	}
	elemICOff, err := assembler.AppendElementIdentityConstraint(1)
	mustSet(t, err)
	if elemICOff != 0 {
		t.Fatalf("AppendElementIdentityConstraint offset = %d, want 0", elemICOff)
	}
	selectorOff, err := assembler.AppendIdentitySelector(1)
	mustSet(t, err)
	if selectorOff != 0 {
		t.Fatalf("AppendIdentitySelector offset = %d, want 0", selectorOff)
	}
	fieldOff, err := assembler.AppendIdentityField(1)
	mustSet(t, err)
	if fieldOff != 0 {
		t.Fatalf("AppendIdentityField offset = %d, want 0", fieldOff)
	}

	mustSet(t, assembler.SetPaths([]PathProgram{{}}))
	pathID, err := assembler.AppendPath(PathProgram{Ops: []PathOp{{Op: OpSelf}}})
	mustSet(t, err)
	if pathID != 1 {
		t.Fatalf("AppendPath ID = %d, want 1", pathID)
	}
	mustSet(t, assembler.SetBuiltin(BuiltinIDs{AnyType: 1, AnySimpleType: 1}))
	mustSet(t, assembler.SetRootPolicy(RootAny))
	mustSet(t, assembler.SetBuildHash(0x55))
}

func mustSet(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("assembler mutation: %v", err)
	}
}
