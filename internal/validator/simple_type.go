package validator

import (
	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// checkSimpleValue validates a string value against a simple type.
// elem is optional and only needed for NOTATION validation (to resolve namespace prefixes).
func (r *validationRun) checkSimpleValue(value string, st *grammar.CompiledType, elem xml.NodeID) []errors.Validation {
	_, violations := r.checkSimpleValueInternal(value, st, elem, true)
	return violations
}

func (r *validationRun) checkSimpleValueInternal(value string, st *grammar.CompiledType, elem xml.NodeID, reportErrors bool) (bool, []errors.Validation) {
	if st == nil || st.Original == nil {
		return true, nil
	}

	if unresolvedName, ok := unresolvedSimpleType(st.Original); ok {
		if reportErrors {
			return false, []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
				"type '%s' is not resolved", unresolvedName)}
		}
		return false, nil
	}

	normalizedValue := types.NormalizeWhiteSpace(value, st.Original)

	if len(st.MemberTypes) > 0 {
		if !r.validateUnionValue(normalizedValue, st.MemberTypes, elem) {
			if reportErrors {
				return false, []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"value '%s' does not match any member type of union", normalizedValue)}
			}
			return false, nil
		}
		return true, nil
	}

	if st.ItemType != nil {
		return r.validateListValueInternal(normalizedValue, st, elem, reportErrors)
	}

	if stInterface, ok := st.Original.(types.SimpleTypeDefinition); ok {
		if err := stInterface.Validate(normalizedValue); err != nil {
			if reportErrors {
				return false, []errors.Validation{errors.NewValidation(errors.ErrDatatypeInvalid, err.Error(), r.path.String())}
			}
			return false, nil
		}
	}

	// special validation for NOTATION types: must reference a declared notation
	if r.isNotationType(st) {
		if reportErrors {
			if violations := r.validateNotationReference(normalizedValue, elem); len(violations) > 0 {
				return false, violations
			}
		} else if !r.isValidNotationReference(normalizedValue, elem) {
			return false, nil
		}
	}

	typedValue := typedValueForFacets(normalizedValue, st.Original, st.Facets)
	var violations []errors.Validation
	for _, facet := range st.Facets {
		if shouldSkipLengthFacet(st, facet) {
			continue
		}
		if err := facet.Validate(typedValue, st.Original); err != nil {
			if !reportErrors {
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

func (r *validationRun) validateListValueInternal(value string, st *grammar.CompiledType, elem xml.NodeID, reportErrors bool) (bool, []errors.Validation) {
	valid := true
	var violations []errors.Validation
	abort := false
	index := 0
	splitWhitespaceSeq(value, func(item string) bool {
		itemValid, itemViolations := r.validateListItemInternal(item, st.ItemType, index, elem, reportErrors)
		index++
		if !itemValid {
			valid = false
			if reportErrors {
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

	// apply list-level facets (e.g., minLength, maxLength, enumeration on the list itself)
	if len(st.Facets) > 0 {
		// for list facets, the value is the list itself (for length facets, it's the number of items)
		typedValue := typedValueForFacets(value, st.Original, st.Facets)
		for _, facet := range st.Facets {
			if shouldSkipLengthFacet(st, facet) {
				continue
			}
			if err := facet.Validate(typedValue, st.Original); err != nil {
				valid = false
				if !reportErrors {
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

func (r *validationRun) validateListItemInternal(item string, itemType *grammar.CompiledType, index int, elem xml.NodeID, reportErrors bool) (bool, []errors.Validation) {
	if itemType == nil || itemType.Original == nil {
		return true, nil
	}

	if unresolvedName, ok := unresolvedSimpleType(itemType.Original); ok {
		if reportErrors {
			return false, []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
				"list item[%d]: type '%s' is not resolved", index, unresolvedName)}
		}
		return false, nil
	}

	normalizedItem := types.NormalizeWhiteSpace(item, itemType.Original)

	var violations []errors.Validation

	if len(itemType.MemberTypes) > 0 {
		if !r.validateUnionValue(normalizedItem, itemType.MemberTypes, elem) {
			if reportErrors {
				violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"list item[%d] '%s' does not match any member type of union", index, normalizedItem))
				return false, violations
			}
			return false, nil
		}
		return true, nil
	}

	if stInterface, ok := itemType.Original.(types.SimpleTypeDefinition); ok {
		if err := stInterface.Validate(normalizedItem); err != nil {
			if reportErrors {
				violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
					"list item[%d]: %s", index, err.Error()))
				return false, violations
			}
			return false, nil
		}
	}

	// special validation for NOTATION types in list items
	if r.isNotationType(itemType) {
		if reportErrors {
			if itemViolations := r.validateNotationReference(normalizedItem, elem); len(itemViolations) > 0 {
				violations = append(violations, itemViolations...)
				return false, violations
			}
		} else if !r.isValidNotationReference(normalizedItem, elem) {
			return false, nil
		}
	}

	typedValue := typedValueForFacets(normalizedItem, itemType.Original, itemType.Facets)
	for _, facet := range itemType.Facets {
		if shouldSkipLengthFacet(itemType, facet) {
			continue
		}
		if err := facet.Validate(typedValue, itemType.Original); err != nil {
			if !reportErrors {
				return false, nil
			}
			violations = append(violations, errors.NewValidationf(errors.ErrFacetViolation, r.path.String(),
				"list item[%d]: %s", index, err.Error()))
		}
	}

	if len(violations) > 0 {
		return false, violations
	}
	return true, nil
}

// checkComplexTypeFacets validates additional facets on complex types with simpleContent.
func (r *validationRun) checkComplexTypeFacets(text string, ct *grammar.CompiledType) []errors.Validation {
	if ct.SimpleContentType == nil || len(ct.Facets) == 0 {
		return nil
	}

	normalizedValue := types.NormalizeWhiteSpace(text, ct.SimpleContentType.Original)

	typedValue := typedValueForFacets(normalizedValue, ct.SimpleContentType.Original, ct.Facets)

	var violations []errors.Validation
	for _, facet := range ct.Facets {
		if shouldSkipLengthFacet(ct.SimpleContentType, facet) {
			continue
		}
		if err := facet.Validate(typedValue, ct.SimpleContentType.Original); err != nil {
			violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), r.path.String()))
		}
	}

	return violations
}

func shouldSkipLengthFacet(ct *grammar.CompiledType, facet facets.Facet) bool {
	if ct == nil || ct.ItemType != nil {
		return false
	}
	if !isLengthFacet(facet) {
		return false
	}
	if ct.IsNotationType {
		return true
	}
	if ct.PrimitiveType != nil {
		if ct.PrimitiveType.QName.Local == string(types.TypeNameQName) ||
			ct.PrimitiveType.QName.Local == string(types.TypeNameNOTATION) {
			return true
		}
	}
	if st, ok := ct.Original.(*types.SimpleType); ok {
		if primitive := st.PrimitiveType(); primitive != nil {
			if primitive.Name().Local == string(types.TypeNameQName) ||
				primitive.Name().Local == string(types.TypeNameNOTATION) {
				return true
			}
		}
	}
	for _, base := range ct.DerivationChain {
		if base.QName.Local == string(types.TypeNameNOTATION) || base.QName.Local == string(types.TypeNameQName) {
			return true
		}
	}
	return false
}

func typedValueForFacets(value string, typ types.Type, facetList []facets.Facet) types.TypedValue {
	if facetsRequireTypedValue(facetList) {
		return facets.TypedValueForFacet(value, typ)
	}
	return &facets.StringTypedValue{Value: value, Typ: typ}
}

func facetsRequireTypedValue(facetList []facets.Facet) bool {
	for _, facet := range facetList {
		switch facet.(type) {
		case *facets.Pattern, *facets.PatternSet, *facets.Enumeration,
			*facets.Length, *facets.MinLength, *facets.MaxLength,
			*facets.TotalDigits, *facets.FractionDigits:
			continue
		default:
			return true
		}
	}
	return false
}

func isLengthFacet(facet facets.Facet) bool {
	switch facet.(type) {
	case *facets.Length, *facets.MinLength, *facets.MaxLength:
		return true
	default:
		return false
	}
}

// validateUnionValue checks if a value matches any member type of a union.
func (r *validationRun) validateUnionValue(value string, memberTypes []*grammar.CompiledType, elem xml.NodeID) bool {
	for _, memberType := range memberTypes {
		if r.validateUnionMemberType(value, memberType, elem) {
			return true
		}
	}
	return false
}

// validateUnionMemberType checks if a value is valid for a single member type.
func (r *validationRun) validateUnionMemberType(value string, mt *grammar.CompiledType, elem xml.NodeID) bool {
	if mt == nil || mt.Original == nil {
		return false
	}

	valid, _ := r.checkSimpleValueInternal(value, mt, elem, false)
	return valid
}

func unresolvedSimpleType(typ types.Type) (types.QName, bool) {
	st, ok := typ.(*types.SimpleType)
	if !ok || st.IsBuiltin() {
		return types.QName{}, false
	}
	if st.Restriction == nil && st.List == nil && st.Union == nil {
		return st.QName, true
	}
	return types.QName{}, false
}

// collectIDRefs tracks ID/IDREF values for later validation and returns violations for duplicate IDs.
// Uses the precomputed IDTypeName field for O(1) lookup instead of traversing the type hierarchy.
func (r *validationRun) collectIDRefs(value string, ct *grammar.CompiledType) []errors.Validation {
	if value == "" || ct == nil {
		return nil
	}

	normalized := value
	if ct.Original != nil {
		normalized = types.NormalizeWhiteSpace(value, ct.Original)
	}

	switch ct.IDTypeName {
	case "ID":
		return r.trackID(normalized, r.path.String())
	case "IDREF":
		r.trackIDREF(normalized, r.path.String())
	case "IDREFS":
		r.trackIDREFS(normalized, r.path.String())
	}
	return nil
}

// trackID records an ID value and checks for duplicates.
func (r *validationRun) trackID(id, path string) []errors.Validation {
	if id == "" {
		return nil
	}
	if r.ids[id] {
		return []errors.Validation{errors.NewValidationf(errors.ErrDuplicateID, path,
			"Duplicate ID value '%s'", id)}
	}
	r.ids[id] = true
	return nil
}

// trackIDREF records an IDREF value for later validation.
func (r *validationRun) trackIDREF(idref, path string) {
	if idref == "" {
		return
	}
	r.idrefs = append(r.idrefs, idrefEntry{ref: idref, path: path})
}

// trackIDREFS records IDREFS values for later validation.
func (r *validationRun) trackIDREFS(idrefs, path string) {
	if idrefs == "" {
		return
	}
	splitWhitespaceSeq(idrefs, func(ref string) bool {
		r.idrefs = append(r.idrefs, idrefEntry{ref: ref, path: path})
		return true
	})
}

// splitWhitespaceSeq yields whitespace-separated fields using XML's ASCII whitespace set.
func splitWhitespaceSeq(s string, yield func(string) bool) {
	start := -1
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			if start >= 0 {
				if !yield(s[start:i]) {
					return
				}
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		_ = yield(s[start:])
	}
}

// checkIDRefs validates that all IDREF values reference valid IDs.
func (r *validationRun) checkIDRefs() []errors.Validation {
	var violations []errors.Validation
	for _, entry := range r.idrefs {
		if !r.ids[entry.ref] {
			violations = append(violations, errors.NewValidationf(errors.ErrIDRefNotFound, entry.path,
				"IDREF '%s' does not reference a valid ID", entry.ref))
		}
	}
	return violations
}

// isNotationType checks if a type is NOTATION or derives from NOTATION.
// This is precomputed during schema compilation for efficiency.
func (r *validationRun) isNotationType(st *grammar.CompiledType) bool {
	return st != nil && st.IsNotationType
}

// validateNotationReference validates that a NOTATION value references a declared notation.
func (r *validationRun) validateNotationReference(value string, elem xml.NodeID) []errors.Validation {
	if elem == xml.InvalidNode {
		// without element context, we can't resolve namespace prefixes
		// this shouldn't happen in practice, but return a violation if it does
		return []errors.Validation{errors.NewValidation(errors.ErrDatatypeInvalid,
			"NOTATION validation requires element context for namespace resolution", r.path.String())}
	}

	notationQName, err := r.parseQNameValue(elem, value)
	if err != nil {
		return []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
			"Invalid NOTATION value '%s': %v", value, err)}
	}

	// check if the notation is declared in the schema
	if r.schema.Notation(notationQName) == nil {
		return []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, r.path.String(),
			"NOTATION value '%s' does not reference a declared notation", value)}
	}

	return nil
}

func (r *validationRun) isValidNotationReference(value string, elem xml.NodeID) bool {
	if elem == xml.InvalidNode {
		return false
	}
	notationQName, err := r.parseQNameValue(elem, value)
	if err != nil {
		return false
	}
	return r.schema.Notation(notationQName) != nil
}
