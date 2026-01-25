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
func (r *streamRun) checkFixedValue(actualValue, fixedValue string, textType *grammar.CompiledType, actualNs, fixedNs map[string]string) []errors.Validation {
	if r.fixedValueEqual(actualValue, fixedValue, textType, actualNs, fixedNs) {
		return nil
	}
	return []errors.Validation{errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
		"Element has fixed value '%s' but actual value is '%s'", fixedValue, actualValue)}
}

func (r *streamRun) fixedValueEqual(actualValue, fixedValue string, textType *grammar.CompiledType, actualNs, fixedNs map[string]string) bool {
	if textType == nil || textType.Original == nil {
		return actualValue == fixedValue
	}

	if textType.IsQNameOrNotationType {
		if match, err := r.compareFixedQNameValue(actualValue, fixedValue, textType.Original, actualNs, fixedNs); err == nil {
			return match
		}
		return false
	}

	if textType.ItemType != nil {
		if match, err := r.compareFixedValueList(actualValue, fixedValue, textType, actualNs, fixedNs); err == nil {
			return match
		}
		return false
	}

	// try value space comparison for union types with compiled member types
	if len(textType.MemberTypes) > 0 {
		if match, err := r.compareFixedValueInUnion(actualValue, fixedValue, textType.MemberTypes, actualNs, fixedNs); err == nil {
			return match
		}
	}

	// try value space comparison for union types from original SimpleType
	if st, ok := textType.Original.(*types.SimpleType); ok && st.Variety() == types.UnionVariety {
		memberTypes := r.resolveUnionMemberTypes(st)
		if len(memberTypes) > 0 {
			if match, err := r.compareFixedValueInUnionTypes(actualValue, fixedValue, memberTypes, actualNs, fixedNs); err == nil {
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

func (r *streamRun) compareFixedValueInUnion(actualValue, fixedValue string, memberTypes []*grammar.CompiledType, actualNs, fixedNs map[string]string) (bool, error) {
	actualParser := contextValueParser{run: r, context: actualNs}
	fixedParser := contextValueParser{run: r, context: fixedNs}
	return r.compareFixedValueInUnionWithParsers(actualValue, fixedValue, memberTypes, actualParser, fixedParser)
}

func (r *streamRun) compareFixedValueInUnionTypes(actualValue, fixedValue string, memberTypes []types.Type, actualNs, fixedNs map[string]string) (bool, error) {
	actualParser := contextValueParser{run: r, context: actualNs}
	fixedParser := contextValueParser{run: r, context: fixedNs}
	return r.compareFixedValueInUnionTypesWithParsers(actualValue, fixedValue, memberTypes, actualParser, fixedParser)
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

type nsResolver interface {
	resolveQName(value string) (types.QName, error)
}

type scopeQNameResolver struct {
	run   *streamRun
	depth int
}

func (r scopeQNameResolver) resolveQName(value string) (types.QName, error) {
	if r.run == nil {
		return types.QName{}, fmt.Errorf("namespace context unavailable")
	}
	return r.run.parseQNameValue(value, r.depth)
}

type contextQNameResolver struct {
	context map[string]string
}

func (r contextQNameResolver) resolveQName(value string) (types.QName, error) {
	return types.ParseQNameValue(value, r.context)
}

type valueParser interface {
	parseValue(value string, typ types.Type) (types.TypedValue, error)
}

type scopeValueParser struct {
	run   *streamRun
	depth int
}

func (p scopeValueParser) parseValue(value string, typ types.Type) (types.TypedValue, error) {
	return p.run.parseValueAsTypeWithScope(value, typ, p.depth)
}

type contextValueParser struct {
	run     *streamRun
	context map[string]string
}

func (p contextValueParser) parseValue(value string, typ types.Type) (types.TypedValue, error) {
	return p.run.parseValueAsTypeWithContext(value, typ, p.context)
}

func (r *streamRun) compareFixedValueInUnionWithParsers(
	actualValue, fixedValue string,
	memberTypes []*grammar.CompiledType,
	actualParser, fixedParser valueParser,
) (bool, error) {
	if len(memberTypes) == 0 {
		return false, errNoMatchingMemberType
	}
	actualTyped, ok := r.selectUnionCandidateValue(actualValue, memberTypes, actualParser)
	if !ok {
		return false, errNoMatchingMemberType
	}
	fixedTyped, ok := r.selectUnionCandidateValue(fixedValue, memberTypes, fixedParser)
	if !ok {
		return false, errNoMatchingMemberType
	}
	return compareTypedValues(actualTyped, fixedTyped), nil
}

func (r *streamRun) compareFixedValueInUnionTypesWithParsers(
	actualValue, fixedValue string,
	memberTypes []types.Type,
	actualParser, fixedParser valueParser,
) (bool, error) {
	if len(memberTypes) == 0 {
		return false, errNoMatchingMemberType
	}
	actualTyped, ok := r.selectUnionCandidateTypeValue(actualValue, memberTypes, actualParser)
	if !ok {
		return false, errNoMatchingMemberType
	}
	fixedTyped, ok := r.selectUnionCandidateTypeValue(fixedValue, memberTypes, fixedParser)
	if !ok {
		return false, errNoMatchingMemberType
	}
	return compareTypedValues(actualTyped, fixedTyped), nil
}

func (r *streamRun) selectUnionCandidateValue(value string, memberTypes []*grammar.CompiledType, parser valueParser) (types.TypedValue, bool) {
	for _, member := range memberTypes {
		if member == nil || member.Original == nil {
			continue
		}
		typed, ok := r.parseUnionMemberValueWithParser(value, member, parser)
		if !ok {
			continue
		}
		return typed, true
	}
	return nil, false
}

func (r *streamRun) selectUnionCandidateTypeValue(value string, memberTypes []types.Type, parser valueParser) (types.TypedValue, bool) {
	for _, member := range memberTypes {
		if member == nil {
			continue
		}
		typed, ok := r.parseUnionMemberTypeValueWithParser(value, member, parser)
		if !ok {
			continue
		}
		return typed, true
	}
	return nil, false
}

func (r *streamRun) parseUnionMemberValueWithParser(value string, member *grammar.CompiledType, parser valueParser) (types.TypedValue, bool) {
	if member == nil || member.Original == nil {
		return nil, false
	}
	memberType, ok := valueSpaceType(member.Original)
	if !ok {
		return nil, false
	}
	typedValue, err := parser.parseValue(value, memberType)
	if err != nil {
		return nil, false
	}
	return typedValue, true
}

func (r *streamRun) parseUnionMemberTypeValueWithParser(value string, member types.Type, parser valueParser) (types.TypedValue, bool) {
	if member == nil {
		return nil, false
	}
	memberType, ok := valueSpaceType(member)
	if !ok {
		return nil, false
	}
	typedValue, err := parser.parseValue(value, memberType)
	if err != nil {
		return nil, false
	}
	return typedValue, true
}

func (r *streamRun) compareFixedQNameValueWithResolvers(actualValue, fixedValue string, typ types.Type, actualResolver, fixedResolver nsResolver) (bool, error) {
	normalizedActual := types.NormalizeWhiteSpace(actualValue, typ)
	actualQName, err := actualResolver.resolveQName(normalizedActual)
	if err != nil {
		return false, err
	}
	normalizedFixed := types.NormalizeWhiteSpace(fixedValue, typ)
	fixedQName, err := fixedResolver.resolveQName(normalizedFixed)
	if err != nil {
		return false, err
	}
	return actualQName == fixedQName, nil
}

func (r *streamRun) compareFixedQNameValue(actualValue, fixedValue string, typ types.Type, actualNs, fixedNs map[string]string) (bool, error) {
	actualResolver := contextQNameResolver{context: actualNs}
	fixedResolver := contextQNameResolver{context: fixedNs}
	return r.compareFixedQNameValueWithResolvers(actualValue, fixedValue, typ, actualResolver, fixedResolver)
}

func (r *streamRun) compareFixedQNameValueWithScope(actualValue, fixedValue string, typ types.Type, scopeDepth int, fixedContext map[string]string) (bool, error) {
	actualResolver := scopeQNameResolver{run: r, depth: scopeDepth}
	fixedResolver := contextQNameResolver{context: fixedContext}
	return r.compareFixedQNameValueWithResolvers(actualValue, fixedValue, typ, actualResolver, fixedResolver)
}

func (r *streamRun) compareFixedValueListWithScope(actualValue, fixedValue string, listType *grammar.CompiledType, scopeDepth int, fixedContext map[string]string) (bool, error) {
	actualParser := scopeValueParser{run: r, depth: scopeDepth}
	fixedParser := contextValueParser{run: r, context: fixedContext}
	return r.compareFixedValueListWithParsers(actualValue, fixedValue, listType, actualParser, fixedParser)
}

func (r *streamRun) parseFixedListItemWithParser(value string, itemType *grammar.CompiledType, parser valueParser) (types.TypedValue, error) {
	if itemType == nil || itemType.Original == nil {
		return nil, errNoMatchingMemberType
	}
	itemValueType, ok := valueSpaceType(itemType.Original)
	if !ok {
		return nil, errNoMatchingMemberType
	}
	typedValue, err := parser.parseValue(value, itemValueType)
	if err != nil {
		return nil, err
	}
	return typedValue, nil
}

func (r *streamRun) compareFixedValueList(actualValue, fixedValue string, listType *grammar.CompiledType, actualNs, fixedNs map[string]string) (bool, error) {
	actualParser := contextValueParser{run: r, context: actualNs}
	fixedParser := contextValueParser{run: r, context: fixedNs}
	return r.compareFixedValueListWithParsers(actualValue, fixedValue, listType, actualParser, fixedParser)
}

func (r *streamRun) compareFixedValueListWithParsers(actualValue, fixedValue string, listType *grammar.CompiledType, actualParser, fixedParser valueParser) (bool, error) {
	if listType == nil || listType.ItemType == nil || listType.Original == nil {
		return false, errNoMatchingMemberType
	}
	itemType := listType.ItemType
	normalizedActual := types.NormalizeWhiteSpace(actualValue, listType.Original)
	normalizedFixed := types.NormalizeWhiteSpace(fixedValue, listType.Original)
	var actualItems, fixedItems []string
	for item := range types.FieldsXMLWhitespaceSeq(normalizedActual) {
		actualItems = append(actualItems, item)
	}
	for item := range types.FieldsXMLWhitespaceSeq(normalizedFixed) {
		fixedItems = append(fixedItems, item)
	}
	if len(actualItems) != len(fixedItems) {
		return false, nil
	}
	for i := range actualItems {
		if len(itemType.MemberTypes) > 0 {
			match, err := r.compareFixedValueInUnionWithParsers(actualItems[i], fixedItems[i], itemType.MemberTypes, actualParser, fixedParser)
			if err != nil {
				return false, err
			}
			if !match {
				return false, nil
			}
			continue
		}
		if st, ok := itemType.Original.(*types.SimpleType); ok && st.Variety() == types.UnionVariety {
			memberTypes := r.resolveUnionMemberTypes(st)
			if len(memberTypes) > 0 {
				match, err := r.compareFixedValueInUnionTypesWithParsers(actualItems[i], fixedItems[i], memberTypes, actualParser, fixedParser)
				if err != nil {
					return false, err
				}
				if !match {
					return false, nil
				}
				continue
			}
		}
		actualParsed, err := r.parseFixedListItemWithParser(actualItems[i], itemType, actualParser)
		if err != nil {
			return false, err
		}
		fixedParsed, err := r.parseFixedListItemWithParser(fixedItems[i], itemType, fixedParser)
		if err != nil {
			return false, err
		}
		if !compareTypedValues(actualParsed, fixedParsed) {
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

func (r *streamRun) parseValueAsTypeWithScope(value string, typ types.Type, scopeDepth int) (types.TypedValue, error) {
	if types.IsQNameOrNotationType(typ) {
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
	if types.IsQNameOrNotationType(typ) {
		normalized := types.NormalizeWhiteSpace(value, typ)
		qname, err := types.ParseQNameValue(normalized, context)
		if err != nil {
			return nil, err
		}
		return qnameTypedValue{typ: typ, lexical: normalized, value: qname}, nil
	}
	return r.parseValueAsType(value, typ)
}

func (r *streamRun) compareFixedValueInUnionWithContext(actualValue, fixedValue string, memberTypes []*grammar.CompiledType, scopeDepth int, fixedContext map[string]string) (bool, error) {
	actualParser := scopeValueParser{run: r, depth: scopeDepth}
	fixedParser := contextValueParser{run: r, context: fixedContext}
	return r.compareFixedValueInUnionWithParsers(actualValue, fixedValue, memberTypes, actualParser, fixedParser)
}

func (r *streamRun) compareFixedValueInUnionTypesWithContext(actualValue, fixedValue string, memberTypes []types.Type, scopeDepth int, fixedContext map[string]string) (bool, error) {
	actualParser := scopeValueParser{run: r, depth: scopeDepth}
	fixedParser := contextValueParser{run: r, context: fixedContext}
	return r.compareFixedValueInUnionTypesWithParsers(actualValue, fixedValue, memberTypes, actualParser, fixedParser)
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

	if textType.IsQNameOrNotationType {
		if match, err := r.compareFixedQNameValueWithScope(actualValue, fixedValue, textType.Original, scopeDepth, fixedContext); err == nil {
			return match
		}
		return false
	}

	if textType.ItemType != nil {
		if match, err := r.compareFixedValueListWithScope(actualValue, fixedValue, textType, scopeDepth, fixedContext); err == nil {
			return match
		}
		return false
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
