package validator

import (
	stderrors "errors"
	"fmt"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

// checkFixedValue validates that element content matches the fixed value constraint.
// Per XSD spec section 3.3.4, fixed values are compared in the value space of the type.
// Both values must be normalized according to the type's whitespace facet before comparison.
func (r *validationRun) checkFixedValue(actualValue, fixedValue string, textType *grammar.CompiledType) []errors.Validation {
	if textType == nil || textType.Original == nil {
		// no type information - compare as strings
		if actualValue != fixedValue {
			return []errors.Validation{errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
				"Element has fixed value '%s' but actual value is '%s'", fixedValue, actualValue)}
		}
		return nil
	}

	// try value space comparison for union types with compiled member types
	if len(textType.MemberTypes) > 0 {
		if match, err := r.compareFixedValueInUnion(actualValue, fixedValue, textType.MemberTypes); err == nil {
			if !match {
				return []errors.Validation{errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
					"Element has fixed value '%s' but actual value is '%s'", fixedValue, actualValue)}
			}
			return nil
		}
	}

	// try value space comparison for union types from original SimpleType
	if st, ok := textType.Original.(*types.SimpleType); ok && st.Variety() == types.UnionVariety {
		memberTypes := r.resolveUnionMemberTypes(st)
		if len(memberTypes) > 0 {
			if match, err := r.compareFixedValueInUnionTypes(actualValue, fixedValue, memberTypes); err == nil {
				if !match {
					return []errors.Validation{errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
						"Element has fixed value '%s' but actual value is '%s'", fixedValue, actualValue)}
				}
				return nil
			}
		}
	}

	// try value space comparison for simple types
	if st, ok := valueSpaceType(textType.Original); ok {
		if match, err := r.compareFixedValueAsType(actualValue, fixedValue, st); err == nil {
			if !match {
				return []errors.Validation{errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
					"Element has fixed value '%s' but actual value is '%s'", fixedValue, actualValue)}
			}
			return nil
		}
	}

	// fall back to normalized string comparison
	if !fixedValueMatches(actualValue, fixedValue, textType.Original) {
		return []errors.Validation{errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
			"Element has fixed value '%s' but actual value is '%s'", fixedValue, actualValue)}
	}
	return nil
}

// resolveUnionMemberTypes resolves all member types from a union SimpleType.
func (r *validationRun) resolveUnionMemberTypes(st *types.SimpleType) []types.Type {
	if st.Union == nil {
		return st.MemberTypes
	}

	// use pre-resolved member types if available
	if len(st.MemberTypes) > 0 {
		return st.MemberTypes
	}

	var memberTypes []types.Type
	for _, inline := range st.Union.InlineTypes {
		memberTypes = append(memberTypes, inline)
	}
	for _, qname := range st.Union.MemberTypes {
		if member := r.schema.Type(qname); member != nil {
			memberTypes = append(memberTypes, member.Original)
		} else if bt := types.GetBuiltinNS(qname.Namespace, qname.Local); bt != nil {
			memberTypes = append(memberTypes, bt)
		}
	}
	return memberTypes
}

// compareFixedValueInUnion compares values using compiled union member types.
func (r *validationRun) compareFixedValueInUnion(actualValue, fixedValue string, memberTypes []*grammar.CompiledType) (bool, error) {
	actualTyped, actualType, err := r.parseUnionValue(actualValue, memberTypes)
	if err != nil {
		return false, err
	}
	fixedTyped, fixedType, err := r.parseUnionValue(fixedValue, memberTypes)
	if err != nil {
		return false, err
	}
	if actualType != fixedType {
		return false, nil
	}
	return compareTypedValues(actualTyped, fixedTyped), nil
}

// compareFixedValueInUnionTypes compares values using resolved union member types.
func (r *validationRun) compareFixedValueInUnionTypes(actualValue, fixedValue string, memberTypes []types.Type) (bool, error) {
	actualTyped, actualType, err := r.parseUnionValueTypes(actualValue, memberTypes)
	if err != nil {
		return false, err
	}
	fixedTyped, fixedType, err := r.parseUnionValueTypes(fixedValue, memberTypes)
	if err != nil {
		return false, err
	}
	if actualType != fixedType {
		return false, nil
	}
	return compareTypedValues(actualTyped, fixedTyped), nil
}

// compareFixedValueAsType compares two values in the value space of a type.
// Returns (equal, nil) if both values can be parsed, or (false, error) if parsing fails.
func (r *validationRun) compareFixedValueAsType(actualValue, fixedValue string, typ types.Type) (bool, error) {
	actualTyped, actualErr := r.parseValueAsType(actualValue, typ)
	if actualErr != nil {
		return false, actualErr
	}

	fixedTyped, fixedErr := r.parseValueAsType(fixedValue, typ)
	if fixedErr != nil {
		return false, fixedErr
	}

	return compareTypedValues(actualTyped, fixedTyped), nil
}

// errNoMatchingMemberType indicates no union member type could parse the value
var errNoMatchingMemberType = stderrors.New("no matching member type")

func (r *validationRun) parseUnionValue(value string, memberTypes []*grammar.CompiledType) (types.TypedValue, *grammar.CompiledType, error) {
	for _, member := range memberTypes {
		if member == nil || member.Original == nil {
			continue
		}
		memberType, ok := valueSpaceType(member.Original)
		if !ok {
			continue
		}
		typedValue, err := r.parseValueAsType(value, memberType)
		if err == nil {
			return typedValue, member, nil
		}
	}
	return nil, nil, errNoMatchingMemberType
}

func (r *validationRun) parseUnionValueTypes(value string, memberTypes []types.Type) (types.TypedValue, types.Type, error) {
	for _, member := range memberTypes {
		if member == nil {
			continue
		}
		memberType, ok := valueSpaceType(member)
		if !ok {
			continue
		}
		typedValue, err := r.parseValueAsType(value, memberType)
		if err == nil {
			return typedValue, member, nil
		}
	}
	return nil, nil, errNoMatchingMemberType
}

func (r *validationRun) parseValueAsType(value string, typ types.Type) (types.TypedValue, error) {
	switch t := typ.(type) {
	case *types.SimpleType:
		if t.IsBuiltin() {
			value = types.ApplyWhiteSpace(value, t.WhiteSpace())
		}
		return t.ParseValue(value)
	case *types.BuiltinType:
		value = types.ApplyWhiteSpace(value, t.WhiteSpace())
		return t.ParseValue(value)
	default:
		return nil, fmt.Errorf("cannot parse value for type %T", typ)
	}
}

func valueSpaceType(typ types.Type) (types.Type, bool) {
	switch typ.(type) {
	case *types.SimpleType, *types.BuiltinType:
		return typ, true
	default:
		return nil, false
	}
}
