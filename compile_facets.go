package xsd

import (
	"fmt"
	"slices"
	"strconv"
)

func parseWhitespaceChecked(v string) (whitespaceMode, bool) {
	switch v {
	case "preserve":
		return whitespacePreserve, true
	case "replace":
		return whitespaceReplace, true
	case "collapse":
		return whitespaceCollapse, true
	default:
		return whitespacePreserve, false
	}
}

func validWhitespaceRestriction(base, next whitespaceMode) bool {
	if base == whitespaceCollapse {
		return next == whitespaceCollapse
	}
	if base == whitespaceReplace {
		return next != whitespacePreserve
	}
	return true
}

func facetAllowedForType(st simpleType, name string) bool {
	if st.Variety == varietyAtomic {
		return true
	}
	if st.Variety == varietyUnion {
		return name == "pattern" || name == "enumeration"
	}
	switch name {
	case "minInclusive", "maxInclusive", "minExclusive", "maxExclusive", "totalDigits", "fractionDigits":
		return false
	default:
		return true
	}
}

func (c *compiler) compileFacets(parent *rawNode, st *simpleType, base simpleTypeID) error {
	baseFacets := c.rt.SimpleTypes[base].Facets
	state := compiledFacetState{
		inheritedEnumeration: slices.Clone(st.Facets.Enumeration),
	}
	if err := c.compileFacetChildren(parent, st, base, &state); err != nil {
		return err
	}
	state.apply(st)
	return validateCompiledFacets(*st, baseFacets, state.orderedStep)
}

type compiledFacetState struct {
	inheritedEnumeration  []compiledLiteral
	restrictedEnumeration []compiledLiteral
	stepPatterns          []pattern
	orderedStep           orderedFacetStep
	sawEnumeration        bool
}

func (s *compiledFacetState) apply(st *simpleType) {
	if s.sawEnumeration {
		st.Facets.Enumeration = s.restrictedEnumeration
	} else {
		st.Facets.Enumeration = s.inheritedEnumeration
	}
	if len(s.stepPatterns) != 0 {
		st.Facets.Patterns = append(st.Facets.Patterns, patternGroup{Patterns: s.stepPatterns})
	}
}

func (c *compiler) compileFacetChildren(parent *rawNode, st *simpleType, base simpleTypeID, state *compiledFacetState) error {
	for _, child := range parent.xsContentChildren() {
		if child.Name.Local == "simpleType" {
			continue
		}
		if !facetAllowedForType(*st, child.Name.Local) {
			return schemaCompile(ErrSchemaFacet, "facet "+child.Name.Local+" is not allowed")
		}
		value, hasValue := child.attr("value")
		switch child.Name.Local {
		case "length", "minLength", "maxLength", "totalDigits", "fractionDigits":
			if err := compileSizeFacet(st, child.Name.Local, value, hasValue); err != nil {
				return err
			}
		case "minInclusive", "maxInclusive", "minExclusive", "maxExclusive":
			if err := c.compileBoundFacet(st, base, child, value, &state.orderedStep); err != nil {
				return err
			}
		case "enumeration":
			lit, err := c.compileLiteral(base, value, c.schemaQNameResolver(child))
			if err != nil {
				return err
			}
			state.restrictedEnumeration = append(state.restrictedEnumeration, lit)
			state.sawEnumeration = true
		case "pattern":
			p, err := compilePattern(value)
			if err != nil {
				return err
			}
			state.stepPatterns = append(state.stepPatterns, p)
		case "whiteSpace":
			if err := c.compileWhitespaceFacet(st, base, value, hasValue); err != nil {
				return err
			}
		default:
			if child.Name.Space == xsdNamespaceURI {
				return schemaCompile(ErrSchemaFacet, "unsupported facet "+child.Name.Local)
			}
		}
	}
	return nil
}

func compileSizeFacet(st *simpleType, name, value string, hasValue bool) error {
	if !hasValue {
		return schemaCompile(ErrSchemaFacet, name+" missing value")
	}
	n, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return schemaCompile(ErrSchemaFacet, "invalid "+name+" facet "+value)
	}
	v := uint32(n)
	if name == "totalDigits" && v == 0 {
		return schemaCompile(ErrSchemaFacet, "totalDigits must be positive")
	}
	switch name {
	case "length":
		st.Facets.Length = &v
	case "minLength":
		st.Facets.MinLength = &v
	case "maxLength":
		st.Facets.MaxLength = &v
	case "totalDigits":
		st.Facets.TotalDigits = &v
	case "fractionDigits":
		st.Facets.FractionDigits = &v
	}
	return nil
}

func (c *compiler) compileBoundFacet(st *simpleType, base simpleTypeID, child *rawNode, value string, step *orderedFacetStep) error {
	lit, err := c.compileLiteral(base, value, c.schemaQNameResolver(child))
	if err != nil {
		return err
	}
	switch child.Name.Local {
	case "minInclusive":
		st.Facets.MinInclusive = &lit
		step.minInclusive = true
	case "maxInclusive":
		st.Facets.MaxInclusive = &lit
		step.maxInclusive = true
	case "minExclusive":
		st.Facets.MinExclusive = &lit
		step.minExclusive = true
	case "maxExclusive":
		st.Facets.MaxExclusive = &lit
		step.maxExclusive = true
	}
	return nil
}

func (c *compiler) compileWhitespaceFacet(st *simpleType, base simpleTypeID, value string, hasValue bool) error {
	mode, ok := parseWhitespaceChecked(value)
	if !hasValue || !ok {
		return schemaCompile(ErrSchemaFacet, "invalid whiteSpace facet "+value)
	}
	if !validWhitespaceRestriction(c.rt.SimpleTypes[base].Whitespace, mode) {
		return schemaCompile(ErrSchemaFacet, "whiteSpace cannot loosen base whiteSpace")
	}
	st.Whitespace = mode
	return nil
}

func validateCompiledFacets(st simpleType, baseFacets facetSet, orderedStep orderedFacetStep) error {
	if st.Facets.Length != nil && st.Facets.MinLength != nil {
		if st.Variety == varietyList {
			if *st.Facets.Length < *st.Facets.MinLength {
				return schemaCompile(ErrSchemaFacet, "length cannot be less than minLength")
			}
		} else if *st.Facets.Length != *st.Facets.MinLength {
			return schemaCompile(ErrSchemaFacet, "length must equal minLength")
		}
	}
	if st.Facets.Length != nil && st.Facets.MaxLength != nil && *st.Facets.Length != *st.Facets.MaxLength {
		return schemaCompile(ErrSchemaFacet, "length must equal maxLength")
	}
	if st.Facets.MinLength != nil && st.Facets.MaxLength != nil && *st.Facets.MinLength > *st.Facets.MaxLength {
		return schemaCompile(ErrSchemaFacet, "minLength cannot exceed maxLength")
	}
	if baseFacets.MinLength != nil && st.Facets.MinLength != nil && *st.Facets.MinLength < *baseFacets.MinLength {
		return schemaCompile(ErrSchemaFacet, "minLength cannot be less than base minLength")
	}
	if baseFacets.MaxLength != nil && st.Facets.MaxLength != nil && *st.Facets.MaxLength > *baseFacets.MaxLength {
		return schemaCompile(ErrSchemaFacet, "maxLength cannot exceed base maxLength")
	}
	if st.Facets.TotalDigits != nil && st.Facets.FractionDigits != nil && *st.Facets.FractionDigits > *st.Facets.TotalDigits {
		return schemaCompile(ErrSchemaFacet, "fractionDigits cannot exceed totalDigits")
	}
	if err := validateOrderedFacetStep(orderedStep); err != nil {
		return schemaCompile(ErrSchemaFacet, err.Error())
	}
	return validatePrimitiveFacetRestrictions(st, baseFacets, orderedStep)
}

func validatePrimitiveFacetRestrictions(st simpleType, baseFacets facetSet, orderedStep orderedFacetStep) error {
	if st.Primitive == primDecimal {
		if err := validateDecimalFacetRestriction(st.Facets, baseFacets, orderedStep); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
		if err := validateDecimalFacetBounds(st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	}
	if st.Primitive == primFloat || st.Primitive == primDouble {
		if err := validateFloatFacetBounds(st.Primitive, st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	}
	if st.Primitive == primDuration {
		if err := validateDurationFacetBounds(st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	}
	if st.Primitive == primGDay {
		if err := validateGDayFacetBounds(st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	}
	if st.Primitive == primGMonthDay {
		if err := validateGMonthDayFacetBounds(st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	}
	if st.Primitive == primGMonth {
		if err := validateGMonthFacetBounds(st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	}
	if st.Primitive == primGYearMonth {
		if err := validateGYearMonthFacetBounds(st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	}
	if st.Primitive == primGYear {
		if err := validateGYearFacetBounds(st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	}
	if st.Primitive == primDate || st.Primitive == primDateTime || st.Primitive == primTime {
		if st.Primitive == primTime {
			if err := validateTimeFacetRestriction(st.Facets, baseFacets, orderedStep); err != nil {
				return schemaCompile(ErrSchemaFacet, err.Error())
			}
		}
		if err := validateTemporalFacetBounds(st.Primitive, st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	}
	return nil
}

type orderedFacetStep struct {
	minInclusive bool
	maxInclusive bool
	minExclusive bool
	maxExclusive bool
}

func validateOrderedFacetStep(step orderedFacetStep) error {
	if step.minInclusive && step.minExclusive {
		return fmt.Errorf("minInclusive and minExclusive cannot both be specified")
	}
	if step.maxInclusive && step.maxExclusive {
		return fmt.Errorf("maxInclusive and maxExclusive cannot both be specified")
	}
	return nil
}

func validateDecimalFacetRestriction(f, base facetSet, step orderedFacetStep) error {
	if base.TotalDigits != nil && f.TotalDigits != nil && *f.TotalDigits > *base.TotalDigits {
		return fmt.Errorf("totalDigits cannot exceed base totalDigits")
	}
	if base.FractionDigits != nil && f.FractionDigits != nil && *f.FractionDigits > *base.FractionDigits {
		return fmt.Errorf("fractionDigits cannot exceed base fractionDigits")
	}
	baseLower := decimalLowerBound(base)
	baseUpper := decimalUpperBound(base)
	if step.minInclusive {
		if err := validateDecimalLowerRestriction("minInclusive", f.MinInclusive, false, baseLower); err != nil {
			return err
		}
	}
	if step.minExclusive {
		if err := validateDecimalLowerRestriction("minExclusive", f.MinExclusive, true, baseLower); err != nil {
			return err
		}
	}
	if step.maxInclusive {
		if err := validateDecimalUpperRestriction("maxInclusive", f.MaxInclusive, false, baseUpper); err != nil {
			return err
		}
	}
	if step.maxExclusive {
		if err := validateDecimalUpperRestriction("maxExclusive", f.MaxExclusive, true, baseUpper); err != nil {
			return err
		}
	}
	return nil
}

func validateDecimalLowerRestriction(name string, lit *compiledLiteral, exclusive bool, base decimalBound) error {
	if lit == nil || !base.ok {
		return nil
	}
	cmp := compareCanonicalDecimal(lit.Canonical, base.value)
	if cmp < 0 || cmp == 0 && !exclusive && base.exclusive {
		return fmt.Errorf("%s cannot be less than base lower bound", name)
	}
	return nil
}

func validateDecimalUpperRestriction(name string, lit *compiledLiteral, exclusive bool, base decimalBound) error {
	if lit == nil || !base.ok {
		return nil
	}
	cmp := compareCanonicalDecimal(lit.Canonical, base.value)
	if cmp > 0 || cmp == 0 && !exclusive && base.exclusive {
		return fmt.Errorf("%s cannot exceed base upper bound", name)
	}
	return nil
}

func validateDecimalFacetBounds(f facetSet) error {
	lower := decimalLowerBound(f)
	upper := decimalUpperBound(f)
	if !lower.ok || !upper.ok {
		return nil
	}
	cmp := compareCanonicalDecimal(lower.value, upper.value)
	if cmp > 0 || cmp == 0 && (lower.exclusive || upper.exclusive) {
		return fmt.Errorf("decimal lower bound cannot exceed upper bound")
	}
	return nil
}

type decimalBound struct {
	value     string
	exclusive bool
	ok        bool
}

func decimalLowerBound(f facetSet) decimalBound {
	if f.MinInclusive != nil {
		out := decimalBound{value: f.MinInclusive.Canonical, ok: true}
		if f.MinExclusive != nil {
			other := decimalBound{value: f.MinExclusive.Canonical, exclusive: true, ok: true}
			if compareDecimalLowerBound(other, out) > 0 {
				return other
			}
		}
		return out
	}
	if f.MinExclusive != nil {
		return decimalBound{value: f.MinExclusive.Canonical, exclusive: true, ok: true}
	}
	return decimalBound{}
}

func decimalUpperBound(f facetSet) decimalBound {
	if f.MaxInclusive != nil {
		out := decimalBound{value: f.MaxInclusive.Canonical, ok: true}
		if f.MaxExclusive != nil {
			other := decimalBound{value: f.MaxExclusive.Canonical, exclusive: true, ok: true}
			if compareDecimalUpperBound(other, out) < 0 {
				return other
			}
		}
		return out
	}
	if f.MaxExclusive != nil {
		return decimalBound{value: f.MaxExclusive.Canonical, exclusive: true, ok: true}
	}
	return decimalBound{}
}

func compareDecimalLowerBound(a, b decimalBound) int {
	cmp := compareCanonicalDecimal(a.value, b.value)
	if cmp != 0 {
		return cmp
	}
	if a.exclusive == b.exclusive {
		return 0
	}
	if a.exclusive {
		return 1
	}
	return -1
}

func compareDecimalUpperBound(a, b decimalBound) int {
	cmp := compareCanonicalDecimal(a.value, b.value)
	if cmp != 0 {
		return cmp
	}
	if a.exclusive == b.exclusive {
		return 0
	}
	if a.exclusive {
		return -1
	}
	return 1
}

func (c *compiler) compileLiteral(base simpleTypeID, lexical string, resolve qnameResolver) (compiledLiteral, error) {
	canon, err := validateSimpleValue(&c.rt, base, lexical, resolve)
	if err != nil {
		if IsUnsupported(err) {
			return compiledLiteral{}, err
		}
		return compiledLiteral{}, schemaCompile(ErrSchemaFacet, "invalid facet value "+lexical)
	}
	return compiledLiteral{Lexical: lexical, Canonical: canon}, nil
}
