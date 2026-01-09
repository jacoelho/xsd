package types

import (
	"fmt"
	"slices"
)

// Enumeration represents an enumeration facet
type Enumeration struct {
	Values []string
}

// Name returns the facet name
func (e *Enumeration) Name() string {
	return "enumeration"
}

// Validate checks if the value is in the enumeration (unified Facet interface)
func (e *Enumeration) Validate(value TypedValue, baseType Type) error {
	lexical := value.Lexical()
	if isDateTimeType(baseType) {
		for _, allowed := range e.Values {
			if dateTimeValuesEqual(lexical, allowed, baseType) {
				return nil
			}
		}
		return fmt.Errorf("value %s not in enumeration: %v", lexical, e.Values)
	}
	if slices.Contains(e.Values, lexical) {
		return nil
	}
	return fmt.Errorf("value %s not in enumeration: %v", lexical, e.Values)
}

func isDateTimeType(baseType Type) bool {
	if baseType == nil {
		return false
	}
	primitive := baseType.PrimitiveType()
	if primitive == nil {
		primitive = baseType
	}
	name := primitive.Name().Local
	return name == "dateTime" || name == "date" || name == "time"
}

func dateTimeValuesEqual(v1, v2 string, baseType Type) bool {
	primitive := baseType.PrimitiveType()
	if primitive == nil {
		primitive = baseType
	}
	switch primitive.Name().Local {
	case "dateTime":
		t1, err1 := ParseDateTime(v1)
		t2, err2 := ParseDateTime(v2)
		if err1 != nil || err2 != nil {
			return v1 == v2
		}
		return t1.Equal(t2)
	case "date":
		t1, err1 := ParseDate(v1)
		t2, err2 := ParseDate(v2)
		if err1 != nil || err2 != nil {
			return v1 == v2
		}
		return t1.Equal(t2)
	case "time":
		t1, err1 := ParseTime(v1)
		t2, err2 := ParseTime(v2)
		if err1 != nil || err2 != nil {
			return v1 == v2
		}
		return t1.Equal(t2)
	default:
		return v1 == v2
	}
}
