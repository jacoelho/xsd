package runtime

import (
	"errors"

	"github.com/jacoelho/xsd/internal/vocab"
)

type builtinAttributeExpectation struct {
	ns      string
	local   string
	typ     SimpleTypeID
	handle  builtinSimpleHandle
	builtin BuiltinValidationKind
}

// BuiltinAttributeInternalTypes stores internal simple types used only by the
// fixed XML attribute declarations.
type BuiltinAttributeInternalTypes struct {
	XMLLang  SimpleTypeID
	XMLSpace SimpleTypeID
}

type builtinAttributeInternalHandle uint8

const (
	builtinAttributeInternalXMLLang builtinAttributeInternalHandle = iota
	builtinAttributeInternalXMLSpace
)

var builtinAttributeSimpleSeedTable = [...]BuiltinAttributeSimpleSeed{
	{
		Namespace: XMLNamespaceURI,
		Local:     vocab.XMLAttrLang,
		builtin:   BuiltinValidationXMLLang,
		handle:    builtinAttributeInternalXMLLang,
	},
	{
		Namespace: XMLNamespaceURI,
		Local:     vocab.XMLAttrSpace,
		builtin:   BuiltinValidationXMLSpace,
		handle:    builtinAttributeInternalXMLSpace,
	},
}

// BuiltinAttributeSimpleSeed is the runtime-owned construction spec for one
// internal simple type used by fixed XML attribute declarations.
type BuiltinAttributeSimpleSeed struct {
	Namespace string
	Local     string
	builtin   BuiltinValidationKind
	handle    builtinAttributeInternalHandle
}

// BuiltinAttributeSimpleSeedCount returns the number of fixed internal simple
// types required by XML attribute declarations.
func BuiltinAttributeSimpleSeedCount() int {
	return len(builtinAttributeSimpleSeedTable)
}

// BuiltinAttributeSimpleSeedAt returns one fixed internal XML attribute simple
// type seed.
func BuiltinAttributeSimpleSeedAt(i int) (BuiltinAttributeSimpleSeed, bool) {
	if i < 0 || i >= len(builtinAttributeSimpleSeedTable) {
		return BuiltinAttributeSimpleSeed{}, false
	}
	return builtinAttributeSimpleSeedTable[i], true
}

// BaseID returns the base simple type assigned to this internal built-in type.
func (s BuiltinAttributeSimpleSeed) BaseID(ids BuiltinIDs) (SimpleTypeID, bool) {
	return ids.String, ids.String != NoSimpleType
}

// SimpleType returns the internal runtime declaration represented by s.
func (s BuiltinAttributeSimpleSeed) SimpleType(name QName, base SimpleTypeID) SimpleType {
	st := SimpleType{
		Name:       name,
		Variety:    SimpleVarietyAtomic,
		Primitive:  PrimitiveString,
		Base:       base,
		ListItem:   NoSimpleType,
		Whitespace: WhitespaceCollapse,
		Builtin:    s.builtin,
	}
	st.Fast = DeriveSimpleFastPathForSimpleType(st)
	return st
}

// RecordID records id in the internal XML attribute simple-type handle table.
func (s BuiltinAttributeSimpleSeed) RecordID(types *BuiltinAttributeInternalTypes, id SimpleTypeID) {
	if types == nil {
		return
	}
	switch s.handle {
	case builtinAttributeInternalXMLLang:
		types.XMLLang = id
	case builtinAttributeInternalXMLSpace:
		types.XMLSpace = id
	}
}

// BuiltinAttributeSeed is the runtime-owned construction spec for one built-in
// XML or XLink global attribute.
type BuiltinAttributeSeed struct {
	Namespace string
	Local     string
	handle    builtinSimpleHandle
	builtin   BuiltinValidationKind
}

var builtinAttributeSeedTable = [...]BuiltinAttributeSeed{
	{Namespace: XMLNamespaceURI, Local: vocab.XMLAttrBase, handle: builtinSimpleAnyURI},
	{Namespace: XMLNamespaceURI, Local: vocab.XMLAttrID, handle: builtinSimpleID},
	{Namespace: XMLNamespaceURI, Local: vocab.XMLAttrLang, builtin: BuiltinValidationXMLLang},
	{Namespace: XMLNamespaceURI, Local: vocab.XMLAttrSpace, builtin: BuiltinValidationXMLSpace},
	{Namespace: XLinkNamespaceURI, Local: vocab.XLinkAttrType, handle: builtinSimpleString},
	{Namespace: XLinkNamespaceURI, Local: vocab.XLinkAttrHref, handle: builtinSimpleAnyURI},
	{Namespace: XLinkNamespaceURI, Local: vocab.XLinkAttrRole, handle: builtinSimpleAnyURI},
	{Namespace: XLinkNamespaceURI, Local: vocab.XLinkAttrArcrole, handle: builtinSimpleAnyURI},
	{Namespace: XLinkNamespaceURI, Local: vocab.XLinkAttrTitle, handle: builtinSimpleString},
	{Namespace: XLinkNamespaceURI, Local: vocab.XLinkAttrShow, handle: builtinSimpleString},
	{Namespace: XLinkNamespaceURI, Local: vocab.XLinkAttrActuate, handle: builtinSimpleString},
}

// BuiltinAttributeSeedAt returns one fixed XML/XLink global attribute seed.
func BuiltinAttributeSeedAt(i int) (BuiltinAttributeSeed, bool) {
	if i < 0 || i >= len(builtinAttributeSeedTable) {
		return BuiltinAttributeSeed{}, false
	}
	return builtinAttributeSeedTable[i], true
}

// TypeID returns the simple type assigned to this built-in attribute.
func (s BuiltinAttributeSeed) TypeID(ids BuiltinIDs, internal BuiltinAttributeInternalTypes) (SimpleTypeID, bool) {
	switch s.builtin {
	case BuiltinValidationXMLLang:
		return internal.XMLLang, internal.XMLLang != NoSimpleType
	case BuiltinValidationXMLSpace:
		return internal.XMLSpace, internal.XMLSpace != NoSimpleType
	case BuiltinValidationNone,
		BuiltinValidationInteger,
		BuiltinValidationName,
		BuiltinValidationNCName,
		BuiltinValidationNMTOKEN,
		BuiltinValidationLanguage,
		BuiltinValidationEntity:
	}
	switch s.handle {
	case builtinSimpleString:
		return ids.String, ids.String != NoSimpleType
	case builtinSimpleAnyURI:
		return ids.AnyURI, ids.AnyURI != NoSimpleType
	case builtinSimpleID:
		return ids.ID, ids.ID != NoSimpleType
	case builtinSimpleNoHandle,
		builtinSimpleAnySimpleType,
		builtinSimpleBoolean,
		builtinSimpleDecimal,
		builtinSimpleInteger,
		builtinSimpleInt,
		builtinSimpleDate,
		builtinSimpleDateTime,
		builtinSimpleTime,
		builtinSimpleQNameHandle,
		builtinSimpleIDREF,
		builtinSimpleIDREFS,
		builtinSimpleNMTOKEN,
		builtinSimpleNMTOKENS,
		builtinSimpleENTITY,
		builtinSimpleENTITIES:
	}
	return NoSimpleType, false
}

// BuiltinAnyTypeLocalName returns the fixed local name for xs:anyType.
func BuiltinAnyTypeLocalName() string {
	return vocab.XSDValueAnyType
}

// BuiltinAnyTypeWildcard returns the attribute wildcard used by xs:anyType.
func BuiltinAnyTypeWildcard() Wildcard {
	return Wildcard{Mode: WildcardAny, Process: ProcessLax}
}

// BuiltinAnyTypeAttributeUseSet returns the attribute-use set used by
// xs:anyType.
func BuiltinAnyTypeAttributeUseSet(wildcard WildcardID) AttributeUseSet {
	return AttributeUseSet{
		Wildcard:         wildcard,
		WildcardBase:     NoWildcard,
		WildcardDeclared: wildcard,
	}
}

// BuiltinAnyTypeContentModel returns the content model used by xs:anyType.
func BuiltinAnyTypeContentModel() ContentModel {
	return ContentModel{Kind: ModelAny, Mixed: true}
}

// BuiltinAnyTypeComplexType returns the fixed xs:anyType complex-type
// declaration using already allocated child component IDs.
func BuiltinAnyTypeComplexType(name QName, content ContentModelID, attrs AttributeUseSetID) ComplexType {
	return ComplexType{
		Name:        name,
		Content:     content,
		Attrs:       attrs,
		TextType:    NoSimpleType,
		ContentKind: ContentMixed,
	}
}

// BuiltinSimpleFacetExpectation is the schema-private facet value check that
// remains after runtime validates built-in simple-type shape.
type BuiltinSimpleFacetExpectation struct {
	Local             string
	MinInclusive      string
	MaxInclusive      string
	Type              SimpleTypeID
	MinLength         uint32
	HasFractionDigits bool
	HasMinLength      bool
}

// BuiltinUnsignedFacet is the runtime projection of a uint32-valued facet.
type BuiltinUnsignedFacet struct {
	Value   uint32
	Present bool
}

// BuiltinDecimalFacet is the runtime projection of a decimal bound facet.
type BuiltinDecimalFacet struct {
	Canonical            string
	Lexical              string
	ActualKind           PrimitiveKind
	Type                 SimpleTypeID
	Present              bool
	ActualValid          bool
	HasResolvedNames     bool
	ValueMatchesExpected bool
}

// BuiltinSimpleFacetValidation is the runtime projection needed to validate
// facets on fixed built-in simple-type declarations.
type BuiltinSimpleFacetValidation struct {
	MinInclusive    BuiltinDecimalFacet
	MaxInclusive    BuiltinDecimalFacet
	FractionDigits  BuiltinUnsignedFacet
	MinLength       BuiltinUnsignedFacet
	EnumerationSize int
	PatternSize     int
	Present         FacetMask
	Fixed           FacetMask
	HasLength       bool
	HasMaxLength    bool
	HasTotalDigits  bool
	HasMinExclusive bool
	HasMaxExclusive bool
}

type builtinSimpleExpectation struct {
	local             string
	baseLocal         string
	listItemLocal     string
	minInclusive      string
	maxInclusive      string
	id                SimpleTypeID
	minLength         uint32
	variety           SimpleVariety
	primitive         PrimitiveKind
	whitespace        WhitespaceMode
	builtin           BuiltinValidationKind
	identity          SimpleIdentityKind
	checkID           bool
	hasFractionDigits bool
	hasMinLength      bool
}

// BuiltinSimpleSeed is the runtime-owned construction spec for one built-in
// simple type.
type BuiltinSimpleSeed struct {
	Namespace         string
	Local             string
	minInclusive      CompiledLiteral
	maxInclusive      CompiledLiteral
	MinLength         uint32
	Base              SimpleTypeID
	ListItem          SimpleTypeID
	Variety           SimpleVariety
	Primitive         PrimitiveKind
	Whitespace        WhitespaceMode
	Builtin           BuiltinValidationKind
	Identity          SimpleIdentityKind
	Fast              SimpleFastKind
	HasFractionDigits bool
	HasMinLength      bool
	hasMinInclusive   bool
	hasMaxInclusive   bool
	handle            builtinSimpleHandle
}

type builtinSimpleHandle uint8

const (
	builtinSimpleNoHandle builtinSimpleHandle = iota
	builtinSimpleAnySimpleType
	builtinSimpleString
	builtinSimpleBoolean
	builtinSimpleDecimal
	builtinSimpleInteger
	builtinSimpleInt
	builtinSimpleDate
	builtinSimpleDateTime
	builtinSimpleTime
	builtinSimpleAnyURI
	builtinSimpleQNameHandle
	builtinSimpleID
	builtinSimpleIDREF
	builtinSimpleIDREFS
	builtinSimpleNMTOKEN
	builtinSimpleNMTOKENS
	builtinSimpleENTITY
	builtinSimpleENTITIES
)

// BuiltinSimpleSeedCount returns the number of topologically ordered fixed XSD
// simple-type declarations.
func BuiltinSimpleSeedCount() int {
	return len(builtinSimpleSeedTable)
}

// BuiltinSimpleSeedAt returns one fixed XSD simple-type declaration seed.
func BuiltinSimpleSeedAt(i int) (BuiltinSimpleSeed, bool) {
	if i < 0 || i >= len(builtinSimpleSeedTable) {
		return BuiltinSimpleSeed{}, false
	}
	return builtinSimpleSeedTable[i], true
}

var builtinSimpleSeedTable = buildBuiltinSimpleSeedTable()

func buildBuiltinSimpleSeedTable() []BuiltinSimpleSeed {
	seeds := make([]BuiltinSimpleSeed, len(builtinSimpleExpectationTable))
	for i, exp := range builtinSimpleExpectationTable {
		seeds[i] = builtinSimpleSeedForExpectation(exp)
	}
	return seeds
}

func builtinSimpleSeedForExpectation(exp builtinSimpleExpectation) BuiltinSimpleSeed {
	base := builtinSimpleDependencyID(exp.baseLocal)
	return BuiltinSimpleSeed{
		Namespace:         XSDNamespaceURI,
		Local:             exp.local,
		MinLength:         exp.minLength,
		Base:              base,
		ListItem:          builtinSimpleDependencyID(exp.listItemLocal),
		Variety:           exp.variety,
		Primitive:         exp.primitive,
		Whitespace:        exp.whitespace,
		Builtin:           exp.builtin,
		Identity:          exp.identity,
		Fast:              builtinSimpleFastKind(exp),
		HasFractionDigits: exp.hasFractionDigits,
		HasMinLength:      exp.hasMinLength,
		hasMinInclusive:   exp.minInclusive != "",
		hasMaxInclusive:   exp.maxInclusive != "",
		handle:            builtinSimpleHandleForLocal(exp.local),
		minInclusive:      builtinDecimalLiteral(base, exp.minInclusive),
		maxInclusive:      builtinDecimalLiteral(base, exp.maxInclusive),
	}
}

// BuiltinSimpleFacetStorage constructs fixed built-in simple-type facets.
type BuiltinSimpleFacetStorage struct{}

// NewBuiltinSimpleFacetStorage returns storage sized for the fixed built-in
// simple-type facet set.
func NewBuiltinSimpleFacetStorage() BuiltinSimpleFacetStorage {
	return BuiltinSimpleFacetStorage{}
}

// SimpleType returns the runtime declaration represented by seed using
// resolved base and list-item IDs.
func (s *BuiltinSimpleFacetStorage) SimpleType(seed *BuiltinSimpleSeed, name QName, base, listItem SimpleTypeID) SimpleType {
	st := SimpleType{
		Name:       name,
		Variety:    seed.Variety,
		Primitive:  seed.Primitive,
		Base:       base,
		ListItem:   listItem,
		Whitespace: seed.Whitespace,
		Facets:     s.facetSet(seed),
		Builtin:    seed.Builtin,
		Identity:   seed.Identity,
		Fast:       seed.Fast,
	}
	return st
}

// RecordID records id in the built-in handle table when s is a named handle
// used by validation and compile decisions.
func (s *BuiltinSimpleSeed) RecordID(ids *BuiltinIDs, id SimpleTypeID) {
	if ids == nil {
		return
	}
	switch s.handle {
	case builtinSimpleAnySimpleType:
		ids.AnySimpleType = id
	case builtinSimpleString:
		ids.String = id
	case builtinSimpleBoolean:
		ids.Boolean = id
	case builtinSimpleDecimal:
		ids.Decimal = id
	case builtinSimpleInteger:
		ids.Integer = id
	case builtinSimpleInt:
		ids.Int = id
	case builtinSimpleDate:
		ids.Date = id
	case builtinSimpleDateTime:
		ids.DateTime = id
	case builtinSimpleTime:
		ids.Time = id
	case builtinSimpleAnyURI:
		ids.AnyURI = id
	case builtinSimpleQNameHandle:
		ids.QName = id
	case builtinSimpleID:
		ids.ID = id
	case builtinSimpleIDREF:
		ids.IDREF = id
	case builtinSimpleIDREFS:
		ids.IDREFS = id
	case builtinSimpleNMTOKEN:
		ids.NMTOKEN = id
	case builtinSimpleNMTOKENS:
		ids.NMTOKENS = id
	case builtinSimpleENTITY:
		ids.ENTITY = id
	case builtinSimpleENTITIES:
		ids.ENTITIES = id
	case builtinSimpleNoHandle:
	}
}

func (s *BuiltinSimpleFacetStorage) facetSet(seed *BuiltinSimpleSeed) FacetSet {
	var f FacetSet
	if seed.HasFractionDigits {
		f.FractionDigits = 0
		SetFacetPresent(&f, FacetFractionDigits)
	}
	if seed.HasMinLength {
		f.MinLength = seed.MinLength
		SetFacetPresent(&f, FacetMinLength)
	}
	if seed.hasMinInclusive {
		SetBoundFacet(&f, FacetMinInclusive, seed.minInclusive, false)
	}
	if seed.hasMaxInclusive {
		SetBoundFacet(&f, FacetMaxInclusive, seed.maxInclusive, false)
	}
	return f
}

func builtinDecimalLiteral(base SimpleTypeID, v string) CompiledLiteral {
	if v == "" {
		return CompiledLiteral{}
	}
	dec, err := ParseDecimalCanonical(v)
	if err != nil {
		return CompiledLiteral{Lexical: v, Canonical: v, Type: base}
	}
	return NewCompiledLiteralForSimpleType(
		SimpleType{Variety: SimpleVarietyAtomic, Primitive: PrimitiveDecimal},
		base,
		v,
		dec.IntegerCanonicalText(),
		nil,
	)
}

func builtinSimpleDependencyID(local string) SimpleTypeID {
	if local == "" {
		return NoSimpleType
	}
	id, ok := builtinSimpleIDForLocal(local)
	if !ok {
		panic("builtin simple type references missing type: " + local)
	}
	return id
}

func builtinSimpleIDForLocal(local string) (SimpleTypeID, bool) {
	for i, exp := range builtinSimpleExpectationTable {
		if exp.local == local {
			return SimpleTypeID(i), true
		}
	}
	return NoSimpleType, false
}

func builtinSimpleFastKind(exp builtinSimpleExpectation) SimpleFastKind {
	if exp.local == vocab.XSDValueInt &&
		exp.variety == SimpleVarietyAtomic &&
		exp.primitive == PrimitiveDecimal &&
		exp.builtin == BuiltinValidationInteger &&
		exp.whitespace == WhitespaceCollapse &&
		exp.hasFractionDigits &&
		exp.minInclusive == "-2147483648" &&
		exp.maxInclusive == "2147483647" {
		return SimpleFastInt
	}
	return SimpleFastNone
}

func builtinSimpleHandleForLocal(local string) builtinSimpleHandle {
	switch local {
	case vocab.XSDValueAnySimpleType:
		return builtinSimpleAnySimpleType
	case vocab.XSDValueString:
		return builtinSimpleString
	case vocab.XSDValueBoolean:
		return builtinSimpleBoolean
	case vocab.XSDValueDecimal:
		return builtinSimpleDecimal
	case vocab.XSDValueInteger:
		return builtinSimpleInteger
	case vocab.XSDValueInt:
		return builtinSimpleInt
	case vocab.XSDValueDate:
		return builtinSimpleDate
	case vocab.XSDValueDateTime:
		return builtinSimpleDateTime
	case vocab.XSDValueTime:
		return builtinSimpleTime
	case vocab.XSDValueAnyURI:
		return builtinSimpleAnyURI
	case vocab.XSDValueQName:
		return builtinSimpleQNameHandle
	case vocab.XSDValueID:
		return builtinSimpleID
	case vocab.XSDValueIDREF:
		return builtinSimpleIDREF
	case vocab.XSDValueIDREFS:
		return builtinSimpleIDREFS
	case vocab.XSDValueNMTOKEN:
		return builtinSimpleNMTOKEN
	case vocab.XSDValueNMTOKENS:
		return builtinSimpleNMTOKENS
	case vocab.XSDValueENTITY:
		return builtinSimpleENTITY
	case vocab.XSDValueENTITIES:
		return builtinSimpleENTITIES
	default:
		return builtinSimpleNoHandle
	}
}

// BuiltinDeclarationCounts is the runtime projection needed to validate that
// the fixed built-in declaration tables were seeded.
type BuiltinDeclarationCounts struct {
	SimpleTypes      int
	Attributes       int
	ComplexTypes     int
	Wildcards        int
	AttributeUseSets int
	Models           int
}

const (
	builtinInternalAttributeSimpleTypeCount = 2
	builtinComplexTypeDeclarationCount      = 1
)

// NewBuiltinSimpleFacetValidation projects a runtime facet set into the shape
// needed to validate fixed built-in simple-type facets.
func NewBuiltinSimpleFacetValidation(f FacetSet, exp BuiltinSimpleFacetExpectation) BuiltinSimpleFacetValidation {
	minInclusive, hasMinInclusive := BoundFacet(f, FacetMinInclusive)
	maxInclusive, hasMaxInclusive := BoundFacet(f, FacetMaxInclusive)
	return BuiltinSimpleFacetValidation{
		MinInclusive:    newBuiltinDecimalFacet(minInclusive, hasMinInclusive, exp.MinInclusive),
		MaxInclusive:    newBuiltinDecimalFacet(maxInclusive, hasMaxInclusive, exp.MaxInclusive),
		FractionDigits:  newBuiltinUnsignedFacet(f.FractionDigits, f.Present&FacetFractionDigits != 0),
		MinLength:       newBuiltinUnsignedFacet(f.MinLength, f.Present&FacetMinLength != 0),
		EnumerationSize: len(f.Enumeration),
		PatternSize:     int(f.patterns.count()),
		Present:         f.Present,
		Fixed:           f.Fixed,
		HasLength:       f.Present&FacetLength != 0,
		HasMaxLength:    f.Present&FacetMaxLength != 0,
		HasTotalDigits:  f.Present&FacetTotalDigits != 0,
		HasMinExclusive: f.Present&FacetMinExclusive != 0,
		HasMaxExclusive: f.Present&FacetMaxExclusive != 0,
	}
}

func newBuiltinUnsignedFacet(got uint32, present bool) BuiltinUnsignedFacet {
	if !present {
		return BuiltinUnsignedFacet{}
	}
	return BuiltinUnsignedFacet{
		Value:   got,
		Present: true,
	}
}

func newBuiltinDecimalFacet(got CompiledLiteral, present bool, want string) BuiltinDecimalFacet {
	if !present {
		return BuiltinDecimalFacet{}
	}
	proof := false
	expected, err := ParseDecimalValue(want)
	if err == nil && got.Actual.Valid && got.Actual.Kind == PrimitiveDecimal {
		proof = CompareDecimalValues(got.Actual.Decimal, expected) == 0
	}
	return BuiltinDecimalFacet{
		Canonical:            got.Canonical,
		Lexical:              got.Lexical,
		ActualKind:           got.Actual.Kind,
		Type:                 got.Type,
		Present:              true,
		ActualValid:          got.Actual.Valid,
		HasResolvedNames:     len(got.ResolvedNames) != 0,
		ValueMatchesExpected: proof,
	}
}

// BuiltinSimpleTypeCount returns the number of simple-type declarations seeded
// before user schema declarations.
func BuiltinSimpleTypeCount() int {
	return BuiltinSimpleSeedCount() + builtinInternalAttributeSimpleTypeCount
}

// BuiltinAttributeCount returns the number of global attributes seeded before
// user schema declarations.
func BuiltinAttributeCount() int {
	return len(builtinAttributeSeedTable)
}

// BuiltinComplexTypeCount returns the number of complex-type declarations
// seeded before user schema declarations.
func BuiltinComplexTypeCount() int {
	return builtinComplexTypeDeclarationCount
}

// BuiltinGlobalTypeCount returns the number of global type bindings seeded
// before user schema declarations.
func BuiltinGlobalTypeCount() int {
	return len(builtinSimpleExpectationTable) + builtinComplexTypeDeclarationCount
}

// ValidateBuiltinDeclarationCounts validates the required fixed declaration
// table cardinalities seeded into every runtime schema.
func ValidateBuiltinDeclarationCounts(counts BuiltinDeclarationCounts) error {
	if counts.SimpleTypes < BuiltinSimpleTypeCount() ||
		counts.Attributes < BuiltinAttributeCount() ||
		counts.ComplexTypes < builtinComplexTypeDeclarationCount ||
		counts.Wildcards == 0 ||
		counts.AttributeUseSets == 0 ||
		counts.Models == 0 {
		return errors.New("runtime is missing builtin declarations")
	}
	return nil
}

// ValidateBuiltinSimpleFacets validates the fixed facet shape for a built-in
// simple type.
func ValidateBuiltinSimpleFacets(shape BuiltinSimpleFacetValidation, exp BuiltinSimpleFacetExpectation) error {
	if shape.Present != builtinSimpleExpectedFacetMask(exp) || shape.Fixed != 0 {
		return errors.New("builtin simple type facet flags do not match handle")
	}
	if exp.HasFractionDigits && !builtinUnsignedFacetValue(shape.FractionDigits, 0) {
		return errors.New("builtin integer fractionDigits facet does not match handle")
	}
	if exp.HasMinLength && !builtinUnsignedFacetValue(shape.MinLength, exp.MinLength) {
		return errors.New("builtin list minLength facet does not match handle")
	}
	if !builtinDecimalFacetValue(shape.MinInclusive, exp.MinInclusive, exp.Type) ||
		!builtinDecimalFacetValue(shape.MaxInclusive, exp.MaxInclusive, exp.Type) {
		return errors.New("builtin numeric bound facet does not match handle")
	}
	if shape.HasLength ||
		shape.HasMaxLength ||
		shape.HasTotalDigits ||
		shape.HasMinExclusive ||
		shape.HasMaxExclusive ||
		shape.EnumerationSize != 0 ||
		shape.PatternSize != 0 {
		return errors.New("builtin simple type stores unexpected facets")
	}
	return nil
}

func builtinSimpleExpectedFacetMask(exp BuiltinSimpleFacetExpectation) FacetMask {
	var present FacetMask
	if exp.HasFractionDigits {
		present |= FacetFractionDigits
	}
	if exp.MinInclusive != "" {
		present |= FacetMinInclusive
	}
	if exp.MaxInclusive != "" {
		present |= FacetMaxInclusive
	}
	if exp.HasMinLength {
		present |= FacetMinLength
	}
	return present
}

func builtinUnsignedFacetValue(got BuiltinUnsignedFacet, want uint32) bool {
	return got.Present && got.Value == want
}

func builtinDecimalFacetValue(got BuiltinDecimalFacet, want string, compilationType SimpleTypeID) bool {
	if want == "" {
		return !got.Present
	}
	return got.Present &&
		got.ActualValid &&
		got.ActualKind == PrimitiveDecimal &&
		got.Lexical == want &&
		got.Canonical == want &&
		got.Type == compilationType &&
		!got.HasResolvedNames &&
		got.ValueMatchesExpected
}

func builtinSimpleQName(names *NameTable, local string) (QName, bool) {
	if names == nil {
		return QName{}, false
	}
	return names.LookupQName(XSDNamespaceURI, local)
}

func (exp builtinSimpleExpectation) facetExpectation(compilationType SimpleTypeID) BuiltinSimpleFacetExpectation {
	return BuiltinSimpleFacetExpectation{
		Local:             exp.local,
		MinInclusive:      exp.minInclusive,
		MaxInclusive:      exp.maxInclusive,
		Type:              compilationType,
		MinLength:         exp.minLength,
		HasFractionDigits: exp.hasFractionDigits,
		HasMinLength:      exp.hasMinLength,
	}
}

func builtinAttributeQName(names *NameTable, exp builtinAttributeExpectation) (QName, bool) {
	if names == nil {
		return QName{}, false
	}
	return names.LookupQName(exp.ns, exp.local)
}

func builtinAnyTypeQName(names *NameTable) (QName, bool) {
	if names == nil {
		return QName{}, false
	}
	return names.LookupQName(XSDNamespaceURI, vocab.XSDValueAnyType)
}

func builtinAttributeExpectationForSeed(seed BuiltinAttributeSeed, builtins BuiltinIDs) builtinAttributeExpectation {
	exp := builtinAttributeExpectation{
		ns:      seed.Namespace,
		local:   seed.Local,
		handle:  seed.handle,
		builtin: seed.builtin,
	}
	switch seed.handle {
	case builtinSimpleString:
		exp.typ = builtins.String
	case builtinSimpleAnyURI:
		exp.typ = builtins.AnyURI
	case builtinSimpleID:
		exp.typ = builtins.ID
	case builtinSimpleNoHandle,
		builtinSimpleAnySimpleType,
		builtinSimpleBoolean,
		builtinSimpleDecimal,
		builtinSimpleInteger,
		builtinSimpleInt,
		builtinSimpleDate,
		builtinSimpleDateTime,
		builtinSimpleTime,
		builtinSimpleQNameHandle,
		builtinSimpleIDREF,
		builtinSimpleIDREFS,
		builtinSimpleNMTOKEN,
		builtinSimpleNMTOKENS,
		builtinSimpleENTITY,
		builtinSimpleENTITIES:
	}
	return exp
}

var builtinSimpleExpectationTable = [...]builtinSimpleExpectation{
	{local: vocab.XSDValueAnySimpleType, checkID: true, variety: SimpleVarietyAtomic, primitive: PrimitiveString, whitespace: WhitespacePreserve, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueString, checkID: true, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveString, whitespace: WhitespacePreserve, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueNormalized, baseLocal: vocab.XSDValueString, variety: SimpleVarietyAtomic, primitive: PrimitiveString, whitespace: WhitespaceReplace, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueToken, baseLocal: vocab.XSDValueNormalized, variety: SimpleVarietyAtomic, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueLanguage, baseLocal: vocab.XSDValueToken, variety: SimpleVarietyAtomic, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinValidationLanguage, identity: SimpleIdentityNone},
	{local: vocab.XSDValueName, baseLocal: vocab.XSDValueToken, variety: SimpleVarietyAtomic, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinValidationName, identity: SimpleIdentityNone},
	{local: vocab.XSDValueNCName, baseLocal: vocab.XSDValueName, variety: SimpleVarietyAtomic, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNCName, identity: SimpleIdentityNone},
	{local: vocab.XSDValueBoolean, checkID: true, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveBoolean, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueDecimal, checkID: true, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueInteger, checkID: true, baseLocal: vocab.XSDValueDecimal, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true},
	{local: vocab.XSDValueNonPositive, baseLocal: vocab.XSDValueInteger, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true, maxInclusive: "0"},
	{local: vocab.XSDValueNegative, baseLocal: vocab.XSDValueNonPositive, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true, maxInclusive: "-1"},
	{local: vocab.XSDValueNonNegative, baseLocal: vocab.XSDValueInteger, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true, minInclusive: "0"},
	{local: vocab.XSDValuePositive, baseLocal: vocab.XSDValueNonNegative, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true, minInclusive: "1"},
	{local: vocab.XSDValueLong, baseLocal: vocab.XSDValueInteger, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true, minInclusive: "-9223372036854775808", maxInclusive: "9223372036854775807"},
	{local: vocab.XSDValueInt, checkID: true, baseLocal: vocab.XSDValueLong, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true, minInclusive: "-2147483648", maxInclusive: "2147483647"},
	{local: vocab.XSDValueShort, baseLocal: vocab.XSDValueInt, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true, minInclusive: "-32768", maxInclusive: "32767"},
	{local: vocab.XSDValueByte, baseLocal: vocab.XSDValueShort, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true, minInclusive: "-128", maxInclusive: "127"},
	{local: vocab.XSDValueUnsignedLong, baseLocal: vocab.XSDValueNonNegative, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true, minInclusive: "0", maxInclusive: "18446744073709551615"},
	{local: vocab.XSDValueUnsignedInt, baseLocal: vocab.XSDValueUnsignedLong, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true, minInclusive: "0", maxInclusive: "4294967295"},
	{local: vocab.XSDValueUnsignedShort, baseLocal: vocab.XSDValueUnsignedInt, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true, minInclusive: "0", maxInclusive: "65535"},
	{local: vocab.XSDValueUnsignedByte, baseLocal: vocab.XSDValueUnsignedShort, variety: SimpleVarietyAtomic, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinValidationInteger, identity: SimpleIdentityNone, hasFractionDigits: true, minInclusive: "0", maxInclusive: "255"},
	{local: vocab.XSDValueFloat, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveFloat, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueDouble, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveDouble, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueDuration, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveDuration, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueDate, checkID: true, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveDate, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueDateTime, checkID: true, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveDateTime, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueTime, checkID: true, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveTime, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueGYearMonth, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveGYearMonth, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueGYear, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveGYear, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueGMonthDay, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveGMonthDay, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueGDay, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveGDay, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueGMonth, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveGMonth, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueAnyURI, checkID: true, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveAnyURI, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueHexBinary, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveHexBinary, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueBase64Binary, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveBase64Binary, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueQName, checkID: true, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveQName, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueNOTATION, baseLocal: vocab.XSDValueAnySimpleType, variety: SimpleVarietyAtomic, primitive: PrimitiveNotation, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone},
	{local: vocab.XSDValueID, checkID: true, baseLocal: vocab.XSDValueNCName, variety: SimpleVarietyAtomic, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNCName, identity: SimpleIdentityID},
	{local: vocab.XSDValueIDREF, checkID: true, baseLocal: vocab.XSDValueNCName, variety: SimpleVarietyAtomic, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNCName, identity: SimpleIdentityIDREF},
	{local: vocab.XSDValueIDREFS, checkID: true, baseLocal: vocab.XSDValueAnySimpleType, listItemLocal: vocab.XSDValueIDREF, variety: SimpleVarietyList, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityIDREFList, hasMinLength: true, minLength: 1},
	{local: vocab.XSDValueNMTOKEN, checkID: true, baseLocal: vocab.XSDValueToken, variety: SimpleVarietyAtomic, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNMTOKEN, identity: SimpleIdentityNone},
	{local: vocab.XSDValueNMTOKENS, checkID: true, baseLocal: vocab.XSDValueAnySimpleType, listItemLocal: vocab.XSDValueNMTOKEN, variety: SimpleVarietyList, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone, hasMinLength: true, minLength: 1},
	{local: vocab.XSDValueENTITY, checkID: true, baseLocal: vocab.XSDValueNCName, variety: SimpleVarietyAtomic, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinValidationEntity, identity: SimpleIdentityNone},
	{local: vocab.XSDValueENTITIES, checkID: true, baseLocal: vocab.XSDValueAnySimpleType, listItemLocal: vocab.XSDValueENTITY, variety: SimpleVarietyList, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinValidationNone, identity: SimpleIdentityNone, hasMinLength: true, minLength: 1},
}

func builtinSimpleExpectationWithBuiltins(exp builtinSimpleExpectation, builtins BuiltinIDs) builtinSimpleExpectation {
	if !exp.checkID {
		return exp
	}
	switch builtinSimpleHandleForLocal(exp.local) {
	case builtinSimpleAnySimpleType:
		exp.id = builtins.AnySimpleType
	case builtinSimpleString:
		exp.id = builtins.String
	case builtinSimpleBoolean:
		exp.id = builtins.Boolean
	case builtinSimpleDecimal:
		exp.id = builtins.Decimal
	case builtinSimpleInteger:
		exp.id = builtins.Integer
	case builtinSimpleInt:
		exp.id = builtins.Int
	case builtinSimpleDate:
		exp.id = builtins.Date
	case builtinSimpleDateTime:
		exp.id = builtins.DateTime
	case builtinSimpleTime:
		exp.id = builtins.Time
	case builtinSimpleAnyURI:
		exp.id = builtins.AnyURI
	case builtinSimpleQNameHandle:
		exp.id = builtins.QName
	case builtinSimpleID:
		exp.id = builtins.ID
	case builtinSimpleIDREF:
		exp.id = builtins.IDREF
	case builtinSimpleIDREFS:
		exp.id = builtins.IDREFS
	case builtinSimpleNMTOKEN:
		exp.id = builtins.NMTOKEN
	case builtinSimpleNMTOKENS:
		exp.id = builtins.NMTOKENS
	case builtinSimpleENTITY:
		exp.id = builtins.ENTITY
	case builtinSimpleENTITIES:
		exp.id = builtins.ENTITIES
	case builtinSimpleNoHandle:
	}
	return exp
}
