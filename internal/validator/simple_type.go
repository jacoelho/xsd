package validator

import (
	"strings"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

// checkComplexTypeFacets validates additional facets on complex types with simpleContent.
func (r *validationRun) checkComplexTypeFacets(text string, ct *grammar.CompiledType) []errors.Validation {
	return collectComplexTypeFacetViolations(text, ct, r.path.String(), nil)
}

func collectComplexTypeFacetViolations(
	text string,
	ct *grammar.CompiledType,
	path string,
	validateQNameEnum func(string, *types.Enumeration) error,
) []errors.Validation {
	if ct == nil || ct.SimpleContentType == nil || len(ct.Facets) == 0 {
		return nil
	}

	normalizedValue := types.NormalizeWhiteSpace(text, ct.SimpleContentType.Original)
	if ws, ok := derivedSimpleContentWhiteSpace(ct); ok {
		normalizedValue = types.ApplyWhiteSpace(text, ws)
	}
	_, violations := validateFacets(&facetValidationInput{
		data: &facetValidationData{
			value:  normalizedValue,
			facets: ct.Facets,
		},
		typ:      ct.SimpleContentType.Original,
		compiled: ct.SimpleContentType,
		context: &facetValidationContext{
			path: path,
			callbacks: &facetValidationCallbacks{
				validateQNameEnum: validateQNameEnum,
			},
		},
		policy: errorPolicyReport,
	})
	return violations
}

func derivedSimpleContentWhiteSpace(ct *grammar.CompiledType) (types.WhiteSpace, bool) {
	if ct == nil || ct.Original == nil {
		return 0, false
	}
	complexType, ok := ct.Original.(*types.ComplexType)
	if !ok {
		return 0, false
	}
	sc, ok := complexType.Content().(*types.SimpleContent)
	if !ok || sc.Restriction == nil || sc.Restriction.SimpleType == nil {
		return 0, false
	}
	derived := sc.Restriction.SimpleType
	if !derived.WhiteSpaceExplicit() {
		return 0, false
	}
	return derived.WhiteSpace(), true
}

// shouldSkipLengthFacet returns true when length facets do not apply to a type.
// QName and NOTATION do not have a meaningful character-length value space.
func shouldSkipLengthFacet(ct *grammar.CompiledType, facet types.Facet) bool {
	if ct == nil {
		return false
	}
	if !types.IsLengthFacet(facet) {
		return false
	}
	if ct.ItemType != nil {
		return false
	}
	return ct.IsQNameOrNotationType
}

func typedValueForFacets(value string, typ types.Type, facetList []types.Facet) types.TypedValue {
	if facetsAllowSimpleValue(facetList) {
		return &types.StringTypedValue{Value: value, Typ: typ}
	}
	return types.TypedValueForFacet(value, typ)
}

func facetsAllowSimpleValue(facetList []types.Facet) bool {
	for _, facet := range facetList {
		switch facet.(type) {
		case *types.Pattern, *types.PatternSet, *types.Enumeration,
			*types.Length, *types.MinLength, *types.MaxLength,
			*types.TotalDigits, *types.FractionDigits:
			continue
		default:
			return false
		}
	}
	return true
}

func unresolvedSimpleType(typ types.Type) (types.QName, bool) {
	st, ok := typ.(*types.SimpleType)
	if !ok || !types.IsPlaceholderSimpleType(st) {
		return types.QName{}, false
	}
	return st.QName, true
}

// collectIDRefs tracks ID/IDREF values for later validation and returns violations for duplicate IDs.
// Uses the precomputed IDTypeName field for O(1) lookup instead of traversing the type hierarchy.
func (r *validationRun) collectIDRefs(value string, ct *grammar.CompiledType, line, column int) []errors.Validation {
	if value == "" || ct == nil {
		return nil
	}

	normalized := value
	if ct.Original != nil {
		normalized = types.NormalizeWhiteSpace(value, ct.Original)
	}

	switch ct.IDTypeName {
	case string(types.TypeNameID):
		return r.trackID(normalized, r.path.String(), line, column)
	case string(types.TypeNameIDREF):
		r.trackIDREF(normalized, r.path.String(), line, column)
	case string(types.TypeNameIDREFS):
		r.trackIDREFS(normalized, r.path.String(), line, column)
	}
	return nil
}

// trackID records an ID value and checks for duplicates.
func (r *validationRun) trackID(id, path string, line, column int) []errors.Validation {
	if id == "" {
		return nil
	}
	if r.ids[id] {
		violation := errors.NewValidationf(errors.ErrDuplicateID, path,
			"Duplicate ID value '%s'", id)
		if line > 0 && column > 0 {
			violation.Line = line
			violation.Column = column
		}
		return []errors.Validation{violation}
	}
	r.ids[strings.Clone(id)] = true
	return nil
}

// trackIDREF records an IDREF value for later schemacheck.
func (r *validationRun) trackIDREF(idref, path string, line, column int) {
	if idref == "" {
		return
	}
	r.idrefs = append(r.idrefs, idrefEntry{
		ref:    strings.Clone(idref),
		path:   path,
		line:   line,
		column: column,
	})
}

// trackIDREFS records IDREFS values for later schemacheck.
func (r *validationRun) trackIDREFS(idrefs, path string, line, column int) {
	if idrefs == "" {
		return
	}
	splitWhitespaceSeq(idrefs, func(ref string) bool {
		if ref != "" {
			r.idrefs = append(r.idrefs, idrefEntry{
				ref:    strings.Clone(ref),
				path:   path,
				line:   line,
				column: column,
			})
		}
		return true
	})
}

// splitWhitespaceSeq yields whitespace-separated fields using XML's ASCII whitespace set.
func splitWhitespaceSeq(s string, yield func(string) bool) {
	for field := range types.FieldsXMLWhitespaceSeq(s) {
		if !yield(field) {
			return
		}
	}
}

// checkIDRefs validates that all IDREF values reference valid IDs.
func (r *validationRun) checkIDRefs() []errors.Validation {
	var violations []errors.Validation
	for _, entry := range r.idrefs {
		if !r.ids[entry.ref] {
			violation := errors.NewValidationf(errors.ErrIDRefNotFound, entry.path,
				"IDREF '%s' does not reference a valid ID", entry.ref)
			if entry.line > 0 && entry.column > 0 {
				violation.Line = entry.line
				violation.Column = entry.column
			}
			violations = append(violations, violation)
		}
	}
	return violations
}

// isNotationType checks if a type is NOTATION or derives from NOTATION.
// This is precomputed during schema compilation for efficiency.
func (r *validationRun) isNotationType(st *grammar.CompiledType) bool {
	return st != nil && st.IsNotationType
}
