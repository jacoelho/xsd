package runtime

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

	UnionMembers []ValidatorID
	Meta         []ValidatorMeta
}

type ValidatorMeta struct {
	Facets     FacetProgramRef
	Index      uint32
	Kind       ValidatorKind
	WhiteSpace WhitespaceMode
	Flags      ValidatorFlags
}

type ValidatorFlags uint8

const (
	ValidatorHasEnum ValidatorFlags = 1 << iota
)

type WhitespaceMode uint8

const (
	WS_Preserve WhitespaceMode = iota
	WS_Replace
	WS_Collapse
)

type FacetProgramRef struct {
	Off uint32
	Len uint32
}

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

type FacetInstr struct {
	Op   FacetOp
	Arg0 uint32
	Arg1 uint32
}

type FacetProgram struct {
	Code []FacetInstr
}

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

type StringValidator struct {
	Kind StringKind
}

type BooleanValidator struct{}

type DecimalValidator struct{}

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

type IntegerValidator struct {
	Kind IntegerKind
}

type FloatValidator struct{}

type DoubleValidator struct{}

type DurationValidator struct{}

type DateTimeValidator struct{}

type TimeValidator struct{}

type DateValidator struct{}

type GYearMonthValidator struct{}

type GYearValidator struct{}

type GMonthDayValidator struct{}

type GDayValidator struct{}

type GMonthValidator struct{}

type AnyURIValidator struct{}

type QNameValidator struct{}

type NotationValidator struct{}

type HexBinaryValidator struct{}

type Base64BinaryValidator struct{}

type ListValidator struct {
	Item ValidatorID
}

type UnionValidator struct {
	MemberOff uint32
	MemberLen uint32
}
