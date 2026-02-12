package num

// ParseError represents a numeric parse failure.
type ParseError struct {
	Kind ParseErrKind
}

// Error is an exported function.
func (e *ParseError) Error() string {
	if e == nil {
		return ""
	}
	return e.Kind.String()
}

// ParseErrKind identifies a parse failure category.
type ParseErrKind uint8

const (
	// ParseInvalid is an exported constant.
	ParseInvalid ParseErrKind = iota
	// ParseEmpty is an exported constant.
	ParseEmpty
	// ParseBadChar is an exported constant.
	ParseBadChar
	// ParseMultipleSigns is an exported constant.
	ParseMultipleSigns
	// ParseMultipleDots is an exported constant.
	ParseMultipleDots
	// ParseNoDigits is an exported constant.
	ParseNoDigits
)

// String is an exported function.
func (k ParseErrKind) String() string {
	switch k {
	case ParseEmpty:
		return "empty"
	case ParseBadChar:
		return "bad character"
	case ParseMultipleSigns:
		return "multiple signs"
	case ParseMultipleDots:
		return "multiple dots"
	case ParseNoDigits:
		return "no digits"
	default:
		return "invalid"
	}
}
