package validator

import (
	"fmt"

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
	_, violations := r.checkSimpleValueInternal(value, st, scopeDepth, errorPolicyReport)
	return violations
}

func (r *streamRun) checkSimpleValueInternal(value string, st *grammar.CompiledType, scopeDepth int, policy errorPolicy) (bool, []errors.Validation) {
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
		if !r.validateUnionValue(normalizedValue, st.MemberTypes, scopeDepth) {
			if policy == errorPolicyReport {
				return false, []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"value '%s' does not match any member type of union", normalizedValue)}
			}
			return false, nil
		}
		return r.validateSimpleTypeFacets(normalizedValue, st, scopeDepth, policy)
	}

	if st.ItemType != nil {
		return r.validateListValueInternal(normalizedValue, st, scopeDepth, policy)
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
		if err := r.validateQNameContext(normalizedValue, scopeDepth); err != nil {
			if policy == errorPolicyReport {
				return false, []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"invalid QName value '%s': %v", normalizedValue, err)}
			}
			return false, nil
		}
	}

	if r.isNotationType(st) {
		if policy == errorPolicyReport {
			if violations := r.validateNotationReference(normalizedValue, scopeDepth); len(violations) > 0 {
				return false, violations
			}
		} else if !r.isValidNotationReference(normalizedValue, scopeDepth) {
			return false, nil
		}
	}

	return r.validateSimpleTypeFacets(normalizedValue, st, scopeDepth, policy)
}

func (r *streamRun) validateSimpleTypeFacets(normalizedValue string, st *grammar.CompiledType, scopeDepth int, policy errorPolicy) (bool, []errors.Validation) {
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
			if err := r.validateQNameEnumeration(normalizedValue, enumFacet, scopeDepth); err != nil {
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

func (r *streamRun) checkComplexTypeFacetsWithContext(text string, ct *grammar.CompiledType, scopeDepth int) []errors.Validation {
	if ct == nil || ct.SimpleContentType == nil || len(ct.Facets) == 0 {
		return nil
	}

	normalizedValue := types.NormalizeWhiteSpace(text, ct.SimpleContentType.Original)
	var violations []errors.Validation
	var typedValue types.TypedValue
	for _, facet := range ct.Facets {
		if shouldSkipLengthFacet(ct.SimpleContentType, facet) {
			continue
		}
		if enumFacet, ok := facet.(*types.Enumeration); ok && ct.SimpleContentType.IsQNameOrNotationType {
			if err := r.validateQNameEnumeration(normalizedValue, enumFacet, scopeDepth); err != nil {
				violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
			}
			continue
		}
		if lexicalFacet, ok := facet.(types.LexicalValidator); ok {
			if err := lexicalFacet.ValidateLexical(normalizedValue, ct.SimpleContentType.Original); err != nil {
				violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
			}
			continue
		}
		if typedValue == nil {
			typedValue = typedValueForFacets(normalizedValue, ct.SimpleContentType.Original, ct.Facets)
		}
		if err := facet.Validate(typedValue, ct.SimpleContentType.Original); err != nil {
			violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
		}
	}

	return violations
}

func (r *streamRun) validateListValueInternal(value string, st *grammar.CompiledType, scopeDepth int, policy errorPolicy) (bool, []errors.Validation) {
	valid := true
	var violations []errors.Validation
	abort := false
	index := 0
	splitWhitespaceSeq(value, func(item string) bool {
		itemValid, itemViolations := r.validateListItemNormalized(item, st.ItemType, index, scopeDepth, policy)
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

func (r *streamRun) validateListItemNormalized(item string, itemType *grammar.CompiledType, index, scopeDepth int, policy errorPolicy) (bool, []errors.Validation) {
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
		if !r.validateUnionValue(item, itemType.MemberTypes, scopeDepth) {
			if policy == errorPolicyReport {
				violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"list item[%d] '%s' does not match any member type of union", index, item))
				return false, violations
			}
			return false, nil
		}
		return true, nil
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
		if err := r.validateQNameContext(item, scopeDepth); err != nil {
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
			if itemViolations := r.validateNotationReference(item, scopeDepth); len(itemViolations) > 0 {
				violations = append(violations, itemViolations...)
				return false, violations
			}
		} else if !r.isValidNotationReference(item, scopeDepth) {
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
				if err := r.validateQNameEnumeration(item, enumFacet, scopeDepth); err != nil {
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
	if st.Restriction != nil {
		baseType := types.GetBuiltinNS(st.Restriction.Base.Namespace, st.Restriction.Base.Local)
		if baseType != nil {
			return baseType.Validate(normalized)
		}
	}
	return nil
}

func (r *streamRun) validateUnionValue(value string, memberTypes []*grammar.CompiledType, scopeDepth int) bool {
	for _, memberType := range memberTypes {
		if r.validateUnionMemberType(value, memberType, scopeDepth) {
			return true
		}
	}
	return false
}

func (r *streamRun) validateUnionMemberType(value string, mt *grammar.CompiledType, scopeDepth int) bool {
	if mt == nil || mt.Original == nil {
		return false
	}

	valid, _ := r.checkSimpleValueInternal(value, mt, scopeDepth, errorPolicySuppress)
	return valid
}

func (r *streamRun) validateNotationReference(value string, scopeDepth int) []errors.Validation {
	notationQName, err := r.parseQNameValue(value, scopeDepth)
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

func (r *streamRun) isValidNotationReference(value string, scopeDepth int) bool {
	notationQName, err := r.parseQNameValue(value, scopeDepth)
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

func (r *streamRun) validateQNameContext(value string, scopeDepth int) error {
	_, err := r.parseQNameValue(value, scopeDepth)
	return err
}

func isQNameType(ct *grammar.CompiledType) bool {
	return ct != nil && ct.IsQNameOrNotationType && !ct.IsNotationType
}
