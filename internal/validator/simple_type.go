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
func (r *validationRun) checkSimpleValue(value string, st *grammar.CompiledType, path string, elem xml.Element) []errors.Validation {
	if st == nil || st.Original == nil {
		return nil
	}

	if unresolvedName, ok := unresolvedSimpleType(st.Original); ok {
		return []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, path,
			"type '%s' is not resolved", unresolvedName)}
	}

	normalizedValue := types.NormalizeWhiteSpace(value, st.Original)

	if len(st.MemberTypes) > 0 {
		if !r.validateUnionValue(normalizedValue, st.MemberTypes, elem) {
			return []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, path,
				"value '%s' does not match any member type of union", normalizedValue)}
		}
		return nil
	}

	if st.ItemType != nil {
		return r.validateListValue(normalizedValue, st, path, elem)
	}

	if stInterface, ok := st.Original.(types.SimpleTypeDefinition); ok {
		if err := stInterface.Validate(normalizedValue); err != nil {
			return []errors.Validation{errors.NewValidation(errors.ErrDatatypeInvalid, err.Error(), path)}
		}
	}

	// Special validation for NOTATION types: must reference a declared notation
	if r.isNotationType(st) {
		if violations := r.validateNotationReference(normalizedValue, elem, path); len(violations) > 0 {
			return violations
		}
	}

	typedValue := facets.TypedValueForFacet(normalizedValue, st.Original)
	var violations []errors.Validation
	for _, facet := range st.Facets {
		if shouldSkipLengthFacet(st, facet) {
			continue
		}
		if err := facet.Validate(typedValue, st.Original); err != nil {
			violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), path))
		}
	}

	return violations
}

// validateListValue validates a list value by validating each item against the item type.
func (r *validationRun) validateListValue(value string, st *grammar.CompiledType, path string, elem xml.Element) []errors.Validation {
	items := splitWhitespace(value)

	var violations []errors.Validation

	for i, item := range items {
		itemViolations := r.validateListItem(item, st.ItemType, path, i, elem)
		violations = append(violations, itemViolations...)
	}

	// Apply list-level facets (e.g., minLength, maxLength, enumeration on the list itself)
	if len(st.Facets) > 0 {
		// For list facets, the value is the list itself (for length facets, it's the number of items)
		typedValue := facets.TypedValueForFacet(value, st.Original)
		for _, facet := range st.Facets {
			if shouldSkipLengthFacet(st, facet) {
				continue
			}
			if err := facet.Validate(typedValue, st.Original); err != nil {
				violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), path))
			}
		}
	}

	return violations
}

// validateListItem validates a single item in a list against the item type.
func (r *validationRun) validateListItem(item string, itemType *grammar.CompiledType, path string, index int, elem xml.Element) []errors.Validation {
	if itemType == nil || itemType.Original == nil {
		return nil
	}

	if unresolvedName, ok := unresolvedSimpleType(itemType.Original); ok {
		return []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, path,
			"list item[%d]: type '%s' is not resolved", index, unresolvedName)}
	}

	normalizedItem := types.NormalizeWhiteSpace(item, itemType.Original)

	var violations []errors.Validation

	if len(itemType.MemberTypes) > 0 {
		if !r.validateUnionValue(normalizedItem, itemType.MemberTypes, elem) {
			violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, path,
				"list item[%d] '%s' does not match any member type of union", index, normalizedItem))
		}
		return violations
	}

	if stInterface, ok := itemType.Original.(types.SimpleTypeDefinition); ok {
		if err := stInterface.Validate(normalizedItem); err != nil {
			violations = append(violations, errors.NewValidationf(errors.ErrDatatypeInvalid, path,
				"list item[%d]: %s", index, err.Error()))
			return violations
		}
	}

	// Special validation for NOTATION types in list items
	if r.isNotationType(itemType) {
		if itemViolations := r.validateNotationReference(normalizedItem, elem, path); len(itemViolations) > 0 {
			violations = append(violations, itemViolations...)
			return violations
		}
	}

	typedValue := facets.TypedValueForFacet(normalizedItem, itemType.Original)
	for _, facet := range itemType.Facets {
		if shouldSkipLengthFacet(itemType, facet) {
			continue
		}
		if err := facet.Validate(typedValue, itemType.Original); err != nil {
			violations = append(violations, errors.NewValidationf(errors.ErrFacetViolation, path,
				"list item[%d]: %s", index, err.Error()))
		}
	}

	return violations
}

// checkComplexTypeFacets validates additional facets on complex types with simpleContent.
func (r *validationRun) checkComplexTypeFacets(text string, ct *grammar.CompiledType, path string) []errors.Validation {
	if ct.SimpleContentType == nil || len(ct.Facets) == 0 {
		return nil
	}

	normalizedValue := types.NormalizeWhiteSpace(text, ct.SimpleContentType.Original)

	typedValue := facets.TypedValueForFacet(normalizedValue, ct.SimpleContentType.Original)

	var violations []errors.Validation
	for _, facet := range ct.Facets {
		if shouldSkipLengthFacet(ct.SimpleContentType, facet) {
			continue
		}
		if err := facet.Validate(typedValue, ct.SimpleContentType.Original); err != nil {
			violations = append(violations, errors.NewValidation(errors.ErrFacetViolation, err.Error(), path))
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

func isLengthFacet(facet facets.Facet) bool {
	switch facet.(type) {
	case *facets.Length, *facets.MinLength, *facets.MaxLength:
		return true
	default:
		return false
	}
}

// validateUnionValue checks if a value matches any member type of a union.
func (r *validationRun) validateUnionValue(value string, memberTypes []*grammar.CompiledType, elem xml.Element) bool {
	for _, memberType := range memberTypes {
		if r.validateUnionMemberType(value, memberType, elem) {
			return true
		}
	}
	return false
}

// validateUnionMemberType checks if a value is valid for a single member type.
func (r *validationRun) validateUnionMemberType(value string, mt *grammar.CompiledType, elem xml.Element) bool {
	if mt == nil || mt.Original == nil {
		return false
	}

	violations := r.checkSimpleValue(value, mt, "", elem)
	return len(violations) == 0
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
func (r *validationRun) collectIDRefs(value string, ct *grammar.CompiledType, path string) []errors.Validation {
	if value == "" || ct == nil {
		return nil
	}

	normalized := value
	if ct.Original != nil {
		normalized = types.NormalizeWhiteSpace(value, ct.Original)
	}

	switch ct.IDTypeName {
	case "ID":
		return r.trackID(normalized, path)
	case "IDREF":
		r.trackIDREF(normalized, path)
	case "IDREFS":
		r.trackIDREFS(normalized, path)
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
	for _, ref := range splitWhitespace(idrefs) {
		r.idrefs = append(r.idrefs, idrefEntry{ref: ref, path: path})
	}
}

// splitWhitespace splits a string by whitespace.
func splitWhitespace(s string) []string {
	var result []string
	start := -1
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if start >= 0 {
				result = append(result, s[start:i])
				start = -1
			}
		} else {
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		result = append(result, s[start:])
	}
	return result
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
func (r *validationRun) validateNotationReference(value string, elem xml.Element, path string) []errors.Validation {
	if elem == nil {
		// Without element context, we can't resolve namespace prefixes
		// This shouldn't happen in practice, but return a violation if it does
		return []errors.Validation{errors.NewValidation(errors.ErrDatatypeInvalid,
			"NOTATION validation requires element context for namespace resolution", path)}
	}

	notationQName, err := r.parseQNameValue(elem, value)
	if err != nil {
		return []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, path,
			"Invalid NOTATION value '%s': %v", value, err)}
	}

	// Check if the notation is declared in the schema
	if r.schema.Notation(notationQName) == nil {
		return []errors.Validation{errors.NewValidationf(errors.ErrDatatypeInvalid, path,
			"NOTATION value '%s' does not reference a declared notation", value)}
	}

	return nil
}
