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
		if r.matchUnionMemberType(normalizedValue, st.MemberTypes, scopeDepth, context) == nil {
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

	if len(r.entityDecls) > 0 {
		if err := r.validateEntityValue(normalizedValue, st); err != nil {
			if policy == errorPolicyReport {
				return false, []errors.Validation{errors.NewValidation(errors.ErrDatatypeInvalid, err.Error(), r.path.String())}
			}
			return false, nil
		}
	}

	return r.validateSimpleTypeFacets(normalizedValue, st, scopeDepth, policy, context)
}

func (r *streamRun) validateSimpleTypeFacets(normalizedValue string, st *grammar.CompiledType, scopeDepth int, policy errorPolicy, context map[string]string) (bool, []errors.Validation) {
	if st == nil || len(st.Facets) == 0 {
		return true, nil
	}

	var validateQNameEnum func(string, *types.Enumeration) error
	if requiresQNameEnumeration(st) {
		validateQNameEnum = func(normalized string, enum *types.Enumeration) error {
			return r.validateQNameEnumerationForType(normalized, enum, st, scopeDepth, context)
		}
	}

	return validateFacets(&facetValidationInput{
		data: &facetValidationData{
			value:  normalizedValue,
			facets: st.Facets,
		},
		typ:      st.Original,
		compiled: st,
		context: &facetValidationContext{
			path: r.path.String,
			callbacks: &facetValidationCallbacks{
				validateQNameEnum: validateQNameEnum,
			},
		},
		policy: policy,
	})
}

func (r *streamRun) checkComplexTypeFacetsWithContext(text string, ct *grammar.CompiledType, scopeDepth int, context map[string]string) []errors.Validation {
	var validateQNameEnum func(string, *types.Enumeration) error
	if ct != nil && requiresQNameEnumeration(ct.SimpleContentType) {
		validateQNameEnum = func(normalized string, enum *types.Enumeration) error {
			return r.validateQNameEnumerationForType(normalized, enum, ct.SimpleContentType, scopeDepth, context)
		}
	}
	return collectComplexTypeFacetViolations(text, ct, r.path.String, validateQNameEnum)
}

func (r *streamRun) checkComplexTypeFacets(text string, ct *grammar.CompiledType, ns map[string]string) []errors.Validation {
	var validateQNameEnum func(string, *types.Enumeration) error
	if ct != nil && requiresQNameEnumeration(ct.SimpleContentType) {
		validateQNameEnum = func(normalized string, enum *types.Enumeration) error {
			return r.validateQNameEnumerationForType(normalized, enum, ct.SimpleContentType, -1, ns)
		}
	}
	return collectComplexTypeFacetViolations(text, ct, r.path.String, validateQNameEnum)
}

func (r *streamRun) validateListValueInternal(value string, st *grammar.CompiledType, scopeDepth int, policy errorPolicy, context map[string]string) (bool, []errors.Validation) {
	valid := true
	var violations []errors.Validation
	abort := false
	index := 0
	splitWhitespaceSeq(value, func(item string) bool {
		itemValid, itemViolations := r.validateListItemNormalized(item, st.ItemType, index, scopeDepth, policy, context, nil)
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
		facets := st.Facets
		if requiresQNameEnumeration(st) {
			nonEnum := make([]types.Facet, 0, len(st.Facets))
			for _, facet := range st.Facets {
				enumFacet, ok := facet.(*types.Enumeration)
				if !ok {
					nonEnum = append(nonEnum, facet)
					continue
				}
				if err := r.validateQNameEnumerationForType(value, enumFacet, st, scopeDepth, context); err != nil {
					if policy == errorPolicySuppress {
						return false, nil
					}
					valid = false
					violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
				}
			}
			facets = nonEnum
		}
		if len(facets) > 0 {
			facetValid, facetViolations := validateFacets(&facetValidationInput{
				data: &facetValidationData{
					value:  value,
					facets: facets,
				},
				typ:      st.Original,
				compiled: st,
				context: &facetValidationContext{
					path: r.path.String,
				},
				policy: policy,
			})
			if !facetValid {
				valid = false
				if policy == errorPolicySuppress {
					return false, nil
				}
				violations = append(violations, facetViolations...)
			}
		}
	}

	if len(violations) > 0 {
		return false, violations
	}
	return valid, nil
}

func (r *streamRun) validateListItemNormalized(item string, itemType *grammar.CompiledType, index, scopeDepth int, policy errorPolicy, context, valueContext map[string]string) (bool, []errors.Validation) {
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

	// Use valueContext if provided, otherwise use context
	nsContext := valueContext
	if nsContext == nil {
		nsContext = context
	}

	if len(itemType.MemberTypes) > 0 {
		if !r.validateUnionValue(item, itemType.MemberTypes, scopeDepth, nsContext) {
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
		if err := r.validateQNameContext(item, scopeDepth, nsContext); err != nil {
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
			if itemViolations := r.validateNotationReference(item, scopeDepth, nsContext); len(itemViolations) > 0 {
				violations = append(violations, itemViolations...)
				return false, violations
			}
		} else if !r.isValidNotationReference(item, scopeDepth, nsContext) {
			return false, nil
		}
	}

	if err := r.validateEntityValue(item, itemType); err != nil {
		if policy == errorPolicyReport {
			violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
				"list item[%d]: %s", index, err.Error()))
			return false, violations
		}
		return false, nil
	}

	if len(itemType.Facets) > 0 {
		var validateQNameEnum func(string, *types.Enumeration) error
		if requiresQNameEnumeration(itemType) {
			validateQNameEnum = func(normalized string, enum *types.Enumeration) error {
				return r.validateQNameEnumerationForType(normalized, enum, itemType, scopeDepth, nsContext)
			}
		}
		makeViolation := func(err error) errors.Validation {
			return errors.NewValidationf(errors.ErrFacetViolation, r.path.String(),
				"list item[%d]: %s", index, err.Error())
		}
		facetValid, facetViolations := validateFacets(&facetValidationInput{
			data: &facetValidationData{
				value:  item,
				facets: itemType.Facets,
			},
			typ:      itemType.Original,
			compiled: itemType,
			context: &facetValidationContext{
				path: r.path.String,
				callbacks: &facetValidationCallbacks{
					validateQNameEnum: validateQNameEnum,
					makeViolation:     makeViolation,
				},
			},
			policy: policy,
		})
		if !facetValid {
			if policy == errorPolicySuppress {
				return false, nil
			}
			violations = append(violations, facetViolations...)
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
	return r.matchUnionMemberType(value, memberTypes, scopeDepth, context) != nil
}

func (r *streamRun) matchUnionMemberType(value string, memberTypes []*grammar.CompiledType, scopeDepth int, context map[string]string) *grammar.CompiledType {
	for _, memberType := range memberTypes {
		if memberType == nil {
			continue
		}
		valid, _ := r.checkSimpleValueInternal(value, memberType, scopeDepth, errorPolicySuppress, context)
		if valid {
			return memberType
		}
	}
	return nil
}

func (r *streamRun) collectIDRefsForValue(value string, ct *grammar.CompiledType, line, column, scopeDepth int, context map[string]string) []errors.Validation {
	return r.collectIDRefsForValueVisited(value, ct, line, column, scopeDepth, context, make(map[*grammar.CompiledType]bool))
}

func (r *streamRun) collectIDRefsForValueVisited(value string, ct *grammar.CompiledType, line, column, scopeDepth int, context map[string]string, visited map[*grammar.CompiledType]bool) []errors.Validation {
	if value == "" || ct == nil {
		return nil
	}
	if visited[ct] {
		return nil
	}
	visited[ct] = true
	defer delete(visited, ct)

	normalized := value
	if ct.Original != nil {
		normalized = types.NormalizeWhiteSpace(value, ct.Original)
	}

	if len(ct.MemberTypes) > 0 {
		if r.idTypeMask(ct) == idTypeNone {
			return nil
		}
		member := r.matchUnionMemberType(normalized, ct.MemberTypes, scopeDepth, context)
		if member == nil {
			return nil
		}
		return r.collectIDRefsForValueVisited(normalized, member, line, column, scopeDepth, context, visited)
	}

	return r.collectIDRefs(normalized, ct, line, column)
}

type entityKind int

const (
	entityNone entityKind = iota
	entitySingle
	entityList
)

func (r *streamRun) validateEntityValue(value string, ct *grammar.CompiledType) error {
	if r == nil || ct == nil {
		return nil
	}
	if len(r.entityDecls) == 0 {
		return nil
	}
	switch entityKindForType(ct) {
	case entitySingle:
		if _, ok := r.entityDecls[value]; !ok {
			return fmt.Errorf("ENTITY value '%s' does not reference a declared entity", value)
		}
	case entityList:
		for item := range types.FieldsXMLWhitespaceSeq(value) {
			if item == "" {
				continue
			}
			if _, ok := r.entityDecls[item]; !ok {
				return fmt.Errorf("ENTITY value '%s' does not reference a declared entity", item)
			}
		}
	}
	return nil
}

func entityKindForType(ct *grammar.CompiledType) entityKind {
	if ct == nil {
		return entityNone
	}
	if ct.Kind == grammar.TypeKindBuiltin && ct.QName.Namespace == types.XSDNamespace {
		switch ct.QName.Local {
		case string(types.TypeNameENTITY):
			return entitySingle
		case string(types.TypeNameENTITIES):
			return entityList
		}
	}
	for base := ct.BaseType; base != nil; base = base.BaseType {
		if base.Kind == grammar.TypeKindBuiltin && base.QName.Namespace == types.XSDNamespace {
			switch base.QName.Local {
			case string(types.TypeNameENTITY):
				return entitySingle
			case string(types.TypeNameENTITIES):
				return entityList
			}
		}
	}
	if ct.ItemType != nil {
		if entityKindForType(ct.ItemType) == entitySingle {
			return entityList
		}
	}
	return entityNone
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
