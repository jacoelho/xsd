package runtime

// SchemaBuild is compiler-owned mutable schema state. PublishSchema is the
// only supported transition to an immutable validation Schema.
type SchemaBuild struct {
	GlobalAttributes   map[QName]AttributeID
	GlobalElements     map[QName]ElementID
	Substitutions      map[ElementID][]ElementID
	SubstitutionLookup map[ElementID]map[QName]ElementID
	Notations          map[QName]bool
	GlobalIdentities   map[QName]IdentityConstraintID
	GlobalTypes        map[QName]TypeID
	Identities         []IdentityConstraint
	ComplexTypes       []ComplexType
	Wildcards          []Wildcard
	AttributeUseSets   []AttributeUseSet
	Models             []ContentModel
	CompiledModels     []CompiledModel
	SimpleTypes        []SimpleType
	Attributes         []AttributeDecl
	Elements           []ElementDecl
	Names              NameTable
	Builtin            BuiltinIDs
}

//nolint:govet // Field order groups projections by owning validation subsystem.
type schemaRuntime struct {
	GlobalAttributes        map[QName]AttributeID
	GlobalElements          map[QName]ElementID
	GlobalTypes             map[QName]TypeID
	SubstitutionLookup      map[ElementID]map[QName]ElementID
	Notations               map[ExpandedName]bool
	Names                   NameReadView
	Identities              []IdentityConstraintRead
	TypeDerivations         TypeDerivationRead
	SimpleValueRoutes       []simpleValueRouteRead
	SimpleValueCold         simpleValueColdReadTable
	SimpleValueQNameNeeds   []bool
	ComplexTypes            []complexTypeRead
	Wildcards               []WildcardView
	AttributeUseSets        []AttributeUseSetRead
	CompiledModels          []compiledModelRead
	Attributes              []AttributeDeclRead
	ElementNames            []QName
	ElementStarts           []ElementStartInfo
	ElementIdentities       [][]IdentityConstraintID
	ElementValueConstraints []ElementValueConstraints
}

type complexTypeRead struct {
	attributeUseSet AttributeUseSetID
	contentModel    ContentModelID
	textType        SimpleTypeID
	block           DerivationMask
	flags           complexTypeReadFlags
}

type complexTypeReadFlags uint8

const (
	complexTypeReadSimple complexTypeReadFlags = 1 << iota
	complexTypeReadMixed
	complexTypeReadAbstract
)

func newComplexTypeReads(types []ComplexType) []complexTypeRead {
	reads := make([]complexTypeRead, len(types))
	for i := range types {
		reads[i] = newComplexTypeRead(types[i])
	}
	return reads
}

func newComplexTypeRead(ct ComplexType) complexTypeRead {
	var flags complexTypeReadFlags
	if ct.SimpleContent() {
		flags |= complexTypeReadSimple
	}
	if ct.Mixed() {
		flags |= complexTypeReadMixed
	}
	if ct.Abstract {
		flags |= complexTypeReadAbstract
	}
	return complexTypeRead{
		attributeUseSet: ct.Attrs,
		contentModel:    ct.Content,
		textType:        ct.TextType,
		block:           ct.Block,
		flags:           flags,
	}
}

func (r complexTypeRead) typeInfo() TypeInfo {
	return NewTypeInfo(TypeInfoShape{
		Block:    r.block,
		Abstract: r.flags&complexTypeReadAbstract != 0,
	})
}

func (r complexTypeRead) simpleContent() SimpleContentTypeRead {
	return NewSimpleContentTypeRead(SimpleContentTypeReadShape{
		Type:    r.textType,
		Present: r.flags&complexTypeReadSimple != 0,
	})
}

func (r complexTypeRead) childContent() ElementChildContent {
	return NewElementChildContent(ElementChildContentShape{
		Complex: true,
		Simple:  r.flags&complexTypeReadSimple != 0,
	})
}

func (r complexTypeRead) textContent(fixed bool) ElementTextContent {
	return NewElementTextContent(ElementTextContentShape{
		Simple:  r.flags&complexTypeReadSimple != 0,
		Complex: true,
		Mixed:   r.flags&complexTypeReadMixed != 0,
		Fixed:   fixed,
	})
}

// Schema is sealed validation-ready schema state.
type Schema struct {
	runtime schemaRuntime
}

// SimpleType returns compiler-owned simple-type state by ID.
func (rt *SchemaBuild) SimpleType(id SimpleTypeID) (*SimpleType, bool) {
	return SimpleTypeByID(rt.SimpleTypes, id)
}

// UsableSimpleType returns non-sentinel compiler-owned simple-type state by ID.
func (rt *SchemaBuild) UsableSimpleType(id SimpleTypeID) (*SimpleType, bool) {
	return UsableSimpleType(rt.SimpleTypes, id)
}

// ComplexType returns compiler-owned complex-type state by ID.
func (rt *SchemaBuild) ComplexType(id ComplexTypeID) (*ComplexType, bool) {
	return ComplexTypeByID(rt.ComplexTypes, id)
}

// ElementDecl returns a compiler-owned element declaration by ID.
func (rt *SchemaBuild) ElementDecl(id ElementID) (*ElementDecl, bool) {
	return ElementDeclByID(rt.Elements, id)
}

// TypeName returns a compiler-owned type name.
func (rt *SchemaBuild) TypeName(t TypeID) QName {
	name, ok := TypeNameByID(rt.SimpleTypes, rt.ComplexTypes, t)
	if !ok {
		panic("invalid runtime type ID")
	}
	return name
}

// AnyTypeID returns the compiler-owned xs:anyType ID.
func (rt *SchemaBuild) AnyTypeID() ComplexTypeID {
	return rt.Builtin.AnyType
}

// ComplexTypeCount returns the number of compiler-owned complex types.
func (rt *SchemaBuild) ComplexTypeCount() int {
	return len(rt.ComplexTypes)
}

// SimpleTypeCount returns the number of compiler-owned simple types.
func (rt *SchemaBuild) SimpleTypeCount() int {
	return len(rt.SimpleTypes)
}

// SimpleTypeFinal returns compiler-owned simple-type final constraints.
func (rt *SchemaBuild) SimpleTypeFinal(id SimpleTypeID) (DerivationMask, bool) {
	st, ok := rt.UsableSimpleType(id)
	if !ok {
		return 0, false
	}
	return st.Final, true
}

// SimpleTypeDerivation returns compiler-owned simple-type derivation metadata.
func (rt *SchemaBuild) SimpleTypeDerivation(id SimpleTypeID) (SimpleTypeDerivation, bool) {
	st, ok := rt.UsableSimpleType(id)
	if !ok {
		return SimpleTypeDerivation{}, false
	}
	return NewSimpleTypeDerivationForSimpleType(*st), true
}

// ComplexTypeDerivation returns compiler-owned complex-type derivation metadata.
func (rt *SchemaBuild) ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool) {
	ct, ok := rt.ComplexType(id)
	if !ok {
		return ComplexTypeDerivation{}, false
	}
	return NewComplexTypeDerivationForComplexType(*ct), true
}

// ContentModel returns a compiler-owned content model by ID.
func (rt *SchemaBuild) ContentModel(id ContentModelID) (ContentModel, bool) {
	return ContentModelByID(rt.Models, id)
}

// ElementName returns a compiler-owned element name by ID.
func (rt *SchemaBuild) ElementName(id ElementID) (QName, bool) {
	decl, ok := rt.ElementDecl(id)
	if !ok {
		return QName{}, false
	}
	return decl.Name, true
}

// ElementType returns a compiler-owned element type by ID.
func (rt *SchemaBuild) ElementType(id ElementID) (TypeID, bool) {
	return ElementTypeByID(rt.Elements, id)
}

// ElementRestriction returns compiler-owned particle-restriction metadata.
func (rt *SchemaBuild) ElementRestriction(id ElementID) (ParticleRestrictionElement, bool) {
	if !ValidElementID(id, len(rt.Elements)) {
		return ParticleRestrictionElement{}, false
	}
	decl := rt.Elements[id]
	return ParticleRestrictionElement{
		Type:     decl.Type,
		Block:    decl.Block,
		Fixed:    NewValueConstraintIdentity(decl.Fixed),
		Nillable: decl.Nillable,
	}, true
}

// Wildcard returns a compiler-owned wildcard by ID.
func (rt *SchemaBuild) Wildcard(id WildcardID) (Wildcard, bool) {
	return WildcardByID(rt.Wildcards, id)
}

// ForEachSubstitutionMember iterates compiler-owned substitution members.
func (rt *SchemaBuild) ForEachSubstitutionMember(id ElementID, fn func(ElementID) bool) {
	ForEachSubstitutionMember(rt.Substitutions, id, fn)
}

// SubstitutionMemberByName returns a compiler-owned substitution member by name.
func (rt *SchemaBuild) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	return SubstitutionMemberByName(rt.SubstitutionLookup, id, name)
}

// SubstitutionMembersByName returns compiler-owned substitution lookups.
func (rt *SchemaBuild) SubstitutionMembersByName(id ElementID) map[QName]ElementID {
	return rt.SubstitutionLookup[id]
}

// TypeLabel formats a compiler-owned type name for diagnostics.
func (rt *SchemaBuild) TypeLabel(t TypeID) string {
	return rt.Names.Format(rt.TypeName(t))
}
