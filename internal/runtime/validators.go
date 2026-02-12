package runtime

// ValidatorKind enumerates validator kind values.
type ValidatorKind uint8

const (
	VString ValidatorKind = iota
	VBoolean
	VDecimal
	VInteger
	VFloat
	VDouble
	VDuration
	VDateTime
	VTime
	VDate
	VGYearMonth
	VGYear
	VGMonthDay
	VGDay
	VGMonth
	VAnyURI
	VQName
	VNotation
	VHexBinary
	VBase64Binary
	VList
	VUnion
)

// ValidatorsBundle groups all validator tables and per-type metadata.
type ValidatorsBundle struct {
	String       []StringValidator
	Boolean      []BooleanValidator
	Decimal      []DecimalValidator
	Integer      []IntegerValidator
	Float        []FloatValidator
	Double       []DoubleValidator
	Duration     []DurationValidator
	DateTime     []DateTimeValidator
	Time         []TimeValidator
	Date         []DateValidator
	GYearMonth   []GYearMonthValidator
	GYear        []GYearValidator
	GMonthDay    []GMonthDayValidator
	GDay         []GDayValidator
	GMonth       []GMonthValidator
	AnyURI       []AnyURIValidator
	QName        []QNameValidator
	Notation     []NotationValidator
	HexBinary    []HexBinaryValidator
	Base64Binary []Base64BinaryValidator
	List         []ListValidator
	Union        []UnionValidator

	UnionMembers      []ValidatorID
	UnionMemberTypes  []TypeID
	UnionMemberSameWS []uint8
	Meta              []ValidatorMeta
}

// ValidatorMeta stores kind-specific metadata for one validator entry.
type ValidatorMeta struct {
	Facets     FacetProgramRef
	Index      uint32
	Kind       ValidatorKind
	WhiteSpace WhitespaceMode
	Flags      ValidatorFlags
}

// ValidatorFlags is a bitset for validator flags options.
type ValidatorFlags uint8

const (
	ValidatorHasEnum ValidatorFlags = 1 << iota
)

// WhitespaceMode enumerates whitespace mode values.
type WhitespaceMode uint8

const (
	WSPreserve WhitespaceMode = iota
	WSReplace
	WSCollapse
)

// FacetProgramRef references facet program ref data in packed tables.
type FacetProgramRef struct {
	Off uint32
	Len uint32
}

// FacetOp enumerates facet bytecode operations.
type FacetOp uint8

const (
	FPattern FacetOp = iota
	FEnum
	FMinInclusive
	FMaxInclusive
	FMinExclusive
	FMaxExclusive
	FMinLength
	FMaxLength
	FLength
	FTotalDigits
	FFractionDigits
)

// FacetInstr stores one facet bytecode instruction.
type FacetInstr struct {
	Op   FacetOp
	Arg0 uint32
	Arg1 uint32
}

// FacetProgram stores a facet bytecode stream.
type FacetProgram struct {
	Code []FacetInstr
}

// StringKind enumerates string kind values.
type StringKind uint8

const (
	StringAny StringKind = iota
	StringNormalized
	StringToken
	StringLanguage
	StringName
	StringNCName
	StringID
	StringIDREF
	StringEntity
	StringNMTOKEN
)

// StringValidator stores the string-kind discriminator for string-family validation.
type StringValidator struct {
	Kind StringKind
}

// BooleanValidator marks the boolean validator kind.
type BooleanValidator struct{}

// DecimalValidator marks the decimal validator kind.
type DecimalValidator struct{}

// IntegerKind enumerates integer kind values.
type IntegerKind uint8

const (
	IntegerAny IntegerKind = iota
	IntegerLong
	IntegerInt
	IntegerShort
	IntegerByte
	IntegerNonNegative
	IntegerPositive
	IntegerNonPositive
	IntegerNegative
	IntegerUnsignedLong
	IntegerUnsignedInt
	IntegerUnsignedShort
	IntegerUnsignedByte
)

// IntegerValidator stores the integer-kind discriminator for integer-family validation.
type IntegerValidator struct {
	Kind IntegerKind
}

// FloatValidator marks the float validator kind.
type FloatValidator struct{}

// DoubleValidator marks the double validator kind.
type DoubleValidator struct{}

// DurationValidator marks the duration validator kind.
type DurationValidator struct{}

// DateTimeValidator marks the dateTime validator kind.
type DateTimeValidator struct{}

// TimeValidator marks the time validator kind.
type TimeValidator struct{}

// DateValidator marks the date validator kind.
type DateValidator struct{}

// GYearMonthValidator marks the gYearMonth validator kind.
type GYearMonthValidator struct{}

// GYearValidator marks the gYear validator kind.
type GYearValidator struct{}

// GMonthDayValidator marks the gMonthDay validator kind.
type GMonthDayValidator struct{}

// GDayValidator marks the gDay validator kind.
type GDayValidator struct{}

// GMonthValidator marks the gMonth validator kind.
type GMonthValidator struct{}

// AnyURIValidator marks the anyURI validator kind.
type AnyURIValidator struct{}

// QNameValidator marks the QName validator kind.
type QNameValidator struct{}

// NotationValidator marks the NOTATION validator kind.
type NotationValidator struct{}

// HexBinaryValidator marks the hexBinary validator kind.
type HexBinaryValidator struct{}

// Base64BinaryValidator marks the base64Binary validator kind.
type Base64BinaryValidator struct{}

// ListValidator stores the validator used for list item members.
type ListValidator struct {
	Item ValidatorID
}

// UnionValidator stores the member span for union validation.
type UnionValidator struct {
	MemberOff uint32
	MemberLen uint32
}
