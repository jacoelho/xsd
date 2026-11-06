package schema

import "github.com/jacoelho/xsd/internal/value"

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
	CompiledModels        []compiledModel
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
)

// newComplexTypeReads points each type at published[i], its model's index in
// the published content-model table.
func newComplexTypeReads(types []complexTypeMetadata, published []contentModelID) []complexTypeRead {
	reads := make([]complexTypeRead, len(types))
	for i := range types {
		reads[i] = newComplexTypeRead(types[i], published[i])
	}
	return reads
}

func newComplexTypeRead(ct complexTypeMetadata, contentModel contentModelID) complexTypeRead {
	var flags complexTypeReadFlags
	if ct.Abstract {
		flags |= complexTypeReadAbstract
	}
	return complexTypeRead{
		attributeUseSet: ct.Attrs,
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

// TypeName returns a compiler-owned type name.
func (rt *schemaBuild) TypeName(t TypeID) QName {
	name, ok := typeNameByID(rt.SimpleTypes, rt.ComplexTypes, t)
	if !ok {
		panic("invalid runtime type ID")
	}
	return name
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
		Fixed:      newValueConstraintIdentity(decl.Fixed),
		Scope:      decl.Scope,
		Nillable:   decl.Nillable,
	}, true
}

// TypeLabel formats a compiler-owned type name for diagnostics.
func (rt *schemaBuild) TypeLabel(t TypeID) string {
	return rt.Names.Format(rt.TypeName(t))
}
