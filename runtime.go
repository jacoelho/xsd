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
	GlobalAttributes map[qName]attributeID
	GlobalElements   map[qName]elementID
	Substitutions    map[elementID][]elementID
	Notations        map[string]bool
	GlobalIdentities map[qName]identityConstraintID
	GlobalTypes      map[qName]typeID
	Identities       []identityConstraint
	ComplexTypes     []complexType
	Wildcards        []wildcard
	AttributeUseSets []attributeUseSet
	Models           []contentModel
	SimpleTypes      []simpleType
	Attributes       []attributeDecl
	Elements         []elementDecl
	Names            nameTable
	Builtin          builtinIDs
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

type elementDecl struct {
	Default    string
	Fixed      string
	Identity   []identityConstraintID
	Name       qName
	Type       typeID
	SubstHead  elementID
	Nillable   bool
	Abstract   bool
	HasDefault bool
	HasFixed   bool
	Block      derivationMask
	Final      derivationMask
}

type identityKind uint8

const (
	identityUnique identityKind = iota
	identityKey
	identityKeyRef
)

type identityConstraint struct {
	Selector []identityPath
	Fields   []identityField
	Name     qName
	Refer    identityConstraintID
	Kind     identityKind
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
	Default    string
	Fixed      string
	Name       qName
	Type       simpleTypeID
	HasDefault bool
	HasFixed   bool
}

type attributeUseSet struct {
	Uses     []attributeUse
	wildcard wildcardID
}

type attributeUse struct {
	Default    string
	Fixed      string
	Name       qName
	Type       simpleTypeID
	Required   bool
	Prohibited bool
	HasDefault bool
	HasFixed   bool
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

type simpleType struct {
	Facets     facetSet
	Union      []simpleTypeID
	Name       qName
	Base       simpleTypeID
	ListItem   simpleTypeID
	Variety    simpleVariety
	Primitive  primitiveKind
	Final      derivationMask
	Whitespace whitespaceMode
	Identity   simpleIdentityKind
	Missing    bool
}

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
}

func (f facetSet) empty() bool {
	return f.Length == nil &&
		f.MinLength == nil &&
		f.MaxLength == nil &&
		f.TotalDigits == nil &&
		f.FractionDigits == nil &&
		f.MinInclusive == nil &&
		f.MaxInclusive == nil &&
		f.MinExclusive == nil &&
		f.MaxExclusive == nil &&
		len(f.Enumeration) == 0 &&
		len(f.Patterns) == 0
}

func (f facetSet) needsLexical() bool {
	return len(f.Patterns) != 0
}

func (f facetSet) needsCanonical() bool {
	return len(f.Enumeration) != 0
}

type compiledLiteral struct {
	Lexical   string
	Canonical string
}

type patternGroup struct {
	Patterns []pattern
}

type pattern struct {
	RE        *regexp.Regexp
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
	CountLimits []restrictionCountLimit
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

type restrictionCountLimit struct {
	particle uint32
	Max      uint32
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
	Replay    bool
	SkipUPA   bool
}

type particle struct {
	Kind     particleKind
	occurs   occurrence
	Element  elementID
	Model    contentModelID
	wildcard wildcardID
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

func (rt runtimeSchema) typeName(t typeID) qName {
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

func (rt *runtimeSchema) substitutionTypeBlocks(t, base typeID) derivationMask {
	var blocks derivationMask
	if base.Kind == typeComplex && int(base.ID) < len(rt.ComplexTypes) {
		blocks |= rt.ComplexTypes[base.ID].Block
	}
	if t.Kind != typeComplex {
		return blocks
	}
	current := complexTypeID(t.ID)
	for steps := 0; steps < len(rt.ComplexTypes); steps++ {
		if int(current) >= len(rt.ComplexTypes) {
			return blocks
		}
		ct := rt.ComplexTypes[current]
		if ct.Base == base {
			return blocks
		}
		if ct.Base.Kind != typeComplex {
			return blocks
		}
		parent := complexTypeID(ct.Base.ID)
		if int(parent) >= len(rt.ComplexTypes) {
			return blocks
		}
		blocks |= rt.ComplexTypes[parent].Block
		current = parent
	}
	return blocks
}

func (rt *runtimeSchema) complexSimpleTypeDerivationMask(t complexTypeID, base simpleTypeID) (derivationMask, bool) {
	if int(t) >= len(rt.ComplexTypes) {
		return 0, false
	}
	ct := rt.ComplexTypes[t]
	if !ct.SimpleValue {
		return 0, false
	}
	var mask derivationMask
	var ok bool
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
	}
	return mask, true
}

func (rt *runtimeSchema) complexAnyTypeDerivationMask(t complexTypeID) (derivationMask, bool) {
	seen := make(map[complexTypeID]bool)
	var mask derivationMask
	for {
		if t == rt.Builtin.AnyType {
			return mask, true
		}
		if int(t) >= len(rt.ComplexTypes) || seen[t] {
			return 0, false
		}
		seen[t] = true
		ct := rt.ComplexTypes[t]
		switch ct.Derivation {
		case derivationExtension:
			mask |= blockExtension
		case derivationRestriction:
			mask |= blockRestriction
		}
		if ct.Base.Kind == typeSimple {
			return mask | blockRestriction, true
		}
		if ct.Base.Kind != typeComplex || complexTypeID(ct.Base.ID) == noComplexType {
			return 0, false
		}
		t = complexTypeID(ct.Base.ID)
	}
}

func (rt *runtimeSchema) simpleTypeDerivationMask(t, base simpleTypeID, seen map[[2]simpleTypeID]bool) (derivationMask, bool) {
	if t == base {
		return 0, true
	}
	if int(t) >= len(rt.SimpleTypes) || int(base) >= len(rt.SimpleTypes) {
		return 0, false
	}
	pair := [2]simpleTypeID{t, base}
	if seen[pair] {
		return 0, false
	}
	seen[pair] = true

	baseType := rt.SimpleTypes[base]
	if baseType.Variety == varietyUnion {
		for _, member := range baseType.Union {
			if mask, ok := rt.simpleTypeDerivationMask(t, member, seen); ok {
				return mask | blockRestriction, true
			}
		}
	}

	st := rt.SimpleTypes[t]
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
	seen := make(map[complexTypeID]bool)
	var mask derivationMask
	for {
		if int(t) >= len(rt.ComplexTypes) || seen[t] {
			return 0, false
		}
		seen[t] = true
		ct := rt.ComplexTypes[t]
		if ct.Base.Kind != typeComplex || complexTypeID(ct.Base.ID) == noComplexType {
			return 0, false
		}
		switch ct.Derivation {
		case derivationExtension:
			mask |= blockExtension
		case derivationRestriction:
			mask |= blockRestriction
		}
		if complexTypeID(ct.Base.ID) == base {
			return mask, true
		}
		t = complexTypeID(ct.Base.ID)
	}
}

func (rt runtimeSchema) typeLabel(t typeID) string {
	q := rt.typeName(t)
	return rt.Names.Format(q)
}

func (o occurrence) allows(n uint32) bool {
	if n < o.Min {
		return false
	}
	if o.Unbounded {
		return true
	}
	return n <= o.Max
}

func (o occurrence) canAdd(n uint32) bool {
	if o.Unbounded {
		return true
	}
	return n < o.Max
}

func (o occurrence) isExactlyOne() bool {
	return o.Min == 1 && o.Max == 1 && !o.Unbounded
}
