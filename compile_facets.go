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
	switch st.Variety {
	case varietyAtomic:
		return atomicFacetAllowed(st.Primitive, name)
	case varietyList:
		switch name {
		case "length", "minLength", "maxLength", "pattern", "enumeration", "whiteSpace":
			return true
		default:
			return false
		}
	case varietyUnion:
		return name == "pattern" || name == "enumeration"
	}
	return false
}

func atomicFacetAllowed(kind primitiveKind, name string) bool {
	switch name {
	case "pattern", "enumeration", "whiteSpace":
		return true
	case "length", "minLength", "maxLength":
		return primitiveHasLengthFacet(kind)
	case "minInclusive", "maxInclusive", "minExclusive", "maxExclusive":
		return primitiveHasOrderFacet(kind)
	case "totalDigits", "fractionDigits":
		return kind == primDecimal
	}
	return false
}

func primitiveHasLengthFacet(kind primitiveKind) bool {
	switch kind {
	case primString, primAnyURI, primHexBinary, primBase64Binary, primQName, primNotation:
		return true
	default:
		return false
	}
}

func primitiveHasOrderFacet(kind primitiveKind) bool {
	switch kind {
	case primDecimal, primFloat, primDouble, primDuration, primDateTime, primTime, primDate,
		primGYearMonth, primGYear, primGMonthDay, primGDay, primGMonth:
		return true
	default:
		return false
	}
}

func (c *compiler) compileFacets(parent *rawNode, st *simpleType, base, literalType simpleTypeID) error {
	baseFacets := c.rt.SimpleTypes[base].Facets
	state := compiledFacetState{
		inheritedEnumeration: slices.Clone(st.Facets.Enumeration),
	}
	if err := c.compileFacetChildren(parent, st, base, literalType, &state); err != nil {
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

func (c *compiler) compileFacetChildren(parent *rawNode, st *simpleType, base, literalType simpleTypeID, state *compiledFacetState) error {
	for _, child := range parent.xsContentChildren() {
		if child.Name.Local == "simpleType" {
			continue
		}
		switch child.Name.Local {
		case "length", "minLength", "maxLength", "totalDigits", "fractionDigits":
			value, err := checkedFacetValue(*st, child)
			if err != nil {
				return err
			}
			if err := compileSizeFacet(st, child.Name.Local, value); err != nil {
				return err
			}
		case "minInclusive", "maxInclusive", "minExclusive", "maxExclusive":
			value, err := checkedFacetValue(*st, child)
			if err != nil {
				return err
			}
			if err := c.compileBoundFacet(st, base, child, value, &state.orderedStep); err != nil {
				return err
			}
		case "enumeration":
			value, err := checkedFacetValue(*st, child)
			if err != nil {
				return err
			}
			lit, err := c.compileLiteral(literalType, value, c.schemaQNameResolver(child))
			if err != nil {
				return err
			}
			state.restrictedEnumeration = append(state.restrictedEnumeration, lit)
			state.sawEnumeration = true
		case "pattern":
			value, err := checkedFacetValue(*st, child)
			if err != nil {
				return err
			}
			p, err := c.compilePattern(value)
			if err != nil {
				return err
			}
			state.stepPatterns = append(state.stepPatterns, p)
		case "whiteSpace":
			value, err := checkedFacetValue(*st, child)
			if err != nil {
				return err
			}
			if err := c.compileWhitespaceFacet(st, base, value); err != nil {
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

func checkedFacetValue(st simpleType, n *rawNode) (string, error) {
	value, err := requiredFacetValue(n)
	if err != nil {
		return "", err
	}
	if !facetAllowedForType(st, n.Name.Local) {
		return "", schemaCompile(ErrSchemaFacet, "facet "+n.Name.Local+" is not allowed")
	}
	return value, nil
}

func requiredFacetValue(n *rawNode) (string, error) {
	value, ok := n.attr("value")
	if !ok {
		return "", schemaCompile(ErrSchemaFacet, n.Name.Local+" missing value")
	}
	return value, nil
}

func compileSizeFacet(st *simpleType, name, value string) error {
	n, err := parseSizeFacetInteger(value)
	if err != nil {
		return schemaCompile(ErrSchemaFacet, "invalid "+name+" facet "+value)
	}
	if name == "totalDigits" && n == 0 {
		return schemaCompile(ErrSchemaFacet, "totalDigits must be positive")
	}
	v := uint32(n)
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

func parseSizeFacetInteger(value string) (uint64, error) {
	if value == "" {
		return 0, strconv.ErrSyntax
	}
	start := 0
	negative := false
	if value[0] == '+' || value[0] == '-' {
		negative = value[0] == '-'
		start = 1
	}
	if start == len(value) {
		return 0, strconv.ErrSyntax
	}
	digitStart := start
	for digitStart < len(value) && value[digitStart] == '0' {
		digitStart++
	}
	if negative && digitStart != len(value) {
		return 0, strconv.ErrSyntax
	}
	if digitStart == len(value) {
		return 0, nil
	}
	for i := start; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return 0, strconv.ErrSyntax
		}
	}
	return strconv.ParseUint(value[digitStart:], 10, 32)
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

func (c *compiler) compileWhitespaceFacet(st *simpleType, base simpleTypeID, value string) error {
	mode, ok := parseWhitespaceChecked(value)
	if !ok {
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
	cmp := compareDecimalValues(literalDecimal(lit), base.value)
	if cmp < 0 || cmp == 0 && !exclusive && base.exclusive {
		return fmt.Errorf("%s cannot be less than base lower bound", name)
	}
	return nil
}

func validateDecimalUpperRestriction(name string, lit *compiledLiteral, exclusive bool, base decimalBound) error {
	if lit == nil || !base.ok {
		return nil
	}
	cmp := compareDecimalValues(literalDecimal(lit), base.value)
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
	cmp := compareDecimalValues(lower.value, upper.value)
	if cmp > 0 || cmp == 0 && (lower.exclusive || upper.exclusive) {
		return fmt.Errorf("decimal lower bound cannot exceed upper bound")
	}
	return nil
}

type decimalBound struct {
	value     decimalValue
	exclusive bool
	ok        bool
}

func decimalLowerBound(f facetSet) decimalBound {
	if f.MinInclusive != nil {
		out := decimalBound{value: literalDecimal(f.MinInclusive), ok: true}
		if f.MinExclusive != nil {
			other := decimalBound{value: literalDecimal(f.MinExclusive), exclusive: true, ok: true}
			if compareDecimalLowerBound(other, out) > 0 {
				return other
			}
		}
		return out
	}
	if f.MinExclusive != nil {
		return decimalBound{value: literalDecimal(f.MinExclusive), exclusive: true, ok: true}
	}
	return decimalBound{}
}

func decimalUpperBound(f facetSet) decimalBound {
	if f.MaxInclusive != nil {
		out := decimalBound{value: literalDecimal(f.MaxInclusive), ok: true}
		if f.MaxExclusive != nil {
			other := decimalBound{value: literalDecimal(f.MaxExclusive), exclusive: true, ok: true}
			if compareDecimalUpperBound(other, out) < 0 {
				return other
			}
		}
		return out
	}
	if f.MaxExclusive != nil {
		return decimalBound{value: literalDecimal(f.MaxExclusive), exclusive: true, ok: true}
	}
	return decimalBound{}
}

func compareDecimalLowerBound(a, b decimalBound) int {
	cmp := compareDecimalValues(a.value, b.value)
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
	cmp := compareDecimalValues(a.value, b.value)
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
	value, err := validateSimpleValueInfo(&c.rt, base, lexical, resolve)
	if err != nil {
		if IsUnsupported(err) {
			return compiledLiteral{}, err
		}
		return compiledLiteral{}, schemaCompile(ErrSchemaFacet, "invalid facet value "+lexical)
	}
	return compiledLiteral{
		Lexical:   lexical,
		Canonical: value.Canonical,
		Actual:    literalActualValue(&c.rt, base, lexical, value.Canonical),
	}, nil
}

func literalActualValue(rt *runtimeSchema, id simpleTypeID, lexical, canonical string) actualValue {
	if id == noSimpleType || !validUint32Index(uint32(id), len(rt.SimpleTypes)) {
		return actualValue{}
	}
	st := rt.SimpleTypes[id]
	if st.Variety != varietyAtomic {
		return actualValue{}
	}
	text := canonical
	if st.Primitive == primGMonthDay || st.Primitive == primGDay || st.Primitive == primGMonth || st.Primitive == primDuration {
		text = lexical
	}
	parsed, err := validatePrimitiveActual(rt, st, text, nil, true)
	if err != nil {
		return actualValue{}
	}
	return parsed.Actual
}
