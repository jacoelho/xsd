package validator

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

type errorPolicy int

const (
	errorPolicyReport errorPolicy = iota
	errorPolicySuppress
)

// checkSimpleValue validates a string value against a simple type using namespace scope.
func (r *streamRun) checkSimpleValue(value string, st *grammar.CompiledType, scopeDepth int) []errors.Validation {
	_, violations := r.checkSimpleValueInternal(value, st, scopeDepth, errorPolicyReport, nil)
	return violations
}

func (r *streamRun) checkSimpleValueWithContext(value string, st *grammar.CompiledType, context map[string]string) []errors.Validation {
	_, violations := r.checkSimpleValueInternal(value, st, 0, errorPolicyReport, context)
	return violations
}

func (r *streamRun) checkSimpleValueInternal(value string, st *grammar.CompiledType, scopeDepth int, policy errorPolicy, context map[string]string) (bool, []errors.Validation) {
	if st == nil || st.Original == nil {
		return true, nil
	}

	if unresolvedName, ok := unresolvedSimpleType(st.Original); ok {
		if policy == errorPolicyReport {
			return false, []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
				"type '%s' is not resolved", unresolvedName)}
		}
		return false, nil
	}

	normalizedValue := types.NormalizeWhiteSpace(value, st.Original)

	if len(st.MemberTypes) > 0 {
		if !r.validateUnionValue(normalizedValue, st.MemberTypes, scopeDepth, context) {
			if policy == errorPolicyReport {
				return false, []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"value '%s' does not match any member type of union", normalizedValue)}
			}
			return false, nil
		}
		return r.validateSimpleTypeFacets(normalizedValue, st, scopeDepth, policy, context)
	}

	if st.ItemType != nil {
		return r.validateListValueInternal(normalizedValue, st, scopeDepth, policy, context)
	}

	switch orig := st.Original.(type) {
	case *types.SimpleType:
		if err := orig.Validate(normalizedValue); err != nil {
			if policy == errorPolicyReport {
				return false, []errors.Validation{errors.NewValidation(errors.ErrDatatypeInvalid, err.Error(), r.path.String())}
			}
			return false, nil
		}
	case *types.BuiltinType:
		if err := orig.Validate(normalizedValue); err != nil {
			if policy == errorPolicyReport {
				return false, []errors.Validation{errors.NewValidation(errors.ErrDatatypeInvalid, err.Error(), r.path.String())}
			}
			return false, nil
		}
	}

	if isQNameType(st) {
		if err := r.validateQNameContext(normalizedValue, scopeDepth, context); err != nil {
			if policy == errorPolicyReport {
				return false, []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"invalid QName value '%s': %v", normalizedValue, err)}
			}
			return false, nil
		}
	}

	if r.isNotationType(st) {
		if policy == errorPolicyReport {
			if violations := r.validateNotationReference(normalizedValue, scopeDepth, context); len(violations) > 0 {
				return false, violations
			}
		} else if !r.isValidNotationReference(normalizedValue, scopeDepth, context) {
			return false, nil
		}
	}

	return r.validateSimpleTypeFacets(normalizedValue, st, scopeDepth, policy, context)
}

func (r *streamRun) validateSimpleTypeFacets(normalizedValue string, st *grammar.CompiledType, scopeDepth int, policy errorPolicy, context map[string]string) (bool, []errors.Validation) {
	if st == nil || len(st.Facets) == 0 {
		return true, nil
	}

	var violations []errors.Validation
	var typedValue types.TypedValue
	for _, facet := range st.Facets {
		if shouldSkipLengthFacet(st, facet) {
			continue
		}
		if enumFacet, ok := facet.(*types.Enumeration); ok && st.IsQNameOrNotationType {
			if err := r.validateQNameEnumeration(normalizedValue, enumFacet, scopeDepth, context); err != nil {
				if policy == errorPolicySuppress {
					return false, nil
				}
				violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
			}
			continue
		}
		if lexicalFacet, ok := facet.(types.LexicalValidator); ok {
			if err := lexicalFacet.ValidateLexical(normalizedValue, st.Original); err != nil {
				if policy == errorPolicySuppress {
					return false, nil
				}
				violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
			}
			continue
		}
		if typedValue == nil {
			typedValue = typedValueForFacets(normalizedValue, st.Original, st.Facets)
		}
		if err := facet.Validate(typedValue, st.Original); err != nil {
			if policy == errorPolicySuppress {
				return false, nil
			}
			violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
		}
	}

	if len(violations) > 0 {
		return false, violations
	}
	return true, nil
}

func (r *streamRun) checkComplexTypeFacetsWithContext(text string, ct *grammar.CompiledType, scopeDepth int, context map[string]string) []errors.Validation {
	if ct == nil || ct.SimpleContentType == nil || len(ct.Facets) == 0 {
		return nil
	}

	var violations []errors.Validation
	var typedValue types.TypedValue
	for _, facet := range st.Facets {
		if shouldSkipLengthFacet(st, facet) {
			continue
		}
		if enumFacet, ok := facet.(*types.Enumeration); ok && ct.SimpleContentType.IsQNameOrNotationType {
			if err := r.validateQNameEnumeration(normalizedValue, enumFacet, scopeDepth, context); err != nil {
				violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
			}
			continue
		}
		if lexicalFacet, ok := facet.(types.LexicalValidator); ok {
			if err := lexicalFacet.ValidateLexical(normalizedValue, st.Original); err != nil {
				if policy == errorPolicySuppress {
					return false, nil
				}
				violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
			}
			continue
		}
		if typedValue == nil {
			typedValue = typedValueForFacets(normalizedValue, st.Original, st.Facets)
		}
		if err := facet.Validate(typedValue, st.Original); err != nil {
			if policy == errorPolicySuppress {
				return false, nil
			}
			violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
		}
	}

	if len(violations) > 0 {
		return false, violations
	}
	return true, nil
}

func (r *streamRun) validateQNameContextWithContext(value string, typ types.Type, scopeDepth int, valueContext map[string]string) error {
	if valueContext == nil {
		return r.validateQNameContext(value, scopeDepth)
	}
	_, err := r.parseQNameWithContext(value, typ, -1, valueContext)
	return err
}

func (r *streamRun) validateQNameEnumerationWithContext(value string, enum *types.Enumeration, typ types.Type, scopeDepth int, valueContext map[string]string) error {
	if valueContext == nil {
		return r.validateQNameEnumeration(value, enum, scopeDepth)
	}
	if enum == nil {
		return nil
	}
	qname, err := r.parseQNameWithContext(value, typ, -1, valueContext)
	if err != nil {
		return err
	}
	allowedQNames, err := enumerationQNameValues(enum)
	if err != nil {
		return err
	}
	if slices.Contains(allowedQNames, qname) {
		return nil
	}
	return fmt.Errorf("value %s not in enumeration: %s", value, types.FormatEnumerationValues(enum.Values))
}

func (r *streamRun) validateNotationReferenceWithContext(value string, typ types.Type, scopeDepth int, valueContext map[string]string) []errors.Validation {
	if valueContext == nil {
		return r.validateNotationReference(value, scopeDepth)
	}
	notationQName, err := r.parseQNameWithContext(value, typ, -1, valueContext)
	if err != nil {
		return []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
			"Invalid NOTATION value '%s': %v", value, err)}
	}
	if r.schema.Notation(notationQName) == nil {
		return []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
			"NOTATION value '%s' does not reference a declared notation", value)}
	}
	return nil
}

func (r *streamRun) isValidNotationReferenceWithContext(value string, typ types.Type, scopeDepth int, valueContext map[string]string) bool {
	if valueContext == nil {
		return r.isValidNotationReference(value, scopeDepth)
	}
	notationQName, err := r.parseQNameWithContext(value, typ, -1, valueContext)
	if err != nil {
		return false
	}
	return r.schema.Notation(notationQName) != nil
}

func (r *streamRun) checkComplexTypeFacetsWithContext(text string, ct *grammar.CompiledType, scopeDepth int) []errors.Validation {
	return collectComplexTypeFacetViolations(text, ct, r.path.String(), func(normalized string, enum *types.Enumeration) error {
		return r.validateQNameEnumeration(normalized, enum, scopeDepth)
	})
}

func (r *streamRun) validateListValueInternal(value string, st *grammar.CompiledType, scopeDepth int, policy errorPolicy, context map[string]string) (bool, []errors.Validation) {
	valid := true
	var violations []errors.Validation
	abort := false
	index := 0
	splitWhitespaceSeq(value, func(item string) bool {
		itemValid, itemViolations := r.validateListItemNormalized(item, st.ItemType, index, scopeDepth, policy, context)
		index++
		if !itemValid {
			valid = false
			if policy == errorPolicyReport {
				violations = append(violations, itemViolations...)
				return true
			}
			abort = true
			return false
		}
		return true
	})
	if abort {
		return false, nil
	}

	if len(st.Facets) > 0 {
		var typedValue types.TypedValue
		for _, facet := range st.Facets {
			if shouldSkipLengthFacet(st, facet) {
				continue
			}
			if lexicalFacet, ok := facet.(types.LexicalValidator); ok {
				if err := lexicalFacet.ValidateLexical(value, st.Original); err != nil {
					valid = false
					if policy == errorPolicySuppress {
						return false, nil
					}
					violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
				}
				continue
			}
			if typedValue == nil {
				typedValue = typedValueForFacets(value, st.Original, st.Facets)
			}
			if err := facet.Validate(typedValue, st.Original); err != nil {
				valid = false
				if policy == errorPolicySuppress {
					return false, nil
				}
				violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
			}
		}
	}

	if len(violations) > 0 {
		return false, violations
	}
	return valid, nil
}

func (r *streamRun) validateListItemNormalized(item string, itemType *grammar.CompiledType, index, scopeDepth int, policy errorPolicy, context map[string]string) (bool, []errors.Validation) {
	if itemType == nil || itemType.Original == nil {
		return true, nil
	}

	if unresolvedName, ok := unresolvedSimpleType(itemType.Original); ok {
		if policy == errorPolicyReport {
			return false, []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
				"list item[%d]: type '%s' is not resolved", index, unresolvedName)}
		}
		return false, nil
	}

	var violations []errors.Validation

	if len(itemType.MemberTypes) > 0 {
		if !r.validateUnionValue(item, itemType.MemberTypes, scopeDepth, context) {
			if policy == errorPolicyReport {
				violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"list item[%d] '%s' does not match any member type of union", index, item))
				return false, violations
			}
			return false, nil
		}
	}

	switch orig := itemType.Original.(type) {
	case *types.SimpleType:
		if err := validateSimpleTypeNormalized(orig, item); err != nil {
			if policy == errorPolicyReport {
				violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"list item[%d]: %s", index, err.Error()))
				return false, violations
			}
			return false, nil
		}
	case *types.BuiltinType:
		if err := orig.Validate(item); err != nil {
			if policy == errorPolicyReport {
				violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"list item[%d]: %s", index, err.Error()))
				return false, violations
			}
			return false, nil
		}
	}

	if isQNameType(itemType) {
		if err := r.validateQNameContext(item, scopeDepth, context); err != nil {
			if policy == errorPolicyReport {
				violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"list item[%d]: invalid QName value '%s': %v", index, item, err))
				return false, violations
			}
			return false, nil
		}
	}

	if r.isNotationType(itemType) {
		if policy == errorPolicyReport {
			if itemViolations := r.validateNotationReference(item, scopeDepth, context); len(itemViolations) > 0 {
				violations = append(violations, itemViolations...)
				return false, violations
			}
		} else if !r.isValidNotationReference(item, scopeDepth, context) {
			return false, nil
		}
	}

	if len(itemType.Facets) > 0 {
		var typedValue types.TypedValue
		for _, facet := range itemType.Facets {
			if shouldSkipLengthFacet(itemType, facet) {
				continue
			}
			if enumFacet, ok := facet.(*types.Enumeration); ok && itemType.IsQNameOrNotationType {
				if err := r.validateQNameEnumeration(item, enumFacet, scopeDepth, context); err != nil {
					if policy == errorPolicySuppress {
						return false, nil
					}
					violations = append(violations, errors.NewValidationf(errors.ErrFacetViolation, r.path.String(),
						"list item[%d]: %s", index, err.Error()))
				}
				continue
			}
			if lexicalFacet, ok := facet.(types.LexicalValidator); ok {
				if err := lexicalFacet.ValidateLexical(item, itemType.Original); err != nil {
					if policy == errorPolicySuppress {
						return false, nil
					}
					violations = append(violations, errors.NewValidationf(errors.ErrFacetViolation, r.path.String(),
						"list item[%d]: %s", index, err.Error()))
				}
				continue
			}
			if typedValue == nil {
				typedValue = typedValueForFacets(item, itemType.Original, itemType.Facets)
			}
			if err := facet.Validate(typedValue, itemType.Original); err != nil {
				if policy == errorPolicySuppress {
					return false, nil
				}
				violations = append(violations, errors.NewValidationf(errors.ErrFacetViolation, r.path.String(),
					"list item[%d]: %s", index, err.Error()))
			}
		}
	}

	if len(violations) > 0 {
		return false, violations
	}
	return true, nil
}

func (r *streamRun) validateListItemNormalizedWithContext(item string, itemType *grammar.CompiledType, index, scopeDepth int, valueContext map[string]string, policy errorPolicy) (bool, []errors.Validation) {
	if itemType == nil || itemType.Original == nil {
		return true, nil
	}

	if unresolvedName, ok := unresolvedSimpleType(itemType.Original); ok {
		if policy == errorPolicyReport {
			return false, []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
				"list item[%d]: type '%s' is not resolved", index, unresolvedName)}
		}
		return false, nil
	}

	var violations []errors.Validation

	if len(itemType.MemberTypes) > 0 {
		if !r.validateUnionValueWithContext(item, itemType.MemberTypes, scopeDepth, valueContext) {
			if policy == errorPolicyReport {
				violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"list item[%d] '%s' does not match any member type of union", index, item))
				return false, violations
			}
			return false, nil
		}
	}

	switch orig := itemType.Original.(type) {
	case *types.SimpleType:
		if err := validateSimpleTypeNormalized(orig, item); err != nil {
			if policy == errorPolicyReport {
				violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"list item[%d]: %s", index, err.Error()))
				return false, violations
			}
			return false, nil
		}
	case *types.BuiltinType:
		if err := orig.Validate(item); err != nil {
			if policy == errorPolicyReport {
				violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"list item[%d]: %s", index, err.Error()))
				return false, violations
			}
			return false, nil
		}
	}

	if isQNameType(itemType) {
		if err := r.validateQNameContextWithContext(item, itemType.Original, scopeDepth, valueContext); err != nil {
			if policy == errorPolicyReport {
				violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"list item[%d]: invalid QName value '%s': %v", index, item, err))
				return false, violations
			}
			return false, nil
		}
	}

	if r.isNotationType(itemType) {
		if policy == errorPolicyReport {
			if itemViolations := r.validateNotationReferenceWithContext(item, itemType.Original, scopeDepth, valueContext); len(itemViolations) > 0 {
				violations = append(violations, itemViolations...)
				return false, violations
			}
		} else if !r.isValidNotationReferenceWithContext(item, itemType.Original, scopeDepth, valueContext) {
			return false, nil
		}
	}

	if len(itemType.Facets) > 0 {
		var typedValue types.TypedValue
		for _, facet := range itemType.Facets {
			if shouldSkipLengthFacet(itemType, facet) {
				continue
			}
			if enumFacet, ok := facet.(*types.Enumeration); ok && itemType.IsQNameOrNotationType {
				if err := r.validateQNameEnumerationWithContext(item, enumFacet, itemType.Original, scopeDepth, valueContext); err != nil {
					if policy == errorPolicySuppress {
						return false, nil
					}
					violations = append(violations, errors.NewValidationf(errors.ErrFacetViolation, r.path.String(),
						"list item[%d]: %s", index, err.Error()))
				}
				continue
			}
			if lexicalFacet, ok := facet.(types.LexicalValidator); ok {
				if err := lexicalFacet.ValidateLexical(item, itemType.Original); err != nil {
					if policy == errorPolicySuppress {
						return false, nil
					}
					violations = append(violations, errors.NewValidationf(errors.ErrFacetViolation, r.path.String(),
						"list item[%d]: %s", index, err.Error()))
				}
				continue
			}
			if typedValue == nil {
				typedValue = typedValueForFacets(item, itemType.Original, itemType.Facets)
			}
			if err := facet.Validate(typedValue, itemType.Original); err != nil {
				if policy == errorPolicySuppress {
					return false, nil
				}
				violations = append(violations, errors.NewValidationf(errors.ErrFacetViolation, r.path.String(),
					"list item[%d]: %s", index, err.Error()))
			}
		}
	}

	if len(violations) > 0 {
		return false, violations
	}
	return true, nil
}

func validateSimpleTypeNormalized(st *types.SimpleType, normalized string) error {
	if st == nil {
		return nil
	}
	if st.IsBuiltin() {
		if bt := types.GetBuiltinNS(st.QName.Namespace, st.QName.Local); bt != nil {
			return bt.Validate(normalized)
		}
	}
	if st.Restriction != nil && st.Variety() == types.AtomicVariety {
		primitive := st.PrimitiveType()
		if builtinType, ok := types.AsBuiltinType(primitive); ok {
			return builtinType.Validate(normalized)
		}
		if primitiveST, ok := types.AsSimpleType(primitive); ok && primitiveST.IsBuiltin() {
			if builtinType := types.GetBuiltinNS(primitiveST.QName.Namespace, primitiveST.QName.Local); builtinType != nil {
				return builtinType.Validate(normalized)
			}
		}
	}
	return nil
}

func (r *streamRun) validateUnionValue(value string, memberTypes []*grammar.CompiledType, scopeDepth int, context map[string]string) bool {
	for _, memberType := range memberTypes {
		if r.validateUnionMemberType(value, memberType, scopeDepth, context) {
			return true
		}
	}
	return false
}

func (r *streamRun) validateUnionMemberType(value string, mt *grammar.CompiledType, scopeDepth int, context map[string]string) bool {
	if mt == nil || mt.Original == nil {
		return false
	}

	valid, _ := r.checkSimpleValueInternal(value, mt, scopeDepth, errorPolicySuppress, context)
	return valid
}

func (r *streamRun) validateNotationReference(value string, scopeDepth int, context map[string]string) []errors.Validation {
	notationQName, err := r.parseQNameValueWithContext(value, scopeDepth, context)
	if err != nil {
		return []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
			"Invalid NOTATION value '%s': %v", value, err)}
	}

	if r.schema.Notation(notationQName) == nil {
		return []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
			"NOTATION value '%s' does not reference a declared notation", value)}
	}

	return nil
}

func (r *streamRun) isValidNotationReference(value string, scopeDepth int, context map[string]string) bool {
	notationQName, err := r.parseQNameValueWithContext(value, scopeDepth, context)
	if err != nil {
		return false
	}
	return r.schema.Notation(notationQName) != nil
}

func (r *streamRun) parseQNameValue(value string, scopeDepth int) (types.QName, error) {
	if r == nil || r.dec == nil {
		return types.QName{}, fmt.Errorf("namespace context unavailable")
	}
	prefix, local, hasPrefix, err := types.ParseQName(value)
	if err != nil {
		return types.QName{}, err
	}
	var ns types.NamespaceURI
	if hasPrefix {
		nsStr, ok := r.dec.LookupNamespaceAt(prefix, scopeDepth)
		if !ok {
			return types.QName{}, fmt.Errorf("undefined namespace prefix '%s'", prefix)
		}
		ns = types.NamespaceURI(nsStr)
	} else {
		if nsStr, ok := r.dec.LookupNamespaceAt("", scopeDepth); ok {
			ns = types.NamespaceURI(nsStr)
		}
	}

	return types.QName{Namespace: ns, Local: local}, nil
}

func (r *streamRun) parseQNameValueWithContext(value string, scopeDepth int, context map[string]string) (types.QName, error) {
	if context != nil {
		return types.ParseQNameValue(value, context)
	}
	return r.parseQNameValue(value, scopeDepth)
}

func (r *streamRun) validateQNameContext(value string, scopeDepth int, context map[string]string) error {
	_, err := r.parseQNameValueWithContext(value, scopeDepth, context)
	return err
}

func isQNameType(ct *grammar.CompiledType) bool {
	return ct != nil && ct.IsQNameOrNotationType && !ct.IsNotationType
}
