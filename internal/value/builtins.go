package value

// The builtin table is fixed XSD metadata. It is independent of any compiler
// or runtime allocation, so every program observes the same IDs and derived
// chains without copying builtin records into its user table.
const (
	// AnySimpleTypeID is the fixed ID of the generic XSD simple-type base.
	AnySimpleTypeID TypeID = iota
	builtinString
	builtinNormalizedString
	builtinToken
	builtinLanguage
	builtinName
	builtinNCName
	builtinBoolean
	builtinDecimal
	builtinInteger
	builtinNonPositiveInteger
	builtinNegativeInteger
	builtinNonNegativeInteger
	builtinPositiveInteger
	builtinLong
	builtinInt
	builtinShort
	builtinByte
	builtinUnsignedLong
	builtinUnsignedInt
	builtinUnsignedShort
	builtinUnsignedByte
	builtinFloat
	builtinDouble
	builtinDuration
	builtinDate
	builtinDateTime
	builtinTime
	builtinGYearMonth
	builtinGYear
	builtinGMonthDay
	builtinGDay
	builtinGMonth
	builtinAnyURI
	builtinHexBinary
	builtinBase64Binary
	builtinQName
	builtinNotation
	builtinID
	builtinIDREF
	builtinIDREFS
	builtinNMTOKEN
	builtinNMTOKENS
	builtinENTITY
	builtinENTITIES
)

// BuiltinTypeCount is the fixed number of XSD simple types.
const BuiltinTypeCount = builtinENTITIES + 1

type builtinMeta struct {
	name       string
	min        string
	max        string
	base       TypeID
	listItem   TypeID
	minLength  uint32
	variety    Variety
	primitive  PrimitiveKind
	whitespace WhitespaceMode
	builtin    BuiltinKind
	carriers   identityCarriers
	fraction   bool
}

// Canonical primitive records precede their derived records. BuiltinType
// skips anySimpleType because its string value space is not xs:string.
var builtinMetadata = [...]builtinMeta{
	{name: "anySimpleType", base: NoType, primitive: PrimitiveString, whitespace: WhitespacePreserve},
	{name: "string", base: AnySimpleTypeID, primitive: PrimitiveString, whitespace: WhitespacePreserve},
	{name: "normalizedString", base: builtinString, primitive: PrimitiveString, whitespace: WhitespaceReplace},
	{name: "token", base: builtinNormalizedString, primitive: PrimitiveString, whitespace: WhitespaceCollapse},
	{name: "language", base: builtinToken, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinLanguage},
	{name: "Name", base: builtinToken, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinName},
	{name: "NCName", base: builtinName, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinNCName},
	{name: "boolean", base: AnySimpleTypeID, primitive: PrimitiveBoolean, whitespace: WhitespaceCollapse},
	{name: "decimal", base: AnySimpleTypeID, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse},
	{name: "integer", base: builtinDecimal, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinInteger, fraction: true},
	{name: "nonPositiveInteger", base: builtinInteger, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinInteger, max: "0", fraction: true},
	{name: "negativeInteger", base: builtinNonPositiveInteger, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinInteger, max: "-1", fraction: true},
	{name: "nonNegativeInteger", base: builtinInteger, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, builtin: BuiltinInteger, min: "0", fraction: true},
	{name: "positiveInteger", base: builtinNonNegativeInteger, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, min: "1", fraction: true, builtin: BuiltinInteger},
	{name: "long", base: builtinInteger, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, min: "-9223372036854775808", max: "9223372036854775807", fraction: true, builtin: BuiltinInteger},
	{name: "int", base: builtinLong, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, min: "-2147483648", max: "2147483647", fraction: true, builtin: BuiltinInteger},
	{name: "short", base: builtinInt, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, min: "-32768", max: "32767", fraction: true, builtin: BuiltinInteger},
	{name: "byte", base: builtinShort, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, min: "-128", max: "127", fraction: true, builtin: BuiltinInteger},
	{name: "unsignedLong", base: builtinNonNegativeInteger, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, min: "0", max: "18446744073709551615", fraction: true, builtin: BuiltinInteger},
	{name: "unsignedInt", base: builtinUnsignedLong, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, min: "0", max: "4294967295", fraction: true, builtin: BuiltinInteger},
	{name: "unsignedShort", base: builtinUnsignedInt, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, min: "0", max: "65535", fraction: true, builtin: BuiltinInteger},
	{name: "unsignedByte", base: builtinUnsignedShort, primitive: PrimitiveDecimal, whitespace: WhitespaceCollapse, min: "0", max: "255", fraction: true, builtin: BuiltinInteger},
	{name: "float", base: AnySimpleTypeID, primitive: PrimitiveFloat, whitespace: WhitespaceCollapse},
	{name: "double", base: AnySimpleTypeID, primitive: PrimitiveDouble, whitespace: WhitespaceCollapse},
	{name: "duration", base: AnySimpleTypeID, primitive: PrimitiveDuration, whitespace: WhitespaceCollapse},
	{name: dateTypeName, base: AnySimpleTypeID, primitive: PrimitiveDate, whitespace: WhitespaceCollapse},
	{name: "dateTime", base: AnySimpleTypeID, primitive: PrimitiveDateTime, whitespace: WhitespaceCollapse},
	{name: "time", base: AnySimpleTypeID, primitive: PrimitiveTime, whitespace: WhitespaceCollapse},
	{name: "gYearMonth", base: AnySimpleTypeID, primitive: PrimitiveGYearMonth, whitespace: WhitespaceCollapse},
	{name: "gYear", base: AnySimpleTypeID, primitive: PrimitiveGYear, whitespace: WhitespaceCollapse},
	{name: "gMonthDay", base: AnySimpleTypeID, primitive: PrimitiveGMonthDay, whitespace: WhitespaceCollapse},
	{name: "gDay", base: AnySimpleTypeID, primitive: PrimitiveGDay, whitespace: WhitespaceCollapse},
	{name: "gMonth", base: AnySimpleTypeID, primitive: PrimitiveGMonth, whitespace: WhitespaceCollapse},
	{name: "anyURI", base: AnySimpleTypeID, primitive: PrimitiveAnyURI, whitespace: WhitespaceCollapse},
	{name: "hexBinary", base: AnySimpleTypeID, primitive: PrimitiveHexBinary, whitespace: WhitespaceCollapse},
	{name: "base64Binary", base: AnySimpleTypeID, primitive: PrimitiveBase64Binary, whitespace: WhitespaceCollapse},
	{name: "QName", base: AnySimpleTypeID, primitive: PrimitiveQName, whitespace: WhitespaceCollapse},
	{name: "NOTATION", base: AnySimpleTypeID, primitive: PrimitiveNotation, whitespace: WhitespaceCollapse},
	{name: "ID", base: builtinNCName, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinNCName, carriers: carriesID},
	{name: "IDREF", base: builtinNCName, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinNCName, carriers: carriesIDREF},
	{name: "IDREFS", base: AnySimpleTypeID, listItem: builtinIDREF, variety: List, primitive: PrimitiveString, whitespace: WhitespaceCollapse, carriers: carriesIDREF, minLength: 1},
	{name: "NMTOKEN", base: builtinToken, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinNMTOKEN},
	{name: "NMTOKENS", base: AnySimpleTypeID, listItem: builtinNMTOKEN, variety: List, primitive: PrimitiveString, whitespace: WhitespaceCollapse, minLength: 1},
	{name: "ENTITY", base: builtinNCName, primitive: PrimitiveString, whitespace: WhitespaceCollapse, builtin: BuiltinEntity},
	{name: "ENTITIES", base: AnySimpleTypeID, listItem: builtinENTITY, variety: List, primitive: PrimitiveString, whitespace: WhitespaceCollapse, minLength: 1},
}

var builtinDefinitions = buildBuiltinDefinitions()

func buildBuiltinDefinitions() [BuiltinTypeCount]typeDef {
	var definitions [BuiltinTypeCount]typeDef
	for id, m := range builtinMetadata {
		definitions[id] = typeDef{
			variety:         m.variety,
			primitive:       m.primitive,
			whitespace:      m.whitespace,
			builtin:         m.builtin,
			carriers:        m.carriers,
			needsQName:      m.primitive == PrimitiveQName || m.primitive == PrimitiveNotation,
			needsQNameKnown: true,
			base:            m.base,
			listItem:        NoType,
			facets:          builtinFacetProgram(m),
		}
		if m.variety == List {
			definitions[id].listItem = m.listItem
		}
		prepareRawDecimalFastPath(&definitions[id])
		prepareRawIntegerFastPath(&definitions[id])
	}
	prepareRawBuiltinValuePlans(&definitions)
	return definitions
}

func builtinTypeDef(id TypeID) (*typeDef, bool) {
	if id >= BuiltinTypeCount {
		return nil, false
	}
	return &builtinDefinitions[id], true
}

func builtinFacetProgram(m builtinMeta) facetProgram {
	f := facetProgram{}
	if m.fraction {
		f.present |= FacetFractionDigits
		f.fractionDigits = CardinalityFacet{Present: true}
	}
	if m.minLength != 0 {
		f.present |= FacetMinLength
		f.minLength = CardinalityFacet{Value: m.minLength, Present: true}
	}
	if m.min != "" {
		if d, err := ParseDecimalCanonical(m.min); err == nil {
			f.present |= FacetMinInclusive
			f.lower = append(f.lower, boundValue{value: atomicValue{kind: PrimitiveDecimal, decimal: d}})
		}
	}
	if m.max != "" {
		if d, err := ParseDecimalCanonical(m.max); err == nil {
			f.present |= FacetMaxInclusive
			f.upper = append(f.upper, boundValue{value: atomicValue{kind: PrimitiveDecimal, decimal: d}})
		}
	}
	return f
}

// BuiltinType returns the canonical builtin ID for a primitive family.
func BuiltinType(kind PrimitiveKind) TypeID {
	for id := builtinString; id < BuiltinTypeCount; id++ {
		if builtinMetadata[id].primitive == kind {
			return id
		}
	}
	return NoType
}

// BuiltinTypeID returns a fixed builtin ID by its XSD local name.
func BuiltinTypeID(local string) (TypeID, bool) {
	for id, meta := range builtinMetadata {
		if meta.name == local {
			return TypeID(id), true
		}
	}
	return NoType, false
}

// BuiltinTypeName returns the fixed local name for id.
func BuiltinTypeName(id TypeID) (string, bool) {
	if id >= BuiltinTypeCount {
		return "", false
	}
	return builtinMetadata[id].name, true
}
