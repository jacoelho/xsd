package schema

import (
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

// SchemaBuild is compiler-owned mutable schema state. PublishSchema is the
// only supported transition to an immutable validation Schema.
type schemaBuild struct {
	GlobalAttributes map[QName]AttributeID
	GlobalElements   map[QName]ElementID
	Notations        map[QName]bool
	GlobalIdentities map[QName]IdentityConstraintID
	GlobalTypes      map[QName]TypeID
	// typeWork charges derivation traversals to the compiler's dependency
	// budget while this mutable build is being compiled or published. A nil
	// budget is reserved for isolated unit-test fixtures.
	typeWork *workBudget
	// valueBuilder owns canonical simple-type construction until publication.
	// It is private compiler state and never escapes as a mutable object.
	valueBuilder          *value.Builder
	Substitutions         substitutionTable
	Models                []contentModelNode
	AttributeUseSets      []attributeUseSet
	Wildcards             []wildcardRuntime
	CompiledContent       compiledContent
	SimpleTypes           []simpleType
	simpleTypeUnavailable []bool
	Attributes            []attributeDecl
	Elements              []elementDecl
	Identities            []identityConstraintSpec
	ComplexTypes          []complexTypeMetadata
	Names                 nameTable
	AnyType               ComplexTypeID
}

// schemaProgram is the immutable validation representation produced by one
// successful publication. It contains only data consumed after compilation;
// compiler records and provenance stay in SchemaBuild until sealing succeeds.
type schemaProgram struct {
	DiagnosticNames  diagnosticNames
	GlobalElements   map[QName]ElementID
	GlobalTypes      map[QName]TypeID
	Notations        map[ExpandedName]bool
	TypeDerivations  typeDerivationRead
	GlobalAttributes map[QName]AttributeID
	Value            *value.Program
	Elements         elementReadTable
	Substitutions    substitutionTable
	Names            nameReadView
	// SimpleTypeUnavailable preserves the compiler's optional-import recovery
	// fact. The value program owns all other simple-type metadata.
	SimpleTypeUnavailable []bool
	AttributeUseSets      []AttributeUseSetRead
	CompiledModels        []compiledModelRead
	Attributes            []AttributeDeclRead
	Wildcards             []WildcardView
	ComplexTypes          []complexTypeRead
	IdentityDispatch      IdentityDispatchRead
}

type complexTypeRead struct {
	attributeUseSet attributeUseSetID
	contentModel    contentModelID
	textType        SimpleTypeID
	block           DerivationMask
	contentKind     complexContentVariety
	flags           complexTypeReadFlags
}

type complexTypeReadFlags uint8

const (
	complexTypeReadAbstract complexTypeReadFlags = 1 << iota
	// complexTypeReadDetached marks a type validation never reaches
	// (complexTypeMetadata.Detached). It has no content model or attribute
	// uses, and publishedComplexType rejects it.
	complexTypeReadDetached
)

// newComplexTypeReads points each type at publishedModels[i], its model's
// index in the published content-model table, and returns the attribute-use
// sets to publish: only those a type that is not detached uses, each once.
// Attribute group sets and the sets only detached types use are not published.
func newComplexTypeReads(types []complexTypeMetadata, publishedModels []contentModelID, sets []attributeUseSet) ([]complexTypeRead, []attributeUseSet, error) {
	reads := make([]complexTypeRead, len(types))
	position := make([]attributeUseSetID, len(sets))
	for set := range position {
		position[set] = noAttributeUseSet
	}
	kept := 0
	for i := range types {
		if types[i].Detached {
			reads[i] = complexTypeRead{attributeUseSet: noAttributeUseSet, contentModel: noContentModel, textType: NoSimpleType, flags: complexTypeReadDetached}
			continue
		}
		set := types[i].Attrs
		if !validAttributeUseSetID(set, len(sets)) {
			return nil, nil, xsderrors.InternalInvariant("complex type references invalid attribute use set")
		}
		if position[set] == noAttributeUseSet {
			position[set] = attributeUseSetID(kept) // #nosec G115 -- at most one set per complex type, and complex-type IDs are uint32.
			kept++
		}
		reads[i] = newComplexTypeRead(types[i], publishedModels[i], position[set])
	}
	return reads, publishedAttributeUseSets(sets, position, kept), nil
}

// publishedAttributeUseSets returns kept sets; the kept positions are dense
// from 0 to kept-1.
func publishedAttributeUseSets(sets []attributeUseSet, position []attributeUseSetID, kept int) []attributeUseSet {
	published := make([]attributeUseSet, kept)
	for set, at := range position {
		if at != noAttributeUseSet {
			published[at] = sets[set]
		}
	}
	return published
}

func newComplexTypeRead(ct complexTypeMetadata, contentModel contentModelID, attributeUseSet attributeUseSetID) complexTypeRead {
	var flags complexTypeReadFlags
	if ct.Abstract {
		flags |= complexTypeReadAbstract
	}
	return complexTypeRead{
		attributeUseSet: attributeUseSet,
		contentModel:    contentModel,
		textType:        ct.TextType,
		block:           ct.Block,
		contentKind:     ct.ContentKind,
		flags:           flags,
	}
}

func (r complexTypeRead) typeInfo() TypeInfo {
	return TypeInfo{
		Block:    r.block,
		Abstract: r.flags&complexTypeReadAbstract != 0,
	}
}

func (r complexTypeRead) simpleContent() (SimpleTypeID, bool) {
	if !r.contentKind.Simple() {
		return NoSimpleType, false
	}
	return r.textType, true
}

func (r complexTypeRead) textContent(fixed, constrained bool) ElementTextContent {
	return ElementTextContent{
		kind:        r.contentKind,
		fixed:       fixed,
		constrained: constrained,
	}
}

// Schema is sealed validation-ready schema state.
type Schema struct {
	program schemaProgram
}

func (rt *schemaBuild) spendTypeDerivationWork(steps int) error {
	if rt.typeWork == nil {
		return nil
	}
	return rt.typeWork.spend(steps)
}

// SimpleTypeFinal returns compiler-owned simple-type final constraints.
func (rt *schemaBuild) SimpleTypeFinal(id SimpleTypeID) (DerivationMask, bool) {
	st, ok := usableSimpleType(rt.SimpleTypes, id)
	if !ok {
		return 0, false
	}
	return st.Final, true
}

// SimpleTypeDerivation returns compiler-owned simple-type derivation metadata.
func (rt *schemaBuild) SimpleTypeDerivation(id SimpleTypeID) (simpleTypeDerivation, bool) {
	st, ok := usableSimpleType(rt.SimpleTypes, id)
	if !ok {
		return simpleTypeDerivation{}, false
	}
	return newSimpleTypeDerivationForSimpleType(*st), true
}

// ComplexTypeDerivation returns compiler-owned complex-type derivation metadata.
func (rt *schemaBuild) ComplexTypeDerivation(id ComplexTypeID) (complexTypeDerivation, bool) {
	ct, ok := complexTypeByID(rt.ComplexTypes, id)
	if !ok {
		return complexTypeDerivation{}, false
	}
	return newComplexTypeDerivationForComplexType(*ct), true
}

// ElementName returns a compiler-owned element name by ID.
func (rt *schemaBuild) ElementName(id ElementID) (QName, bool) {
	decl, ok := elementDeclByID(rt.Elements, id)
	if !ok {
		return QName{}, false
	}
	return decl.Name, true
}

// ElementRestriction returns compiler-owned particle-restriction metadata.
func (rt *schemaBuild) ElementRestriction(id ElementID) (particleRestrictionElement, bool) {
	if !validElementID(id, len(rt.Elements)) {
		return particleRestrictionElement{}, false
	}
	decl := rt.Elements[id]
	return particleRestrictionElement{
		Identities: borrowedIdentityConstraintIDs(decl.Identity),
		Type:       decl.Type,
		Block:      decl.Block,
		Fixed:      newValueConstraintIdentity(decl.ValueConstraint.fixedConstraint()),
		Scope:      decl.Scope,
		Nillable:   decl.Nillable,
	}, true
}

// elementTypeLabel names the type t of an element declaration in a
// diagnostic: a named type by its name, an anonymous type, which has no name,
// by its variety.
func (rt *schemaBuild) elementTypeLabel(t TypeID) string {
	name, ok := typeNameByID(rt.SimpleTypes, rt.ComplexTypes, t)
	if !ok {
		panic("invalid runtime type ID")
	}
	if name != noQName() {
		return "type " + rt.Names.Format(name)
	}
	if _, simple := t.Simple(); simple {
		return "local simple type"
	}
	return "local complex type"
}
