package runtime

// Schema is the compiled, immutable form of a schema. Every ID stored
// in a published Schema is index-valid: freezeCompilerRuntime runs
// validateRuntimeSchema before the schema is handed to an Engine, so code
// reading freeze-validated IDs may index the component slices directly
// (typeName, the session hot paths). The checked accessors (simpleType,
// complexType, usableSimpleType, element) are for IDs that are not
// freeze-trusted: values still being compiled, or IDs derived from instance
// input. Instance-derived element IDs in particular use noElement for "no
// declaration" and resolve through rt.element; raw rt.Elements indexing is
// reserved for freeze-trusted IDs and is greppable as such.
//
// freezeCompilerRuntime validates and moves Schema out of the compiler.
// After freeze, the compiler runtime and name interner are cleared. Validation
// read projections are published once before first session use, after which
// sessions only read through invariant-bearing maps and slices. Concurrent
// sessions on one Engine are safe because that publication is synchronized by
// the Engine; TestEngineConcurrentValidation under the race detector is its
// executable form.
type Schema struct {
	GlobalAttributes               map[QName]AttributeID
	GlobalAttributeReads           map[QName]AttributeID
	GlobalElements                 map[QName]ElementID
	GlobalElementReads             map[QName]ElementID
	Substitutions                  map[ElementID][]ElementID
	SubstitutionReads              map[ElementID][]ElementID
	SubstitutionLookup             map[ElementID]map[QName]ElementID
	SubstitutionLookupReads        map[ElementID]map[QName]ElementID
	Notations                      map[QName]bool
	NotationReads                  map[ExpandedName]bool
	GlobalIdentities               map[QName]IdentityConstraintID
	GlobalTypes                    map[QName]TypeID
	GlobalTypeReads                map[QName]TypeID
	NameReads                      NameReadView
	Identities                     []IdentityConstraint
	IdentityConstraintReads        []IdentityConstraintRead
	TypeDerivations                TypeDerivationRead
	SimpleTypePrimitives           []PrimitiveKind
	SimpleTypeIdentities           []SimpleIdentityKind
	SimpleTypeFinals               []DerivationMask
	SimpleValueTypeReads           []SimpleValueTypeRead
	SimpleValueFacetReads          SimpleValueFacetReadTable
	SimpleValueReads               []SimpleValueRead
	SimpleValueQNameResolverNeeds  []bool
	simpleValueCallbacks           SimpleValueCallbacks
	rawSimpleValueCallbacks        RawSimpleValueCallbacks
	ComplexTypes                   []ComplexType
	ComplexTypeInfos               []TypeInfo
	ComplexAttributeUseSetIDs      []AttributeUseSetID
	ComplexContentModelIDs         []ContentModelID
	ComplexSimpleContentReads      []SimpleContentTypeRead
	ComplexChildContentReads       []ElementChildContent
	ComplexTextContentReads        []ElementTextContent
	FixedComplexTextContentReads   []ElementTextContent
	Wildcards                      []Wildcard
	WildcardReads                  []WildcardView
	AttributeUseSets               []AttributeUseSet
	AttributeUseSetReads           []AttributeUseSetRead
	Models                         []ContentModel
	CompiledModels                 []CompiledModel
	CompiledModelViews             []CompiledModelView
	SimpleTypes                    []SimpleType
	Attributes                     []AttributeDecl
	AttributeDeclReads             []AttributeDeclRead
	Elements                       []ElementDecl
	ElementNames                   []QName
	ElementStartInfos              []ElementStartInfo
	ElementIdentityConstraintReads [][]IdentityConstraintID
	ElementValueConstraintReads    []ElementValueConstraints
	Names                          NameTable
	Builtin                        BuiltinIDs
	SimpleTextContentRead          ElementTextContent
	readProjectionsPublished       bool
	validationHotPathsPrepared     bool
	//nolint:modernize // Schema is copied during publication; atomic.Uint32 would make it non-copyable.
	prepareState uint32
}

// SimpleType returns a simple type by runtime ID.
func (rt *Schema) SimpleType(id SimpleTypeID) (*SimpleType, bool) {
	return SimpleTypeByID(rt.SimpleTypes, id)
}

// UsableSimpleType returns a non-sentinel simple type by runtime ID.
func (rt *Schema) UsableSimpleType(id SimpleTypeID) (*SimpleType, bool) {
	return UsableSimpleType(rt.SimpleTypes, id)
}

// ComplexType returns a complex type by runtime ID.
func (rt *Schema) ComplexType(id ComplexTypeID) (*ComplexType, bool) {
	return ComplexTypeByID(rt.ComplexTypes, id)
}

// ElementDecl resolves an element ID that is not freeze-trusted: instance-derived
// match results use noElement for "no declaration", and the bounds check
// subsumes that sentinel. Freeze-trusted IDs may index rt.Elements directly.
func (rt *Schema) ElementDecl(id ElementID) (*ElementDecl, bool) {
	return ElementDeclByID(rt.Elements, id)
}

// TypeName returns the QName for a runtime type ID.
func (rt *Schema) TypeName(t TypeID) QName {
	name, ok := TypeNameByID(rt.SimpleTypes, rt.ComplexTypes, t)
	if !ok {
		panic("invalid runtime type ID")
	}
	return name
}

// AnyTypeID returns the runtime xs:anyType complex-type ID for derivation traversal.
func (rt *Schema) AnyTypeID() ComplexTypeID {
	return RuntimeAnyTypeID(rt.TypeDerivations, rt.Builtin)
}

// ComplexTypeCount returns the number of runtime complex types.
func (rt *Schema) ComplexTypeCount() int {
	return RuntimeComplexTypeCount(rt.TypeDerivations, rt.ComplexTypes)
}

// SimpleTypeFinal returns final constraints for a runtime simple type.
func (rt *Schema) SimpleTypeFinal(id SimpleTypeID) (DerivationMask, bool) {
	if !rt.ReadProjectionsPublished() {
		st, ok := rt.UsableSimpleType(id)
		if !ok {
			return 0, false
		}
		return st.Final, true
	}
	return SimpleTypeFinalByID(rt.SimpleTypeFinals, id)
}

// SimpleTypeDerivation returns graph metadata for simple-type derivation traversal.
func (rt *Schema) SimpleTypeDerivation(id SimpleTypeID) (SimpleTypeDerivation, bool) {
	return RuntimeSimpleTypeDerivation(rt.TypeDerivations, rt.SimpleTypes, id)
}

// ComplexTypeDerivation returns graph metadata for complex-type derivation traversal.
func (rt *Schema) ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool) {
	return RuntimeComplexTypeDerivation(rt.TypeDerivations, rt.ComplexTypes, id)
}

// ContentModel returns a cloned content model by runtime ID.
func (rt *Schema) ContentModel(id ContentModelID) (ContentModel, bool) {
	return ContentModelByID(rt.Models, id)
}

// ElementName returns the QName for an element declaration.
func (rt *Schema) ElementName(id ElementID) (QName, bool) {
	if !rt.ReadProjectionsPublished() {
		decl, ok := rt.ElementDecl(id)
		if !ok {
			return QName{}, false
		}
		return decl.Name, true
	}
	return ElementNameByID(rt.ElementNames, id)
}

// ElementType returns the declared type for an element declaration.
func (rt *Schema) ElementType(id ElementID) (TypeID, bool) {
	return ElementTypeByID(rt.Elements, id)
}

// ElementRestriction returns the element projection needed by particle restriction checks.
func (rt *Schema) ElementRestriction(id ElementID) (ParticleRestrictionElement, bool) {
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
	return WildcardByID(rt.Wildcards, id)
}

// ForEachSubstitutionMember iterates substitution members for an element.
func (rt *Schema) ForEachSubstitutionMember(id ElementID, fn func(ElementID) bool) {
	ForEachSubstitutionMember(rt.SubstitutionReads, id, fn)
}

// SubstitutionMemberByName returns a substitution member with the given QName.
func (rt *Schema) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	if !rt.ReadProjectionsPublished() {
		return SubstitutionMemberByName(rt.SubstitutionLookup, id, name)
	}
	return SubstitutionMemberByName(rt.SubstitutionLookupReads, id, name)
}

// SubstitutionMembersByName returns the QName lookup for substitution members.
func (rt *Schema) SubstitutionMembersByName(id ElementID) map[QName]ElementID {
	if !rt.ReadProjectionsPublished() {
		return rt.SubstitutionLookup[id]
	}
	return rt.SubstitutionLookupReads[id]
}

// RuntimeGlobals returns global declaration projections for runtime validation.
func (rt *Schema) RuntimeGlobals() RuntimeGlobals {
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

// TypeLabel formats a runtime type name for diagnostics.
func (rt *Schema) TypeLabel(t TypeID) string {
	q := rt.TypeName(t)
	return rt.Names.Format(q)
}
