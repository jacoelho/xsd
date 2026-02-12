package value

// TimezoneKind describes whether a lexical value included a timezone.
type TimezoneKind uint8

const (
	// TZNone is an exported constant.
	TZNone TimezoneKind = iota
	// TZKnown is an exported constant.
	TZKnown
)

// TimezoneKindFromLexical reports the timezone kind for a lexical value.
func TimezoneKindFromLexical(lexical []byte) TimezoneKind {
	lexical = TrimXMLWhitespace(lexical)
	if len(lexical) == 0 {
		return TZNone
	}
	last := lexical[len(lexical)-1]
	if last == 'Z' {
		return TZKnown
	}
	if len(lexical) >= 6 {
		tz := lexical[len(lexical)-6:]
		if (tz[0] == '+' || tz[0] == '-') && tz[3] == ':' {
			return TZKnown
		}
	}
	return TZNone
}
