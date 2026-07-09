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
type schemaReads struct {
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
	SimpleValueTypes              []SimpleValueTypeRead
	SimpleValueFacets             SimpleValueFacetReadTable
	SimpleValueQNameResolverNeeds []bool
	simpleValueCallbacks          SimpleValueCallbacks
	ComplexTypeInfos              []TypeInfo
	ComplexAttributeUseSetIDs     []AttributeUseSetID
	ComplexContentModelIDs        []ContentModelID
	ComplexSimpleContent          []SimpleContentTypeRead
	ComplexChildContent           []ElementChildContent
	ComplexTextContent            []ElementTextContent
	FixedComplexTextContent       []ElementTextContent
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

// Schema is sealed validation-ready schema state. Its source tables and read
// projections are private so validation cannot mutate compiler-owned data.
//
//nolint:govet // Keeping owned build data and projections as values avoids publication allocations.
type Schema struct {
	build SchemaBuild
	reads schemaReads
}

// Builtins returns the immutable built-in declaration handles.
func (rt *Schema) Builtins() BuiltinIDs {
	return rt.build.Builtin
}

// LocalNameCount returns the number of interned local names.
func (rt *Schema) LocalNameCount() int {
	return rt.build.Names.LocalCount()
}

// NamespaceCount returns the number of interned namespace URIs.
func (rt *Schema) NamespaceCount() int {
	return rt.build.Names.NamespaceCount()
}

// WildcardCount returns the number of compiled wildcards.
func (rt *Schema) WildcardCount() int {
	return len(rt.build.Wildcards)
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

// TypeName returns the QName for a runtime type ID.
func (rt *Schema) TypeName(t TypeID) QName {
	name, ok := TypeNameByID(rt.build.SimpleTypes, rt.build.ComplexTypes, t)
	if !ok {
		panic("invalid runtime type ID")
	}
	return name
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
	return RuntimeAnyTypeID(rt.reads.TypeDerivations, rt.build.Builtin)
}

// AnyTypeID returns the compiler-owned xs:anyType ID.
func (rt *SchemaBuild) AnyTypeID() ComplexTypeID {
	return rt.Builtin.AnyType
}

// ComplexTypeCount returns the number of runtime complex types.
func (rt *Schema) ComplexTypeCount() int {
	return RuntimeComplexTypeCount(rt.reads.TypeDerivations, rt.build.ComplexTypes)
}

// ComplexTypeCount returns the number of compiler-owned complex types.
func (rt *SchemaBuild) ComplexTypeCount() int {
	return len(rt.ComplexTypes)
}

// SimpleTypeFinal returns final constraints for a runtime simple type.
func (rt *Schema) SimpleTypeFinal(id SimpleTypeID) (DerivationMask, bool) {
	return SimpleTypeFinalByID(rt.reads.SimpleTypeFinals, id)
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
	return RuntimeSimpleTypeDerivation(rt.reads.TypeDerivations, rt.build.SimpleTypes, id)
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
	return RuntimeComplexTypeDerivation(rt.reads.TypeDerivations, rt.build.ComplexTypes, id)
}

// ComplexTypeDerivation returns compiler-owned complex-type derivation metadata.
func (rt *SchemaBuild) ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool) {
	ct, ok := rt.ComplexType(id)
	if !ok {
		return ComplexTypeDerivation{}, false
	}
	return NewComplexTypeDerivationForComplexType(*ct), true
}

// ContentModel returns a cloned content model by runtime ID.
func (rt *Schema) ContentModel(id ContentModelID) (ContentModel, bool) {
	return ContentModelByID(rt.build.Models, id)
}

// CompiledModel returns a detached compiled content model for inspection.
func (rt *Schema) CompiledModel(id ContentModelID) (CompiledModel, bool) {
	if !ValidContentModelID(id, len(rt.build.CompiledModels)) {
		return CompiledModel{}, false
	}
	return CloneCompiledModel(rt.build.CompiledModels[id]), true
}

// ContentModel returns a compiler-owned content model by ID.
func (rt *SchemaBuild) ContentModel(id ContentModelID) (ContentModel, bool) {
	return ContentModelByID(rt.Models, id)
}

// ElementName returns the QName for an element declaration.
func (rt *Schema) ElementName(id ElementID) (QName, bool) {
	return ElementNameByID(rt.reads.ElementNames, id)
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
	return ElementTypeByID(rt.build.Elements, id)
}

// ElementType returns a compiler-owned element type by ID.
func (rt *SchemaBuild) ElementType(id ElementID) (TypeID, bool) {
	return ElementTypeByID(rt.Elements, id)
}

// ElementRestriction returns the element projection needed by particle restriction checks.
func (rt *Schema) ElementRestriction(id ElementID) (ParticleRestrictionElement, bool) {
	if !ValidElementID(id, len(rt.build.Elements)) {
		return ParticleRestrictionElement{}, false
	}
	decl := rt.build.Elements[id]
	return ParticleRestrictionElement{
		Type:     decl.Type,
		Block:    decl.Block,
		Fixed:    NewValueConstraintIdentity(decl.Fixed),
		Nillable: decl.Nillable,
	}, true
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

// Wildcard returns a cloned wildcard by runtime ID.
func (rt *Schema) Wildcard(id WildcardID) (Wildcard, bool) {
	return WildcardByID(rt.build.Wildcards, id)
}

// Wildcard returns a compiler-owned wildcard by ID.
func (rt *SchemaBuild) Wildcard(id WildcardID) (Wildcard, bool) {
	return WildcardByID(rt.Wildcards, id)
}

// ForEachSubstitutionMember iterates substitution members for an element.
func (rt *Schema) ForEachSubstitutionMember(id ElementID, fn func(ElementID) bool) {
	ForEachSubstitutionMember(rt.reads.Substitutions, id, fn)
}

// ForEachSubstitutionMember iterates compiler-owned substitution members.
func (rt *SchemaBuild) ForEachSubstitutionMember(id ElementID, fn func(ElementID) bool) {
	ForEachSubstitutionMember(rt.Substitutions, id, fn)
}

// SubstitutionMemberByName returns a substitution member with the given QName.
func (rt *Schema) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	return SubstitutionMemberByName(rt.reads.SubstitutionLookup, id, name)
}

// SubstitutionMemberByName returns a compiler-owned substitution member by name.
func (rt *SchemaBuild) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	return SubstitutionMemberByName(rt.SubstitutionLookup, id, name)
}

// SubstitutionMembersByName returns compiler-owned substitution lookups.
func (rt *SchemaBuild) SubstitutionMembersByName(id ElementID) map[QName]ElementID {
	return rt.SubstitutionLookup[id]
}

// RuntimeGlobals returns global declaration projections for runtime validation.
func (rt *Schema) RuntimeGlobals() RuntimeGlobals {
	return NewRuntimeGlobals(RuntimeGlobalInput{
		GlobalAttributes: rt.build.GlobalAttributes,
		GlobalElements:   rt.build.GlobalElements,
		GlobalTypes:      rt.build.GlobalTypes,
		GlobalIdentities: rt.build.GlobalIdentities,
		Notations:        rt.build.Notations,
		Attributes:       rt.build.Attributes,
		Elements:         rt.build.Elements,
		SimpleTypes:      rt.build.SimpleTypes,
		ComplexTypes:     rt.build.ComplexTypes,
		Identities:       rt.build.Identities,
	})
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

// TypeLabel formats a runtime type name for diagnostics.
func (rt *Schema) TypeLabel(t TypeID) string {
	q := rt.TypeName(t)
	return rt.build.Names.Format(q)
}

// TypeLabel formats a compiler-owned type name for diagnostics.
func (rt *SchemaBuild) TypeLabel(t TypeID) string {
	return rt.Names.Format(rt.TypeName(t))
}

// SimpleTypeFastPath returns the published fast-path classification for id.
func (rt *Schema) SimpleTypeFastPath(id SimpleTypeID) (SimpleFastKind, bool) {
	if !ValidSimpleTypeID(id, len(rt.build.SimpleTypes)) {
		return SimpleFastNone, false
	}
	return rt.build.SimpleTypes[id].Fast, true
}
