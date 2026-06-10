package xsd

import "regexp"

type typeKind uint8

const (
	typeSimple typeKind = iota
	typeComplex
)

type typeID struct {
	Kind typeKind
	ID   uint32
}

func simpleRef(id simpleTypeID) typeID {
	return typeID{Kind: typeSimple, ID: uint32(id)}
}

func complexRef(id complexTypeID) typeID {
	return typeID{Kind: typeComplex, ID: uint32(id)}
}

func (t typeID) simple() (simpleTypeID, bool) {
	if t.Kind != typeSimple {
		return noSimpleType, false
	}
	return simpleTypeID(t.ID), true
}

func (t typeID) complex() (complexTypeID, bool) {
	if t.Kind != typeComplex {
		return noComplexType, false
	}
	return complexTypeID(t.ID), true
}

type simpleTypeID uint32
type complexTypeID uint32
type elementID uint32
type attributeID uint32
type contentModelID uint32
type attributeUseSetID uint32
type wildcardID uint32
type identityConstraintID uint32

const noSimpleType = simpleTypeID(^uint32(0))
const noComplexType = complexTypeID(^uint32(0))
const noElement = elementID(^uint32(0))
const noContentModel = contentModelID(^uint32(0))
const noAttributeUseSet = attributeUseSetID(^uint32(0))
const noWildcard = wildcardID(^uint32(0))
const noIdentityConstraint = identityConstraintID(^uint32(0))

type runtimeSchema struct {
	GlobalAttributes   map[qName]attributeID
	GlobalElements     map[qName]elementID
	Substitutions      map[elementID][]elementID
	SubstitutionLookup map[elementID]map[qName]elementID
	Notations          map[string]bool
	GlobalIdentities   map[qName]identityConstraintID
	GlobalTypes        map[qName]typeID
	Identities         []identityConstraint
	ComplexTypes       []complexType
	Wildcards          []wildcard
	AttributeUseSets   []attributeUseSet
	Models             []contentModel
	CompiledModels     []compiledModel
	SimpleTypes        []simpleType
	Attributes         []attributeDecl
	Elements           []elementDecl
	Names              nameTable
	Builtin            builtinIDs

	SimpleIdentitiesClassified bool
}

type builtinIDs struct {
	AnyType       complexTypeID
	AnySimpleType simpleTypeID
	String        simpleTypeID
	Boolean       simpleTypeID
	Decimal       simpleTypeID
	Integer       simpleTypeID
	Int           simpleTypeID
	Date          simpleTypeID
	DateTime      simpleTypeID
	Time          simpleTypeID
	AnyURI        simpleTypeID
	qName         simpleTypeID
	ID            simpleTypeID
	IDREF         simpleTypeID
	IDREFS        simpleTypeID
	NMTOKEN       simpleTypeID
	NMTOKENS      simpleTypeID
	ENTITY        simpleTypeID
	ENTITIES      simpleTypeID
}

type valueConstraint struct {
	Lexical   string
	Canonical string
	Value     simpleValue
	Present   bool
}

type elementDecl struct {
	Default   valueConstraint
	Fixed     valueConstraint
	Identity  []identityConstraintID
	Type      typeID
	Name      qName
	SubstHead elementID
	Nillable  bool
	Abstract  bool
	Block     derivationMask
	Final     derivationMask
}

type identityKind uint8

const (
	identityUnique identityKind = iota
	identityKey
	identityKeyRef
)

type identityConstraint struct {
	Selector                []identityPath
	Fields                  []identityField
	ElementFields           []compiledIdentityField
	AttributeFields         map[qName][]compiledIdentityField
	AttributeWildcardFields []compiledIdentityField
	Name                    qName
	Refer                   identityConstraintID
	Kind                    identityKind
}

type compiledIdentityField struct {
	Paths []identityFieldPath
	Field int
}

type identityPath struct {
	Steps      []identityStep
	Descendant bool
	Self       bool
}

type identityStep struct {
	Name         qName
	wildcard     bool
	NamespaceSet bool
	Namespace    namespaceID
}

type identityField struct {
	Paths []identityFieldPath
}

type identityFieldPath struct {
	Steps            []identityStep
	Attribute        qName
	AttrNamespace    namespaceID
	Descendant       bool
	Self             bool
	Attr             bool
	AttrWildcard     bool
	AttrNamespaceSet bool
}

type attributeDecl struct {
	Default valueConstraint
	Fixed   valueConstraint
	Name    qName
	Type    simpleTypeID
}

type attributeUseSet struct {
	Index            map[qName]uint32
	Uses             []attributeUse
	Required         []uint32
	ValueConstraints []uint32
	wildcard         wildcardID
}

type attributeUse struct {
	Default    valueConstraint
	Fixed      valueConstraint
	Name       qName
	Type       simpleTypeID
	Required   bool
	Prohibited bool
}

type simpleVariety uint8

const (
	varietyAtomic simpleVariety = iota
	varietyList
	varietyUnion
)

type primitiveKind uint8

const (
	primString primitiveKind = iota
	primBoolean
	primDecimal
	primFloat
	primDouble
	primDuration
	primDateTime
	primTime
	primDate
	primGYearMonth
	primGYear
	primGMonthDay
	primGDay
	primGMonth
	primHexBinary
	primBase64Binary
	primAnyURI
	primQName
	primNotation
)

type whitespaceMode uint8

const (
	whitespacePreserve whitespaceMode = iota
	whitespaceReplace
	whitespaceCollapse
)

type simpleIdentityKind uint8

const (
	simpleIdentityNone simpleIdentityKind = iota
	simpleIdentityID
	simpleIdentityIDREF
	simpleIdentityIDREFList
)

type builtinValidationKind uint8

const (
	builtinValidationNone builtinValidationKind = iota
	builtinValidationInteger
	builtinValidationName
	builtinValidationNCName
	builtinValidationNMTOKEN
	builtinValidationLanguage
	builtinValidationEntity
	builtinValidationXMLLang
	builtinValidationXMLSpace
)

type simpleType struct {
	Union     []simpleTypeID
	Facets    facetSet
	Name      qName
	Base      simpleTypeID
	ListItem  simpleTypeID
	Variety   simpleVariety
	Primitive primitiveKind
	Final     derivationMask
	// Whitespace holds the whiteSpace facet value; it lives on the type
	// because every simple type has one. Only its fixedness is facet state
	// (facetFlagWhiteSpace in Facets.Fixed).
	Whitespace whitespaceMode
	Builtin    builtinValidationKind
	Identity   simpleIdentityKind
	Missing    bool
}

type facetFlag uint16

const (
	facetFlagLength facetFlag = 1 << iota
	facetFlagMinLength
	facetFlagMaxLength
	facetFlagTotalDigits
	facetFlagFractionDigits
	facetFlagMinInclusive
	facetFlagMaxInclusive
	facetFlagMinExclusive
	facetFlagMaxExclusive
	facetFlagEnumeration
	facetFlagPattern
	// facetFlagWhiteSpace is valid only in facetSet.Fixed; the whiteSpace
	// value itself is simpleType.Whitespace and is never absent.
	facetFlagWhiteSpace
)

const facetValueMask = facetFlagLength | facetFlagMinLength | facetFlagMaxLength |
	facetFlagTotalDigits | facetFlagFractionDigits |
	facetFlagMinInclusive | facetFlagMaxInclusive | facetFlagMinExclusive | facetFlagMaxExclusive

const facetLengthMask = facetFlagLength | facetFlagMinLength | facetFlagMaxLength

type facetSet struct {
	Length         *uint32
	MinLength      *uint32
	MaxLength      *uint32
	TotalDigits    *uint32
	FractionDigits *uint32
	MinInclusive   *compiledLiteral
	MaxInclusive   *compiledLiteral
	MinExclusive   *compiledLiteral
	MaxExclusive   *compiledLiteral
	Enumeration    []compiledLiteral
	Patterns       []patternGroup
	Present        facetFlag
	Fixed          facetFlag
}

func (f facetSet) empty() bool {
	return f.Present == 0
}

func (f facetSet) onlyPatterns() bool {
	return f.Present == facetFlagPattern
}

func (f facetSet) onlyEnumeration() bool {
	return f.Present == facetFlagEnumeration
}

func (f facetSet) hasValueFacets() bool {
	return f.Present&facetValueMask != 0
}

func (f facetSet) needsLexical() bool {
	return f.Present&facetFlagPattern != 0
}

func (f facetSet) needsCanonical() bool {
	return f.Present&facetFlagEnumeration != 0
}

func (f facetSet) needsLength() bool {
	return f.Present&facetLengthMask != 0
}

type compiledLiteral struct {
	Lexical   string
	Canonical string
	Actual    actualValue
}

type patternGroup struct {
	Patterns []pattern
}

type pattern struct {
	RE        *regexp.Regexp
	Fast      *simplePattern
	XSDSource string
	GoSource  string
}

type derivationKind uint8

const (
	derivationNone derivationKind = iota
	derivationRestriction
	derivationExtension
)

type derivationMask uint8

const (
	blockExtension derivationMask = 1 << iota
	blockRestriction
	blockSubstitution
	blockList
	blockUnion
)

type complexType struct {
	Name        qName
	Base        typeID
	Content     contentModelID
	Attrs       attributeUseSetID
	TextType    simpleTypeID
	Mixed       bool
	Abstract    bool
	Derivation  derivationKind
	Block       derivationMask
	Final       derivationMask
	SimpleValue bool
}

type modelKind uint8

const (
	modelEmpty modelKind = iota
	modelAny
	modelSequence
	modelChoice
	modelAll
)

type particleKind uint8

const (
	particleElement particleKind = iota
	particleModel
	particleWildcard
)

type occurrence struct {
	Min       uint32
	Max       uint32
	Unbounded bool
}

type contentModel struct {
	Particles []particle
	occurs    occurrence
	Kind      modelKind
	Mixed     bool
}

type particle struct {
	Kind     particleKind
	occurs   occurrence
	Element  elementID
	Model    contentModelID
	wildcard wildcardID
}

type compiledModelKind uint8

const (
	compiledModelEmpty compiledModelKind = iota
	compiledModelAny
	compiledModelAll
	compiledModelDFA
)

type compiledModel struct {
	Rows      []compiledModelRow
	All       []compiledAllTerm
	Start     uint32
	AllBitLen uint32
	Kind      compiledModelKind
	Mixed     bool
	Empty     bool
}

type compiledModelRow struct {
	Edges         []compiledModelEdge
	NameToEdge    map[qName]uint32 // element-name → edge position; nil for narrow or ambiguous rows
	WildcardEdges []uint32         // wildcard edge positions in row order; only set with NameToEdge
	CountParticle particle
	Min           uint32
	Max           uint32
	Accept        bool
	Counted       bool
	Unbounded     bool
}

type compiledModelEdge struct {
	Particle particle
	To       uint32
}

type compiledAllTerm struct {
	Particle particle
	Required bool
}

type wildcardMode uint8

const (
	wildAny wildcardMode = iota
	wildOther
	wildLocal
	wildTargetNamespace
	wildList
)

type processContents uint8

const (
	processStrict processContents = iota
	processLax
	processSkip
)

type wildcard struct {
	Namespaces []namespaceID
	OtherThan  namespaceID
	Mode       wildcardMode
	Process    processContents
}

func (rt *runtimeSchema) simpleType(id simpleTypeID) (*simpleType, bool) {
	if !validUint32Index(uint32(id), len(rt.SimpleTypes)) {
		return nil, false
	}
	return &rt.SimpleTypes[id], true
}

func (rt *runtimeSchema) complexType(id complexTypeID) (*complexType, bool) {
	if !validUint32Index(uint32(id), len(rt.ComplexTypes)) {
		return nil, false
	}
	return &rt.ComplexTypes[id], true
}

func (rt *runtimeSchema) typeName(t typeID) qName {
	if t.Kind == typeSimple {
		return rt.SimpleTypes[t.ID].Name
	}
	return rt.ComplexTypes[t.ID].Name
}

func (rt *runtimeSchema) typeDerivationMask(t, base typeID) (derivationMask, bool) {
	if t == base {
		return 0, true
	}
	if t.Kind == typeSimple && base.Kind == typeComplex && complexTypeID(base.ID) == rt.Builtin.AnyType {
		return blockRestriction, true
	}
	if t.Kind == typeComplex && base.Kind == typeComplex && complexTypeID(base.ID) == rt.Builtin.AnyType {
		return rt.complexAnyTypeDerivationMask(complexTypeID(t.ID))
	}
	if t.Kind == typeComplex && base.Kind == typeSimple {
		return rt.complexSimpleTypeDerivationMask(complexTypeID(t.ID), simpleTypeID(base.ID))
	}
	if t.Kind != base.Kind {
		return 0, false
	}
	if t.Kind == typeSimple {
		return rt.simpleTypeDerivationMask(simpleTypeID(t.ID), simpleTypeID(base.ID), make(map[[2]simpleTypeID]bool))
	}
	return rt.complexTypeDerivationMask(complexTypeID(t.ID), complexTypeID(base.ID))
}

func (rt *runtimeSchema) substitutionDerivationAllowed(t, base typeID, block derivationMask) bool {
	mask, ok := rt.typeDerivationMask(t, base)
	if !ok {
		return false
	}
	if mask&block != 0 {
		return false
	}
	return mask&rt.substitutionTypeBlocks(t, base) == 0
}

func (rt *runtimeSchema) substitutionAllowed(headID, memberID elementID) bool {
	head := rt.Elements[headID]
	member := rt.Elements[memberID]
	if head.Block&blockSubstitution != 0 {
		return false
	}
	return rt.substitutionDerivationAllowed(member.Type, head.Type, head.Block)
}

func (rt *runtimeSchema) substitutionTypeBlocks(t, base typeID) derivationMask {
	var blocks derivationMask
	if baseID, ok := base.complex(); ok {
		if baseCT, ok := rt.complexType(baseID); ok {
			blocks |= baseCT.Block
		}
	}
	current, ok := t.complex()
	if !ok {
		return blocks
	}
	for range len(rt.ComplexTypes) {
		ct, ok := rt.complexType(current)
		if !ok {
			return blocks
		}
		if ct.Base == base {
			return blocks
		}
		parent, ok := ct.Base.complex()
		if !ok {
			return blocks
		}
		parentCT, ok := rt.complexType(parent)
		if !ok {
			return blocks
		}
		blocks |= parentCT.Block
		current = parent
	}
	return blocks
}

func (rt *runtimeSchema) complexSimpleTypeDerivationMask(t complexTypeID, base simpleTypeID) (derivationMask, bool) {
	ct, ok := rt.complexType(t)
	if !ok || !ct.SimpleValue {
		return 0, false
	}
	var mask derivationMask
	switch ct.Base.Kind {
	case typeSimple:
		mask, ok = rt.simpleTypeDerivationMask(simpleTypeID(ct.Base.ID), base, make(map[[2]simpleTypeID]bool))
	case typeComplex:
		mask, ok = rt.complexSimpleTypeDerivationMask(complexTypeID(ct.Base.ID), base)
	default:
		return 0, false
	}
	if !ok {
		return 0, false
	}
	switch ct.Derivation {
	case derivationExtension:
		mask |= blockExtension
	case derivationRestriction:
		mask |= blockRestriction
	case derivationNone:
	}
	return mask, true
}

func (rt *runtimeSchema) complexAnyTypeDerivationMask(t complexTypeID) (derivationMask, bool) {
	var mask derivationMask
	for range len(rt.ComplexTypes) {
		if t == rt.Builtin.AnyType {
			return mask, true
		}
		ct, ok := rt.complexType(t)
		if !ok {
			return 0, false
		}
		switch ct.Derivation {
		case derivationExtension:
			mask |= blockExtension
		case derivationRestriction:
			mask |= blockRestriction
		case derivationNone:
		}
		if ct.Base.Kind == typeSimple {
			return mask | blockRestriction, true
		}
		parent, ok := ct.Base.complex()
		if !ok || parent == noComplexType {
			return 0, false
		}
		t = parent
	}
	return 0, false
}

func (rt *runtimeSchema) simpleTypeDerivationMask(t, base simpleTypeID, seen map[[2]simpleTypeID]bool) (derivationMask, bool) {
	if t == base {
		return 0, true
	}
	st, ok := rt.simpleType(t)
	if !ok {
		return 0, false
	}
	baseType, ok := rt.simpleType(base)
	if !ok {
		return 0, false
	}
	pair := [2]simpleTypeID{t, base}
	if seen[pair] {
		return 0, false
	}
	seen[pair] = true

	if baseType.Variety == varietyUnion {
		for _, member := range baseType.Union {
			if mask, ok := rt.simpleTypeDerivationMask(t, member, seen); ok {
				return mask | blockRestriction, true
			}
		}
	}

	if st.Base == noSimpleType || st.Base == t {
		return 0, false
	}
	mask, ok := rt.simpleTypeDerivationMask(st.Base, base, seen)
	if !ok {
		return 0, false
	}
	return mask | blockRestriction, true
}

func (rt *runtimeSchema) complexTypeDerivationMask(t, base complexTypeID) (derivationMask, bool) {
	var mask derivationMask
	for range len(rt.ComplexTypes) {
		ct, ok := rt.complexType(t)
		if !ok {
			return 0, false
		}
		parent, ok := ct.Base.complex()
		if !ok || parent == noComplexType {
			return 0, false
		}
		switch ct.Derivation {
		case derivationExtension:
			mask |= blockExtension
		case derivationRestriction:
			mask |= blockRestriction
		case derivationNone:
		}
		if parent == base {
			return mask, true
		}
		t = parent
	}
	return 0, false
}

func (rt *runtimeSchema) typeLabel(t typeID) string {
	q := rt.typeName(t)
	return rt.Names.Format(q)
}

func (o occurrence) isExactlyOne() bool {
	return o.Min == 1 && o.Max == 1 && !o.Unbounded
}
