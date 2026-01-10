package validator

import (
	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

// checkComplexTypeFacets validates additional facets on complex types with simpleContent.
func (r *validationRun) checkComplexTypeFacets(text string, ct *grammar.CompiledType) []errors.Validation {
	if ct == nil || ct.SimpleContentType == nil || len(ct.Facets) == 0 {
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

func shouldSkipLengthFacet(ct *grammar.CompiledType, facet types.Facet) bool {
	if ct == nil || ct.ItemType != nil {
		return false
	}
	if !isLengthFacet(facet) {
		return false
	}
	if ct.IsNotationType {
		return true
	}
	if ct.PrimitiveType != nil && isQNameOrNotation(ct.PrimitiveType.QName) {
		return true
	}
	if st, ok := ct.Original.(*types.SimpleType); ok {
		if primitive := st.PrimitiveType(); primitive != nil && isQNameOrNotation(primitive.Name()) {
			return true
		}
	}
	for _, base := range ct.DerivationChain {
		if isQNameOrNotation(base.QName) {
			return true
		}
	}
	return false
}

func isQNameOrNotation(name types.QName) bool {
	return name.Local == string(types.TypeNameQName) || name.Local == string(types.TypeNameNOTATION)
}

func typedValueForFacets(value string, typ types.Type, facetList []types.Facet) types.TypedValue {
	if facetsRequireTypedValue(facetList) {
		return types.TypedValueForFacet(value, typ)
	}
	return &types.StringTypedValue{Value: value, Typ: typ}
}

func facetsRequireTypedValue(facetList []types.Facet) bool {
	for _, facet := range facetList {
		switch facet.(type) {
		case *types.Pattern, *types.PatternSet, *types.Enumeration,
			*types.Length, *types.MinLength, *types.MaxLength,
			*types.TotalDigits, *types.FractionDigits:
			continue
		default:
			return true
		}
	}
	return false
}

func isLengthFacet(facet types.Facet) bool {
	switch facet.(type) {
	case *types.Length, *types.MinLength, *types.MaxLength:
		return true
	default:
		return false
	}
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

// trackIDREF records an IDREF value for later schemacheck.
func (r *validationRun) trackIDREF(idref, path string) {
	if idref == "" {
		return
	}
	r.idrefs = append(r.idrefs, idrefEntry{ref: idref, path: path})
}

// trackIDREFS records IDREFS values for later schemacheck.
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
