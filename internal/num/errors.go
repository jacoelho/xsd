package num

// ParseError represents a numeric parse failure.
type ParseError struct {
	Kind ParseErrKind
}

func (e *ParseError) Error() string {
	if e == nil {
		return ""
	}
	return e.Kind.String()
}

// ParseErrKind identifies a parse failure category.
type ParseErrKind uint8

const (
	ParseInvalid ParseErrKind = iota
	ParseEmpty
	ParseBadChar
	ParseMultipleSigns
	ParseMultipleDots
	ParseNoDigits
)

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
