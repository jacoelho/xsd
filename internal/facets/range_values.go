package facets

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
)

// ValidateRangeValues validates range facet literals against base-type lexical space.
func ValidateRangeValues(minExclusive, maxExclusive, minInclusive, maxInclusive *string, baseType model.Type, bt *model.BuiltinType) error {
	var validator model.TypeValidator
	var whiteSpace model.WhiteSpace

	if bt != nil {
		validator = func(value string) error {
			return bt.Validate(value)
		}
		whiteSpace = bt.WhiteSpace()
	} else if baseType != nil {
		switch t := baseType.(type) {
		case *model.BuiltinType:
			validator = func(value string) error {
				return t.Validate(value)
			}
			whiteSpace = t.WhiteSpace()
		case *model.SimpleType:
			if t.IsBuiltin() || t.QName.Namespace == model.XSDNamespace {
				if builtinType := builtins.GetNS(t.QName.Namespace, t.QName.Local); builtinType != nil {
					validator = func(value string) error {
						return builtinType.Validate(value)
					}
					whiteSpace = builtinType.WhiteSpace()
				}
			} else {
				builtinType := findBuiltinAncestor(baseType)
				if builtinType != nil {
					validator = func(value string) error {
						return builtinType.Validate(value)
					}
					whiteSpace = builtinType.WhiteSpace()
				}
			}
		}
	}
	if validator == nil {
		return nil
	}

	normalizeValue := func(val string) string {
		switch whiteSpace {
		case model.WhiteSpaceCollapse:
			return joinFields(model.TrimXMLWhitespace(val))
		case model.WhiteSpaceReplace:
			return strings.Map(func(r rune) rune {
				if r == '\t' || r == '\n' || r == '\r' {
					return ' '
				}
				return r
			}, val)
		default:
			return val
		}
	}

	for _, rangeFacet := range [...]struct {
		value *string
		name  string
	}{
		{name: "minExclusive", value: minExclusive},
		{name: "maxExclusive", value: maxExclusive},
		{name: "minInclusive", value: minInclusive},
		{name: "maxInclusive", value: maxInclusive},
	} {
		if rangeFacet.value == nil {
			continue
		}
		normalized := normalizeValue(*rangeFacet.value)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("%s value %q is not valid for base type: %w", rangeFacet.name, *rangeFacet.value, err)
		}
	}

	return nil
}

func joinFields(value string) string {
	var b strings.Builder
	first := true
	for field := range model.FieldsXMLWhitespaceSeq(value) {
		if !first {
			b.WriteByte(' ')
		}
		first = false
		b.WriteString(field)
	}
	return b.String()
}

func findBuiltinAncestor(t model.Type) *model.BuiltinType {
	visited := make(map[model.Type]bool)
	current := t
	for current != nil && !visited[current] {
		visited[current] = true
		switch ct := current.(type) {
		case *model.BuiltinType:
			return ct
		case *model.SimpleType:
			if ct.IsBuiltin() || ct.QName.Namespace == model.XSDNamespace {
				if bt := builtins.GetNS(ct.QName.Namespace, ct.QName.Local); bt != nil {
					return bt
				}
			}
		}
		current = current.BaseType()
	}
	return nil
}
