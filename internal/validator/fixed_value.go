package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

// checkFixedValue validates that element content matches the fixed value constraint.
// Per XSD spec section 3.3.4, fixed values are compared in the value space of the type.
// Both values must be normalized according to the type's whitespace facet before comparison.
func (r *streamRun) checkFixedValue(actualValue, fixedValue string, textType *grammar.CompiledType, scopeDepth int, fixedContext map[string]string) []errors.Validation {
	if r.fixedValueEqualWithContext(actualValue, fixedValue, textType, scopeDepth, fixedContext) {
		return nil
	}
	return []errors.Validation{errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
		"Element has fixed value '%s' but actual value is '%s'", fixedValue, actualValue)}
}

func (r *streamRun) fixedValueEqualWithContext(actualValue, fixedValue string, textType *grammar.CompiledType, scopeDepth int, fixedContext map[string]string) bool {
	if textType == nil || textType.Original == nil {
		return actualValue == fixedValue
	}

	if textType.IsQNameOrNotationType {
		if match, err := r.compareFixedQNameValue(actualValue, fixedValue, textType.Original, scopeDepth, fixedContext); err == nil {
			return match
		}
		return false
	}

	if textType.ItemType != nil && textType.ItemType.IsQNameOrNotationType {
		if match, err := r.compareFixedValueListWithContext(actualValue, fixedValue, textType, scopeDepth, fixedContext); err == nil {
			return match
		}
		return false
	}

	// try value space comparison for union types with compiled member types
	if len(textType.MemberTypes) > 0 {
		if match, err := r.compareFixedValueInUnionWithContext(actualValue, fixedValue, textType.MemberTypes, scopeDepth, fixedContext); err == nil {
			return match
		}
	}

	// try value space comparison for union types from original SimpleType
	if st, ok := textType.Original.(*types.SimpleType); ok && st.Variety() == types.UnionVariety {
		memberTypes := r.resolveUnionMemberTypes(st)
		if len(memberTypes) > 0 {
			if match, err := r.compareFixedValueInUnionTypesWithContext(actualValue, fixedValue, memberTypes, scopeDepth, fixedContext); err == nil {
				return match
			}
		}
	}

	// try value space comparison for simple types
	if st, ok := valueSpaceType(textType.Original); ok {
		if match, err := r.compareFixedValueAsType(actualValue, fixedValue, st); err == nil {
			return match
		}
	}

	// fall back to normalized string comparison
	return fixedValueMatches(actualValue, fixedValue, textType.Original)
}

// resolveUnionMemberTypes resolves all member types from a union SimpleType.
func (r *streamRun) resolveUnionMemberTypes(st *types.SimpleType) []types.Type {
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

func (r *streamRun) compareFixedValueInUnionWithContext(actualValue, fixedValue string, memberTypes []*grammar.CompiledType, scopeDepth int, fixedContext map[string]string) (bool, error) {
	actualParsed, ok := r.parseUnionValueWithContext(actualValue, memberTypes, scopeDepth, nil)
	if !ok {
		return false, errNoMatchingMemberType
	}
	fixedParsed, ok := r.parseUnionValueWithContext(fixedValue, memberTypes, -1, fixedContext)
	if !ok {
		return false, errNoMatchingMemberType
	}
	return actualParsed.equal(fixedParsed), nil
}

func (r *streamRun) compareFixedValueInUnionTypesWithContext(actualValue, fixedValue string, memberTypes []types.Type, scopeDepth int, fixedContext map[string]string) (bool, error) {
	actualParsed, ok := r.parseUnionValueTypesWithContext(actualValue, memberTypes, scopeDepth, nil)
	if !ok {
		return false, errNoMatchingMemberType
	}
	fixedParsed, ok := r.parseUnionValueTypesWithContext(fixedValue, memberTypes, -1, fixedContext)
	if !ok {
		return false, errNoMatchingMemberType
	}
	return actualParsed.equal(fixedParsed), nil
}

// compareFixedValueAsType compares two values in the value space of a type.
// Returns (equal, nil) if both values can be parsed, or (false, error) if parsing fails.
func (r *streamRun) compareFixedValueAsType(actualValue, fixedValue string, typ types.Type) (bool, error) {
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
var errNoMatchingMemberType = fmt.Errorf("no matching member type")

type fixedValueParsed struct {
	typed   types.TypedValue
	qname   types.QName
	isQName bool
}

func (p fixedValueParsed) equal(other fixedValueParsed) bool {
	if p.isQName || other.isQName {
		return p.isQName && other.isQName && p.qname == other.qname
	}
	return compareTypedValues(p.typed, other.typed)
}

func (r *streamRun) parseUnionValueWithContext(value string, memberTypes []*grammar.CompiledType, scopeDepth int, fixedContext map[string]string) (fixedValueParsed, bool) {
	for _, member := range memberTypes {
		if member == nil || member.Original == nil {
			continue
		}
		memberType, ok := valueSpaceType(member.Original)
		if !ok {
			continue
		}
		if isQNameOrNotationType(memberType) {
			qname, err := r.parseQNameWithContext(value, memberType, scopeDepth, fixedContext)
			if err == nil {
				return fixedValueParsed{qname: qname, isQName: true}, true
			}
			continue
		}
		typedValue, err := r.parseValueAsType(value, memberType)
		if err == nil {
			return fixedValueParsed{typed: typedValue}, true
		}
	}
	return fixedValueParsed{}, false
}

func (r *streamRun) parseUnionValueTypesWithContext(value string, memberTypes []types.Type, scopeDepth int, fixedContext map[string]string) (fixedValueParsed, bool) {
	for _, member := range memberTypes {
		if member == nil {
			continue
		}
		memberType, ok := valueSpaceType(member)
		if !ok {
			continue
		}
		if isQNameOrNotationType(memberType) {
			qname, err := r.parseQNameWithContext(value, memberType, scopeDepth, fixedContext)
			if err == nil {
				return fixedValueParsed{qname: qname, isQName: true}, true
			}
			continue
		}
		typedValue, err := r.parseValueAsType(value, memberType)
		if err == nil {
			return fixedValueParsed{typed: typedValue}, true
		}
	}
	return fixedValueParsed{}, false
}

func (r *streamRun) compareFixedQNameValue(actualValue, fixedValue string, typ types.Type, scopeDepth int, fixedContext map[string]string) (bool, error) {
	actualQName, err := r.parseQNameWithContext(actualValue, typ, scopeDepth, nil)
	if err != nil {
		return false, err
	}
	fixedQName, err := r.parseQNameWithContext(fixedValue, typ, -1, fixedContext)
	if err != nil {
		return false, err
	}
	return actualQName == fixedQName, nil
}

func (r *streamRun) parseQNameWithContext(value string, typ types.Type, scopeDepth int, fixedContext map[string]string) (types.QName, error) {
	normalized := types.NormalizeWhiteSpace(value, typ)
	if fixedContext != nil {
		return types.ParseQNameValue(normalized, fixedContext)
	}
	return r.parseQNameValue(normalized, scopeDepth)
}

func (r *streamRun) compareFixedValueListWithContext(actualValue, fixedValue string, listType *grammar.CompiledType, scopeDepth int, fixedContext map[string]string) (bool, error) {
	if listType == nil || listType.ItemType == nil || listType.Original == nil {
		return false, errNoMatchingMemberType
	}
	itemType := listType.ItemType
	normalizedActual := types.NormalizeWhiteSpace(actualValue, listType.Original)
	normalizedFixed := types.NormalizeWhiteSpace(fixedValue, listType.Original)
	actualItems := types.SplitXMLWhitespaceFields(normalizedActual)
	fixedItems := types.SplitXMLWhitespaceFields(normalizedFixed)
	if len(actualItems) != len(fixedItems) {
		return false, nil
	}
	for i := range actualItems {
		if itemType.IsQNameOrNotationType {
			actualQName, err := r.parseQNameWithContext(actualItems[i], itemType.Original, scopeDepth, nil)
			if err != nil {
				return false, err
			}
			fixedQName, err := r.parseQNameWithContext(fixedItems[i], itemType.Original, -1, fixedContext)
			if err != nil {
				return false, err
			}
			if actualQName != fixedQName {
				return false, nil
			}
			continue
		}
		actualTyped, err := r.parseValueAsType(actualItems[i], itemType.Original)
		if err != nil {
			return false, err
		}
		fixedTyped, err := r.parseValueAsType(fixedItems[i], itemType.Original)
		if err != nil {
			return false, err
		}
		if !compareTypedValues(actualTyped, fixedTyped) {
			return false, nil
		}
	}
	return true, nil
}

func (r *streamRun) parseValueAsType(value string, typ types.Type) (types.TypedValue, error) {
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

func isQNameOrNotationTypeValue(typ types.Type) bool {
	if typ == nil {
		return false
	}
	switch t := typ.(type) {
	case *types.SimpleType:
		return t.IsQNameOrNotationType()
	case *types.BuiltinType:
		return t.IsQNameOrNotationType()
	default:
		if prim := typ.PrimitiveType(); prim != nil {
			switch p := prim.(type) {
			case *types.SimpleType:
				return p.IsQNameOrNotationType()
			case *types.BuiltinType:
				return p.IsQNameOrNotationType()
			}
		}
		return false
	}
}

func (r *streamRun) parseValueAsTypeWithScope(value string, typ types.Type, scopeDepth int) (types.TypedValue, error) {
	if isQNameOrNotationTypeValue(typ) {
		normalized := types.NormalizeWhiteSpace(value, typ)
		qname, err := r.parseQNameValue(normalized, scopeDepth)
		if err != nil {
			return nil, err
		}
		return qnameTypedValue{typ: typ, lexical: normalized, value: qname}, nil
	}
	return r.parseValueAsType(value, typ)
}

func (r *streamRun) parseValueAsTypeWithContext(value string, typ types.Type, context map[string]string) (types.TypedValue, error) {
	if isQNameOrNotationTypeValue(typ) {
		normalized := types.NormalizeWhiteSpace(value, typ)
		qname, err := types.ParseQNameValue(normalized, context)
		if err != nil {
			return nil, err
		}
		return qnameTypedValue{typ: typ, lexical: normalized, value: qname}, nil
	}
	return r.parseValueAsType(value, typ)
}

func (r *streamRun) parseUnionValueWithScope(value string, memberTypes []*grammar.CompiledType, scopeDepth int) (types.TypedValue, *grammar.CompiledType, error) {
	for _, member := range memberTypes {
		if member == nil || member.Original == nil {
			continue
		}
		memberType, ok := valueSpaceType(member.Original)
		if !ok {
			continue
		}
		typedValue, err := r.parseValueAsTypeWithScope(value, memberType, scopeDepth)
		if err == nil {
			return typedValue, member, nil
		}
	}
	return nil, nil, errNoMatchingMemberType
}

func (r *streamRun) parseUnionValueTypesWithScope(value string, memberTypes []types.Type, scopeDepth int) (types.TypedValue, types.Type, error) {
	for _, member := range memberTypes {
		if member == nil {
			continue
		}
		memberType, ok := valueSpaceType(member)
		if !ok {
			continue
		}
		typedValue, err := r.parseValueAsTypeWithScope(value, memberType, scopeDepth)
		if err == nil {
			return typedValue, member, nil
		}
	}
	return nil, nil, errNoMatchingMemberType
}

func (r *streamRun) parseUnionValueTypesWithContext(value string, memberTypes []types.Type, context map[string]string) (types.TypedValue, types.Type, error) {
	for _, member := range memberTypes {
		if member == nil {
			continue
		}
		memberType, ok := valueSpaceType(member)
		if !ok {
			continue
		}
		typedValue, err := r.parseValueAsTypeWithContext(value, memberType, context)
		if err == nil {
			return typedValue, member, nil
		}
	}
	return nil, nil, errNoMatchingMemberType
}

func (r *streamRun) compareFixedValueInUnionWithContext(actualValue, fixedValue string, memberTypes []*grammar.CompiledType, scopeDepth int, fixedContext map[string]string) (bool, error) {
	actualTyped, actualType, err := r.parseUnionValueWithScope(actualValue, memberTypes, scopeDepth)
	if err != nil {
		return false, err
	}
	fixedTyped, fixedType, err := r.parseUnionValueWithContext(fixedValue, memberTypes, fixedContext)
	if err != nil {
		return false, err
	}
	if actualType != fixedType {
		return false, nil
	}
	return compareTypedValues(actualTyped, fixedTyped), nil
}

func (r *streamRun) parseUnionValueWithContext(value string, memberTypes []*grammar.CompiledType, context map[string]string) (types.TypedValue, *grammar.CompiledType, error) {
	for _, member := range memberTypes {
		if member == nil || member.Original == nil {
			continue
		}
		memberType, ok := valueSpaceType(member.Original)
		if !ok {
			continue
		}
		typedValue, err := r.parseValueAsTypeWithContext(value, memberType, context)
		if err == nil {
			return typedValue, member, nil
		}
	}
	return nil, nil, errNoMatchingMemberType
}

func (r *streamRun) compareFixedValueInUnionTypesWithContext(actualValue, fixedValue string, memberTypes []types.Type, scopeDepth int, fixedContext map[string]string) (bool, error) {
	actualTyped, actualType, err := r.parseUnionValueTypesWithScope(actualValue, memberTypes, scopeDepth)
	if err != nil {
		return false, err
	}
	fixedTyped, fixedType, err := r.parseUnionValueTypesWithContext(fixedValue, memberTypes, fixedContext)
	if err != nil {
		return false, err
	}
	if actualType != fixedType {
		return false, nil
	}
	return compareTypedValues(actualTyped, fixedTyped), nil
}

func (r *streamRun) compareFixedValueAsTypeWithContext(actualValue, fixedValue string, typ types.Type, scopeDepth int, fixedContext map[string]string) (bool, error) {
	actualTyped, actualErr := r.parseValueAsTypeWithScope(actualValue, typ, scopeDepth)
	if actualErr != nil {
		return false, actualErr
	}

	fixedTyped, fixedErr := r.parseValueAsTypeWithContext(fixedValue, typ, fixedContext)
	if fixedErr != nil {
		return false, fixedErr
	}

	return compareTypedValues(actualTyped, fixedTyped), nil
}

func (r *streamRun) compareFixedValueWithContext(actualValue, fixedValue string, textType *grammar.CompiledType, scopeDepth int, fixedContext map[string]string) bool {
	if textType == nil || textType.Original == nil {
		return actualValue == fixedValue
	}

	if len(textType.MemberTypes) > 0 {
		if match, err := r.compareFixedValueInUnionWithContext(actualValue, fixedValue, textType.MemberTypes, scopeDepth, fixedContext); err == nil {
			return match
		}
	}

	if st, ok := textType.Original.(*types.SimpleType); ok && st.Variety() == types.UnionVariety {
		memberTypes := r.resolveUnionMemberTypes(st)
		if len(memberTypes) > 0 {
			if match, err := r.compareFixedValueInUnionTypesWithContext(actualValue, fixedValue, memberTypes, scopeDepth, fixedContext); err == nil {
				return match
			}
		}
	}

	if st, ok := valueSpaceType(textType.Original); ok {
		if match, err := r.compareFixedValueAsTypeWithContext(actualValue, fixedValue, st, scopeDepth, fixedContext); err == nil {
			return match
		}
	}

	return fixedValueMatches(actualValue, fixedValue, textType.Original)
}

func (r *streamRun) checkElementFixedValue(actualValue string, decl *grammar.CompiledElement, textType *grammar.CompiledType, scopeDepth int) []errors.Validation {
	if decl == nil || !decl.HasFixed {
		return nil
	}
	var context map[string]string
	if decl.Original != nil {
		context = decl.Original.FixedContext
	}
	match := r.compareFixedValueWithContext(actualValue, decl.Fixed, textType, scopeDepth, context)
	if !match {
		return []errors.Validation{errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
			"Element has fixed value '%s' but actual value is '%s'", decl.Fixed, actualValue)}
	}
	return nil
}

func (r *streamRun) checkAttributeFixedValue(actualValue string, decl *grammar.CompiledAttribute, scopeDepth int) []errors.Validation {
	if decl == nil || !decl.HasFixed {
		return nil
	}
	var context map[string]string
	if decl.Original != nil {
		context = decl.Original.FixedContext
	}
	match := r.compareFixedValueWithContext(actualValue, decl.Fixed, decl.Type, scopeDepth, context)
	if !match {
		return []errors.Validation{errors.NewValidationf(errors.ErrAttributeFixedValue, r.path.String(),
			"Attribute '%s' has fixed value '%s', but found '%s'", decl.QName.Local, decl.Fixed, actualValue)}
	}
	return nil
}
