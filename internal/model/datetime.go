package model

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/value"
)

// TimezoneKind reports the timezone kind for a lexical date/time value.
func TimezoneKind(lexical string) value.TimezoneKind {
	lexical = TrimXMLWhitespace(lexical)
	return value.TimezoneKindFromLexical([]byte(lexical))
}

// HasTimezone reports whether a lexical date/time value includes a timezone indicator.
func HasTimezone(lexical string) bool {
	return TimezoneKind(lexical) != value.TZNone
}

// ParseDuration validates an XSD duration string and returns the lexical value.
// It does not map to time.Duration.
func ParseDuration(lexical string) (string, error) {
	lexical = TrimXMLWhitespace(lexical)
	if lexical == "" {
		return "", fmt.Errorf("invalid duration: empty string")
	}

	if _, err := durationlex.Parse(lexical); err != nil {
		return "", err
	}

	return lexical, nil
}
