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
	Builtin                       BuiltinIDs
	GlobalAttributes              map[QName]AttributeID
	GlobalElements                map[QName]ElementID
	GlobalTypes                   map[QName]TypeID
	Substitutions                 map[ElementID][]ElementID
	SubstitutionLookup            map[ElementID]map[QName]ElementID
	Notations                     map[ExpandedName]bool
	Names                         NameReadView
	Identities                    []IdentityConstraintRead
	TypeDerivations               TypeDerivationRead
	SimpleTypePrimitives          []PrimitiveKind
	SimpleTypeIdentities          []SimpleIdentityKind
	SimpleTypeFinals              []DerivationMask
	SimpleValueRoutes             []simpleValueRouteRead
	SimpleValueCold               simpleValueColdReadTable
	SimpleValueQNameResolverNeeds []bool
	simpleValueCallbacks          SimpleValueCallbacks
	rawSimpleValueCallbacks       RawSimpleValueCallbacks
	ComplexTypes                  []complexTypeRead
	Wildcards                     []WildcardView
	AttributeUseSets              []AttributeUseSetRead
	CompiledModels                []CompiledModelView
	Attributes                    []AttributeDeclRead
	ElementNames                  []QName
	ElementStarts                 []ElementStartInfo
	ElementIdentities             [][]IdentityConstraintID
	ElementValueConstraints       []ElementValueConstraints
	SimpleTextContent             ElementTextContent
}

type complexTypeRead struct {
	simpleContent   SimpleContentTypeRead
	attributeUseSet AttributeUseSetID
	contentModel    ContentModelID
	textContent     ElementTextContent
	fixedText       ElementTextContent
	info            TypeInfo
	childContent    ElementChildContent
}

func newComplexTypeReads(types []ComplexType) []complexTypeRead {
	reads := make([]complexTypeRead, len(types))
	for i := range types {
		reads[i] = complexTypeRead{
			info:            NewTypeInfoForComplexType(types[i]),
			attributeUseSet: types[i].Attrs,
			contentModel:    types[i].Content,
			simpleContent:   NewSimpleContentTypeReadForComplexType(types[i]),
			childContent:    NewElementChildContentForComplexType(types[i]),
			textContent:     NewElementTextContentForComplexType(types[i], false),
			fixedText:       NewElementTextContentForComplexType(types[i], true),
		}
	}
	return reads
}

// Schema is sealed validation-ready schema state.
type Schema struct {
	runtime schemaRuntime
}

// Builtins returns the immutable built-in declaration handles.
func (rt *Schema) Builtins() BuiltinIDs {
	return rt.runtime.Builtin
}

// LocalNameCount returns the number of interned local names.
func (rt *Schema) LocalNameCount() int {
	return rt.runtime.Names.LocalCount()
}

// NamespaceCount returns the number of interned namespace URIs.
func (rt *Schema) NamespaceCount() int {
	return rt.runtime.Names.NamespaceCount()
}

// WildcardCount returns the number of compiled wildcards.
func (rt *Schema) WildcardCount() int {
	return len(rt.runtime.Wildcards)
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

// AnyTypeID returns the runtime xs:anyType complex-type ID for derivation traversal.
func (rt *Schema) AnyTypeID() ComplexTypeID {
	return rt.runtime.TypeDerivations.AnyTypeID()
}

// AnyTypeID returns the compiler-owned xs:anyType ID.
func (rt *SchemaBuild) AnyTypeID() ComplexTypeID {
	return rt.Builtin.AnyType
}

// ComplexTypeCount returns the number of runtime complex types.
func (rt *Schema) ComplexTypeCount() int {
	return rt.runtime.TypeDerivations.ComplexTypeCount()
}

// ComplexTypeCount returns the number of compiler-owned complex types.
func (rt *SchemaBuild) ComplexTypeCount() int {
	return len(rt.ComplexTypes)
}

// SimpleTypeFinal returns final constraints for a runtime simple type.
func (rt *Schema) SimpleTypeFinal(id SimpleTypeID) (DerivationMask, bool) {
	return SimpleTypeFinalByID(rt.runtime.SimpleTypeFinals, id)
}

// SimpleTypeFinal returns compiler-owned simple-type final constraints.
func (rt *SchemaBuild) SimpleTypeFinal(id SimpleTypeID) (DerivationMask, bool) {
	st, ok := rt.UsableSimpleType(id)
	if !ok {
		return 0, false
	}
	return st.Final, true
}

// SimpleTypeDerivation returns graph metadata for simple-type derivation traversal.
func (rt *Schema) SimpleTypeDerivation(id SimpleTypeID) (SimpleTypeDerivation, bool) {
	return rt.runtime.TypeDerivations.SimpleTypeDerivation(id)
}

// SimpleTypeDerivation returns compiler-owned simple-type derivation metadata.
func (rt *SchemaBuild) SimpleTypeDerivation(id SimpleTypeID) (SimpleTypeDerivation, bool) {
	st, ok := rt.UsableSimpleType(id)
	if !ok {
		return SimpleTypeDerivation{}, false
	}
	return NewSimpleTypeDerivationForSimpleType(*st), true
}

// ComplexTypeDerivation returns graph metadata for complex-type derivation traversal.
func (rt *Schema) ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool) {
	return rt.runtime.TypeDerivations.ComplexTypeDerivation(id)
}

// ComplexTypeDerivation returns compiler-owned complex-type derivation metadata.
func (rt *SchemaBuild) ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool) {
	ct, ok := rt.ComplexType(id)
	if !ok {
		return ComplexTypeDerivation{}, false
	}
	return NewComplexTypeDerivationForComplexType(*ct), true
}

// CompiledModel returns a detached compiled content model for inspection.
func (rt *Schema) CompiledModel(id ContentModelID) (CompiledModel, bool) {
	view, ok := CompiledModelViewByID(rt.runtime.CompiledModels, id)
	if !ok {
		return CompiledModel{}, false
	}
	return CloneCompiledModel(*view.compiled()), true
}

// ContentModel returns a compiler-owned content model by ID.
func (rt *SchemaBuild) ContentModel(id ContentModelID) (ContentModel, bool) {
	return ContentModelByID(rt.Models, id)
}

// ElementName returns the QName for an element declaration.
func (rt *Schema) ElementName(id ElementID) (QName, bool) {
	return ElementNameByID(rt.runtime.ElementNames, id)
}

// ElementName returns a compiler-owned element name by ID.
func (rt *SchemaBuild) ElementName(id ElementID) (QName, bool) {
	decl, ok := rt.ElementDecl(id)
	if !ok {
		return QName{}, false
	}
	return decl.Name, true
}

// ElementType returns the declared type for an element declaration.
func (rt *Schema) ElementType(id ElementID) (TypeID, bool) {
	return DeclaredElementTypeByID(rt.runtime.ElementStarts, id)
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

// ForEachSubstitutionMember iterates substitution members for an element.
func (rt *Schema) ForEachSubstitutionMember(id ElementID, fn func(ElementID) bool) {
	ForEachSubstitutionMember(rt.runtime.Substitutions, id, fn)
}

// ForEachSubstitutionMember iterates compiler-owned substitution members.
func (rt *SchemaBuild) ForEachSubstitutionMember(id ElementID, fn func(ElementID) bool) {
	ForEachSubstitutionMember(rt.Substitutions, id, fn)
}

// SubstitutionMemberByName returns a substitution member with the given QName.
func (rt *Schema) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	return SubstitutionMemberByName(rt.runtime.SubstitutionLookup, id, name)
}

// SubstitutionMemberByName returns a compiler-owned substitution member by name.
func (rt *SchemaBuild) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	return SubstitutionMemberByName(rt.SubstitutionLookup, id, name)
}

// SubstitutionMembersByName returns compiler-owned substitution lookups.
func (rt *SchemaBuild) SubstitutionMembersByName(id ElementID) map[QName]ElementID {
	return rt.SubstitutionLookup[id]
}

// RuntimeGlobals returns detached compiler-owned global declaration metadata.
func (rt *SchemaBuild) RuntimeGlobals() RuntimeGlobals {
	return NewRuntimeGlobals(RuntimeGlobalInput{
		GlobalAttributes: rt.GlobalAttributes,
		GlobalElements:   rt.GlobalElements,
		GlobalTypes:      rt.GlobalTypes,
		GlobalIdentities: rt.GlobalIdentities,
		Notations:        rt.Notations,
		Attributes:       rt.Attributes,
		Elements:         rt.Elements,
		SimpleTypes:      rt.SimpleTypes,
		ComplexTypes:     rt.ComplexTypes,
		Identities:       rt.Identities,
	})
}

// LookupQName resolves a compiler-owned schema name.
func (rt *SchemaBuild) LookupQName(ns, local string) (QName, bool) {
	return rt.Names.LookupQName(ns, local)
}

// GlobalType returns a compiler-owned global type declaration.
func (rt *SchemaBuild) GlobalType(name QName) (TypeID, bool) {
	typ, ok := rt.GlobalTypes[name]
	return typ, ok
}

// TypeLabel formats a compiler-owned type name for diagnostics.
func (rt *SchemaBuild) TypeLabel(t TypeID) string {
	return rt.Names.Format(rt.TypeName(t))
}

// SimpleTypeFastPath returns the published fast-path classification for id.
func (rt *Schema) SimpleTypeFastPath(id SimpleTypeID) (SimpleFastKind, bool) {
	read, ok := simpleValueRouteReadByID(rt.runtime.SimpleValueRoutes, id)
	if !ok {
		return SimpleFastNone, false
	}
	return read.fast, true
}
