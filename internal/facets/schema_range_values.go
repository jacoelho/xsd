package facets

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/types"
)

// ValidateRangeValues validates range facet literals against base-type lexical space.
func ValidateRangeValues(minExclusive, maxExclusive, minInclusive, maxInclusive *string, baseType types.Type, bt *types.BuiltinType) error {
	var validator types.TypeValidator
	var whiteSpace types.WhiteSpace

	if bt != nil {
		validator = func(value string) error {
			return bt.Validate(value)
		}
		whiteSpace = bt.WhiteSpace()
	} else if baseType != nil {
		switch t := baseType.(type) {
		case *types.BuiltinType:
			validator = func(value string) error {
				return t.Validate(value)
			}
			whiteSpace = t.WhiteSpace()
		case *types.SimpleType:
			if t.IsBuiltin() || t.QName.Namespace == types.XSDNamespace {
				if builtinType := types.GetBuiltinNS(t.QName.Namespace, t.QName.Local); builtinType != nil {
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
		case types.WhiteSpaceCollapse:
			return joinFields(types.TrimXMLWhitespace(val))
		case types.WhiteSpaceReplace:
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

	if minExclusive != nil {
		normalized := normalizeValue(*minExclusive)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("minExclusive value %q is not valid for base type: %w", *minExclusive, err)
		}
	}
	if maxExclusive != nil {
		normalized := normalizeValue(*maxExclusive)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("maxExclusive value %q is not valid for base type: %w", *maxExclusive, err)
		}
	}
	if minInclusive != nil {
		normalized := normalizeValue(*minInclusive)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("minInclusive value %q is not valid for base type: %w", *minInclusive, err)
		}
	}
	if maxInclusive != nil {
		normalized := normalizeValue(*maxInclusive)
		if err := validator(normalized); err != nil {
			return fmt.Errorf("maxInclusive value %q is not valid for base type: %w", *maxInclusive, err)
		}
	}

	return nil
}

func joinFields(value string) string {
	var b strings.Builder
	first := true
	for field := range types.FieldsXMLWhitespaceSeq(value) {
		if !first {
			b.WriteByte(' ')
		}
		first = false
		b.WriteString(field)
	}
	return b.String()
}

func findBuiltinAncestor(t types.Type) *types.BuiltinType {
	visited := make(map[types.Type]bool)
	current := t
	for current != nil && !visited[current] {
		visited[current] = true
		switch ct := current.(type) {
		case *types.BuiltinType:
			return ct
		case *types.SimpleType:
			if ct.IsBuiltin() || ct.QName.Namespace == types.XSDNamespace {
				if bt := types.GetBuiltinNS(ct.QName.Namespace, ct.QName.Local); bt != nil {
					return bt
				}
			}
		}
		current = current.BaseType()
	}
	return nil
}
