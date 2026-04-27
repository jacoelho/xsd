package runtime

import (
	"bytes"
	"testing"
)

func TestSchemaAccessorsValidLookups(t *testing.T) {
	schema, sym := buildAccessorSchema(t)

	if schema.SymbolCount() == 0 {
		t.Fatalf("SymbolCount = 0")
	}
	if schema.NamespaceCount() == 0 {
		t.Fatalf("NamespaceCount = 0")
	}
	if schema.TypeCount() != 2 {
		t.Fatalf("TypeCount = %d, want 2", schema.TypeCount())
	}
	if schema.AncestorIDCount() != 1 || schema.AncestorMaskCount() != 1 {
		t.Fatalf("ancestor counts = %d/%d, want 1/1", schema.AncestorIDCount(), schema.AncestorMaskCount())
	}
	if schema.AttributeUseCount() != 1 || schema.AttributeHashTableCount() != 1 {
		t.Fatalf("attribute index counts = %d/%d, want 1/1", schema.AttributeUseCount(), schema.AttributeHashTableCount())
	}
	if schema.AllSubstitutionCount() != 1 || schema.WildcardNamespaceCount() != 1 {
		t.Fatalf("model/wildcard counts = %d/%d, want 1/1", schema.AllSubstitutionCount(), schema.WildcardNamespaceCount())
	}
	if schema.ElementIdentityConstraintCount() != 1 || schema.IdentitySelectorCount() != 1 || schema.IdentityFieldCount() != 1 {
		t.Fatalf("identity counts = %d/%d/%d, want 1/1/1", schema.ElementIdentityConstraintCount(), schema.IdentitySelectorCount(), schema.IdentityFieldCount())
	}
	if schema.BuildHashValue() != 0x1234 {
		t.Fatalf("BuildHashValue = %x, want 1234", schema.BuildHashValue())
	}
	if schema.RootPolicyValue() != RootAny {
		t.Fatalf("RootPolicyValue = %d, want RootAny", schema.RootPolicyValue())
	}
	symbols := schema.SymbolsTable()
	if symbols.Count() != schema.symbols.Count() {
		t.Fatalf("SymbolsTable mismatch")
	}
	namespaces := schema.NamespaceTable()
	if namespaces.Count() != schema.namespaces.Count() {
		t.Fatalf("NamespaceTable mismatch")
	}
	if len(schema.GlobalTypeIDs()) != len(schema.globalTypes) ||
		len(schema.GlobalElementIDs()) != len(schema.globalElements) ||
		len(schema.GlobalAttributeIDs()) != len(schema.globalAttributes) {
		t.Fatalf("global index accessors returned wrong lengths")
	}
	if len(schema.TypeTable()) != len(schema.types) ||
		len(schema.ComplexTypeTable()) != len(schema.complexTypes) ||
		len(schema.ElementTable()) != len(schema.elements) ||
		len(schema.AttributeTable()) != len(schema.attributes) {
		t.Fatalf("descriptor table accessors returned wrong lengths")
	}
	if schema.PredefinedSymbols() != schema.predef ||
		schema.PredefinedNamespaces() != schema.predefNS ||
		schema.BuiltinTypes() != schema.builtin {
		t.Fatalf("predefined accessor mismatch")
	}

	ns := schema.NamespaceLookup([]byte("urn:test"))
	if ns == 0 {
		t.Fatalf("NamespaceLookup returned zero")
	}
	if !bytes.Equal(schema.NamespaceBytes(ns), []byte("urn:test")) {
		t.Fatalf("NamespaceBytes mismatch")
	}
	if got := schema.SymbolLookup(ns, []byte("root")); got != sym {
		t.Fatalf("SymbolLookup = %d, want %d", got, sym)
	}
	symNS, local, ok := schema.SymbolBytes(sym)
	if !ok || symNS != ns || !bytes.Equal(local, []byte("root")) {
		t.Fatalf("SymbolBytes = %d %q %v, want %d root true", symNS, local, ok, ns)
	}

	if id, ok := schema.GlobalType(sym); !ok || id != 1 {
		t.Fatalf("GlobalType = %d %v, want 1 true", id, ok)
	}
	if id, ok := schema.GlobalElement(sym); !ok || id != 1 {
		t.Fatalf("GlobalElement = %d %v, want 1 true", id, ok)
	}
	if id, ok := schema.GlobalAttribute(sym); !ok || id != 1 {
		t.Fatalf("GlobalAttribute = %d %v, want 1 true", id, ok)
	}

	if typ, ok := schema.Type(1); !ok || typ.Kind != TypeSimple {
		t.Fatalf("Type = %+v %v, want simple true", typ, ok)
	}
	if ct, ok := schema.ComplexType(1); !ok || ct.Content != ContentElementOnly {
		t.Fatalf("ComplexType = %+v %v, want element-only true", ct, ok)
	}
	if elem, ok := schema.Element(1); !ok || elem.Name != sym {
		t.Fatalf("Element = %+v %v, want symbol true", elem, ok)
	}
	if attr, ok := schema.Attribute(1); !ok || attr.Name != sym {
		t.Fatalf("Attribute = %+v %v, want symbol true", attr, ok)
	}
	if meta, ok := schema.ValidatorMeta(1); !ok || meta.Kind != VString {
		t.Fatalf("ValidatorMeta = %+v %v, want string true", meta, ok)
	}

	if model, ok := schema.DFAModelByRef(ModelRef{Kind: ModelDFA, ID: 1}); !ok || model.Start != 7 {
		t.Fatalf("DFAModelByRef = %+v %v, want start 7 true", model, ok)
	}
	if model, ok := schema.NFAModelByRef(ModelRef{Kind: ModelNFA, ID: 1}); !ok || model.Start.Len != 1 {
		t.Fatalf("NFAModelByRef = %+v %v, want len 1 true", model, ok)
	}
	if model, ok := schema.AllModelByRef(ModelRef{Kind: ModelAll, ID: 1}); !ok || len(model.Members) != 1 {
		t.Fatalf("AllModelByRef = %+v %v, want one member true", model, ok)
	}
	if wildcard, ok := schema.Wildcard(1); !ok || wildcard.PC != PCSkip {
		t.Fatalf("Wildcard = %+v %v, want skip true", wildcard, ok)
	}
	if ic, ok := schema.IdentityConstraint(1); !ok || ic.Name != sym {
		t.Fatalf("IdentityConstraint = %+v %v, want symbol true", ic, ok)
	}
	if path, ok := schema.Path(1); !ok || len(path.Ops) != 1 {
		t.Fatalf("Path = %+v %v, want one op true", path, ok)
	}

	if len(schema.AttributeUses(AttrIndexRef{Off: 0, Len: 1})) != 1 {
		t.Fatalf("AttributeUses did not return one use")
	}
	if _, ok := schema.AttributeHashTable(0); !ok {
		t.Fatalf("AttributeHashTable(0) returned false")
	}
	if len(schema.AncestorIDs(0, 1)) != 1 || len(schema.AncestorMasks(0, 1)) != 1 {
		t.Fatalf("ancestor span accessors failed")
	}
	if len(schema.FacetProgram(FacetProgramRef{Off: 0, Len: 1})) != 1 {
		t.Fatalf("FacetProgram did not return one instr")
	}
	if !bytes.Equal(schema.Value(ValueRef{Off: 0, Len: 3, Present: true}), []byte("abc")) {
		t.Fatalf("Value did not return bytes")
	}
	if len(schema.AllSubstitutions(0, 1)) != 1 {
		t.Fatalf("AllSubstitutions did not return one elem")
	}
	if len(schema.WildcardNamespaceSpan(NSConstraint{Off: 0, Len: 1})) != 1 {
		t.Fatalf("WildcardNamespaceSpan did not return one namespace")
	}
	if len(schema.ElementIdentityConstraintIDs(Element{ICOff: 0, ICLen: 1})) != 1 {
		t.Fatalf("ElementIdentityConstraintIDs did not return one id")
	}
	if len(schema.IdentitySelectorPathIDs(IdentityConstraint{SelectorOff: 0, SelectorLen: 1})) != 1 {
		t.Fatalf("IdentitySelectorPathIDs did not return one id")
	}
	if len(schema.IdentityFieldPathIDs(IdentityConstraint{FieldOff: 0, FieldLen: 1})) != 1 {
		t.Fatalf("IdentityFieldPathIDs did not return one id")
	}
}

func TestSchemaAccessorsOutOfBounds(t *testing.T) {
	var nilSchema *Schema
	if nilSchema.TypeCount() != 0 || nilSchema.BuildHashValue() != 0 {
		t.Fatalf("nil schema returned non-zero count/hash")
	}
	if _, ok := nilSchema.Type(1); ok {
		t.Fatalf("nil schema Type returned ok")
	}
	if nilSchema.NamespaceBytes(1) != nil || nilSchema.SymbolLocalBytes(1) != nil {
		t.Fatalf("nil schema bytes accessors returned data")
	}
	if _, _, ok := nilSchema.SymbolBytes(1); ok {
		t.Fatalf("nil schema SymbolBytes returned ok")
	}

	schema := &Schema{}
	if _, ok := schema.Type(0); ok {
		t.Fatalf("Type(0) returned ok")
	}
	if _, ok := schema.Element(1); ok {
		t.Fatalf("Element out of bounds returned ok")
	}
	if _, ok := schema.Attribute(1); ok {
		t.Fatalf("Attribute out of bounds returned ok")
	}
	if _, ok := schema.ComplexType(1); ok {
		t.Fatalf("ComplexType out of bounds returned ok")
	}
	if _, ok := schema.ValidatorMeta(1); ok {
		t.Fatalf("ValidatorMeta out of bounds returned ok")
	}
	if _, ok := schema.DFAModelByRef(ModelRef{Kind: ModelDFA, ID: 1}); ok {
		t.Fatalf("DFAModelByRef out of bounds returned ok")
	}
	if _, ok := schema.NFAModelByRef(ModelRef{Kind: ModelDFA, ID: 1}); ok {
		t.Fatalf("NFAModelByRef wrong kind returned ok")
	}
	if _, ok := schema.AllModelByRef(ModelRef{Kind: ModelAll, ID: 0}); ok {
		t.Fatalf("AllModelByRef zero ID returned ok")
	}
	if _, ok := schema.GlobalType(1); ok {
		t.Fatalf("GlobalType out of bounds returned ok")
	}
	if _, ok := schema.SymbolNamespace(1); ok {
		t.Fatalf("SymbolNamespace out of bounds returned ok")
	}
	if _, ok := schema.AttributeHashTable(0); ok {
		t.Fatalf("AttributeHashTable out of bounds returned ok")
	}
	if schema.AttributeUses(AttrIndexRef{Off: 0, Len: 1}) != nil {
		t.Fatalf("AttributeUses out of bounds returned data")
	}
	if schema.AncestorIDs(0, 1) != nil || schema.AncestorMasks(0, 1) != nil {
		t.Fatalf("ancestor out-of-bounds spans returned data")
	}
	if schema.FacetProgram(FacetProgramRef{Off: 0, Len: 1}) != nil {
		t.Fatalf("FacetProgram out of bounds returned data")
	}
	if schema.Value(ValueRef{Off: 0, Len: 1, Present: true}) != nil {
		t.Fatalf("Value out of bounds returned data")
	}
	if schema.AllSubstitutions(0, 1) != nil {
		t.Fatalf("AllSubstitutions out of bounds returned data")
	}
	if schema.WildcardNamespaceSpan(NSConstraint{Off: 0, Len: 1}) != nil {
		t.Fatalf("WildcardNamespaceSpan out of bounds returned data")
	}
	if schema.ElementIdentityConstraintIDs(Element{ICOff: 0, ICLen: 1}) != nil {
		t.Fatalf("ElementIdentityConstraintIDs out of bounds returned data")
	}
	if schema.IdentitySelectorPathIDs(IdentityConstraint{SelectorOff: 0, SelectorLen: 1}) != nil {
		t.Fatalf("IdentitySelectorPathIDs out of bounds returned data")
	}
	if schema.IdentityFieldPathIDs(IdentityConstraint{FieldOff: 0, FieldLen: 1}) != nil {
		t.Fatalf("IdentityFieldPathIDs out of bounds returned data")
	}
}

func buildAccessorSchema(t *testing.T) (*Schema, SymbolID) {
	t.Helper()

	builder := NewBuilder()
	ns := mustInternNamespace(t, builder, []byte("urn:test"))
	sym := mustInternSymbol(t, builder, ns, []byte("root"))
	schema, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	schema.globalTypes = make([]TypeID, int(sym)+1)
	schema.globalElements = make([]ElemID, int(sym)+1)
	schema.globalAttributes = make([]AttrID, int(sym)+1)
	schema.globalTypes[sym] = 1
	schema.globalElements[sym] = 1
	schema.globalAttributes[sym] = 1
	schema.types = []Type{{}, {Name: sym, Kind: TypeSimple, Validator: 1}}
	schema.ancestors = TypeAncestors{
		IDs:   []TypeID{1},
		Masks: []DerivationMethod{DerRestriction},
	}
	schema.complexTypes = []ComplexType{{}, {Content: ContentElementOnly}}
	schema.elements = []Element{{}, {Name: sym, Type: 1, ICOff: 0, ICLen: 1}}
	schema.attributes = []Attribute{{}, {Name: sym, Validator: 1}}
	schema.attrIndex = ComplexAttrIndex{
		Uses:       []AttrUse{{Name: sym, Validator: 1}},
		HashTables: []AttrHashTable{{Hash: []uint64{1}, Slot: []uint32{1}}},
	}
	schema.validators = ValidatorsBundle{Meta: []ValidatorMeta{{}, {Kind: VString}}}
	schema.facets = []FacetInstr{{Op: FLength, Arg0: 1}}
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
	schema.rootPolicy = RootAny
	schema.buildHash = 0x1234
	return schema, sym
}
