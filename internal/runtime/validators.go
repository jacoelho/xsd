package runtime

// ValidatorKind defines an exported type.
type ValidatorKind uint8

const (
	// VString is an exported constant.
	VString ValidatorKind = iota
	// VBoolean is an exported constant.
	VBoolean
	// VDecimal is an exported constant.
	VDecimal
	// VInteger is an exported constant.
	VInteger
	// VFloat is an exported constant.
	VFloat
	// VDouble is an exported constant.
	VDouble
	// VDuration is an exported constant.
	VDuration
	// VDateTime is an exported constant.
	VDateTime
	// VTime is an exported constant.
	VTime
	// VDate is an exported constant.
	VDate
	// VGYearMonth is an exported constant.
	VGYearMonth
	// VGYear is an exported constant.
	VGYear
	// VGMonthDay is an exported constant.
	VGMonthDay
	// VGDay is an exported constant.
	VGDay
	// VGMonth is an exported constant.
	VGMonth
	// VAnyURI is an exported constant.
	VAnyURI
	// VQName is an exported constant.
	VQName
	// VNotation is an exported constant.
	VNotation
	// VHexBinary is an exported constant.
	VHexBinary
	// VBase64Binary is an exported constant.
	VBase64Binary
	// VList is an exported constant.
	VList
	// VUnion is an exported constant.
	VUnion
)

// ValidatorsBundle defines an exported type.
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

// ValidatorMeta defines an exported type.
type ValidatorMeta struct {
	Facets     FacetProgramRef
	Index      uint32
	Kind       ValidatorKind
	WhiteSpace WhitespaceMode
	Flags      ValidatorFlags
}

// ValidatorFlags defines an exported type.
type ValidatorFlags uint8

const (
	// ValidatorHasEnum is an exported constant.
	ValidatorHasEnum ValidatorFlags = 1 << iota
)

// WhitespaceMode defines an exported type.
type WhitespaceMode uint8

const (
	// WSPreserve is an exported constant.
	WSPreserve WhitespaceMode = iota
	// WSReplace is an exported constant.
	WSReplace
	// WSCollapse is an exported constant.
	WSCollapse
)

// FacetProgramRef defines an exported type.
type FacetProgramRef struct {
	Off uint32
	Len uint32
}

// FacetOp defines an exported type.
type FacetOp uint8

const (
	// FPattern is an exported constant.
	FPattern FacetOp = iota
	// FEnum is an exported constant.
	FEnum
	// FMinInclusive is an exported constant.
	FMinInclusive
	// FMaxInclusive is an exported constant.
	FMaxInclusive
	// FMinExclusive is an exported constant.
	FMinExclusive
	// FMaxExclusive is an exported constant.
	FMaxExclusive
	// FMinLength is an exported constant.
	FMinLength
	// FMaxLength is an exported constant.
	FMaxLength
	// FLength is an exported constant.
	FLength
	// FTotalDigits is an exported constant.
	FTotalDigits
	// FFractionDigits is an exported constant.
	FFractionDigits
)

// FacetInstr defines an exported type.
type FacetInstr struct {
	Op   FacetOp
	Arg0 uint32
	Arg1 uint32
}

// FacetProgram defines an exported type.
type FacetProgram struct {
	Code []FacetInstr
}

// StringKind defines an exported type.
type StringKind uint8

const (
	// StringAny is an exported constant.
	StringAny StringKind = iota
	// StringNormalized is an exported constant.
	StringNormalized
	// StringToken is an exported constant.
	StringToken
	// StringLanguage is an exported constant.
	StringLanguage
	// StringName is an exported constant.
	StringName
	// StringNCName is an exported constant.
	StringNCName
	// StringID is an exported constant.
	StringID
	// StringIDREF is an exported constant.
	StringIDREF
	// StringEntity is an exported constant.
	StringEntity
	// StringNMTOKEN is an exported constant.
	StringNMTOKEN
)

// StringValidator defines an exported type.
type StringValidator struct {
	Kind StringKind
}

// BooleanValidator defines an exported type.
type BooleanValidator struct{}

// DecimalValidator defines an exported type.
type DecimalValidator struct{}

// IntegerKind defines an exported type.
type IntegerKind uint8

const (
	// IntegerAny is an exported constant.
	IntegerAny IntegerKind = iota
	// IntegerLong is an exported constant.
	IntegerLong
	// IntegerInt is an exported constant.
	IntegerInt
	// IntegerShort is an exported constant.
	IntegerShort
	// IntegerByte is an exported constant.
	IntegerByte
	// IntegerNonNegative is an exported constant.
	IntegerNonNegative
	// IntegerPositive is an exported constant.
	IntegerPositive
	// IntegerNonPositive is an exported constant.
	IntegerNonPositive
	// IntegerNegative is an exported constant.
	IntegerNegative
	// IntegerUnsignedLong is an exported constant.
	IntegerUnsignedLong
	// IntegerUnsignedInt is an exported constant.
	IntegerUnsignedInt
	// IntegerUnsignedShort is an exported constant.
	IntegerUnsignedShort
	// IntegerUnsignedByte is an exported constant.
	IntegerUnsignedByte
)

// IntegerValidator defines an exported type.
type IntegerValidator struct {
	Kind IntegerKind
}

// FloatValidator defines an exported type.
type FloatValidator struct{}

// DoubleValidator defines an exported type.
type DoubleValidator struct{}

// DurationValidator defines an exported type.
type DurationValidator struct{}

// DateTimeValidator defines an exported type.
type DateTimeValidator struct{}

// TimeValidator defines an exported type.
type TimeValidator struct{}

// DateValidator defines an exported type.
type DateValidator struct{}

// GYearMonthValidator defines an exported type.
type GYearMonthValidator struct{}

// GYearValidator defines an exported type.
type GYearValidator struct{}

// GMonthDayValidator defines an exported type.
type GMonthDayValidator struct{}

// GDayValidator defines an exported type.
type GDayValidator struct{}

// GMonthValidator defines an exported type.
type GMonthValidator struct{}

// AnyURIValidator defines an exported type.
type AnyURIValidator struct{}

// QNameValidator defines an exported type.
type QNameValidator struct{}

// NotationValidator defines an exported type.
type NotationValidator struct{}

// HexBinaryValidator defines an exported type.
type HexBinaryValidator struct{}

// Base64BinaryValidator defines an exported type.
type Base64BinaryValidator struct{}

// ListValidator defines an exported type.
type ListValidator struct {
	Item ValidatorID
}

// UnionValidator defines an exported type.
type UnionValidator struct {
	MemberOff uint32
	MemberLen uint32
}
