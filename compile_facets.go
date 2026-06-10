package xsd

import (
	"fmt"
	"slices"
	"strconv"
)

func parseWhitespaceChecked(v string) (whitespaceMode, bool) {
	switch v {
	case xsdWhitespacePreserve:
		return whitespacePreserve, true
	case xsdWhitespaceReplace:
		return whitespaceReplace, true
	case xsdWhitespaceCollapse:
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
		case xsdFacetLength, xsdFacetMinLength, xsdFacetMaxLength, xsdFacetPattern, xsdFacetEnumeration, xsdFacetWhiteSpace:
			return true
		default:
			return false
		}
	case varietyUnion:
		return name == xsdFacetPattern || name == xsdFacetEnumeration
	}
	return false
}

func atomicFacetAllowed(kind primitiveKind, name string) bool {
	switch name {
	case xsdFacetPattern, xsdFacetEnumeration, xsdFacetWhiteSpace:
		return true
	case xsdFacetLength, xsdFacetMinLength, xsdFacetMaxLength:
		return primitiveHasLengthFacet(kind)
	case xsdFacetMinInclusive, xsdFacetMaxInclusive, xsdFacetMinExclusive, xsdFacetMaxExclusive:
		return primitiveHasOrderFacet(kind)
	case xsdFacetTotalDigits, xsdFacetFractionDigits:
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
	return withSchemaCompileLocation(parent, c.compileFacetList(parent.xsContentChildren(), st, base, literalType))
}

func (c *compiler) compileFacetList(children []*rawNode, st *simpleType, base, literalType simpleTypeID) error {
	baseType := c.rt.SimpleTypes[base]
	state := compiledFacetState{
		inheritedEnumeration: slices.Clone(st.Facets.Enumeration),
	}
	for _, child := range children {
		if child.Name.Local == xsdElemSimpleType {
			continue
		}
		if err := c.compileFacetChild(child, st, base, literalType, &state); err != nil {
			return err
		}
	}
	state.apply(st)
	return validateCompiledFacets(*st, baseType, state.orderedStep)
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
	if len(st.Facets.Enumeration) != 0 {
		st.Facets.Present |= facetFlagEnumeration
	} else {
		st.Facets.Present &^= facetFlagEnumeration
	}
	if len(s.stepPatterns) != 0 {
		st.Facets.Patterns = append(st.Facets.Patterns, patternGroup{Patterns: s.stepPatterns})
		st.Facets.Present |= facetFlagPattern
	}
}

func (c *compiler) compileFacetChild(child *rawNode, st *simpleType, base, literalType simpleTypeID, state *compiledFacetState) error {
	if !isFacetNode(child.Name.Local) {
		if child.Name.Space == xsdNamespaceURI {
			return schemaCompileAt(child, ErrSchemaFacet, "unsupported facet "+child.Name.Local)
		}
		return nil
	}
	facet, err := checkedFacet(*st, child)
	if err != nil {
		return err
	}
	switch child.Name.Local {
	case xsdFacetLength, xsdFacetMinLength, xsdFacetMaxLength, xsdFacetTotalDigits, xsdFacetFractionDigits:
		return compileSizeFacet(st, child, facet.value, facet.fixed)
	case xsdFacetMinInclusive, xsdFacetMaxInclusive, xsdFacetMinExclusive, xsdFacetMaxExclusive:
		return c.compileBoundFacet(st, base, child, facet.value, facet.fixed, &state.orderedStep)
	case xsdFacetEnumeration:
		lit, err := c.compileLiteral(literalType, facet.value, c.schemaQNameResolver(child))
		if err != nil {
			return withSchemaCompileLocation(child, err)
		}
		state.restrictedEnumeration = append(state.restrictedEnumeration, lit)
		state.sawEnumeration = true
	case xsdFacetPattern:
		p, err := c.compilePattern(child, facet.value)
		if err != nil {
			return err
		}
		state.stepPatterns = append(state.stepPatterns, p)
	case xsdFacetWhiteSpace:
		return c.compileWhitespaceFacet(st, base, child, facet.value, facet.fixed)
	}
	return nil
}

type facetInput struct {
	value string
	fixed bool
}

func checkedFacet(st simpleType, n *rawNode) (facetInput, error) {
	value, err := requiredFacetValue(n)
	if err != nil {
		return facetInput{}, err
	}
	if !facetAllowedForType(st, n.Name.Local) {
		return facetInput{}, schemaCompileAt(n, ErrSchemaFacet, "facet "+n.Name.Local+" is not allowed")
	}
	fixed, err := schemaBoolAttr(n, xsdAttrFixed)
	if err != nil {
		return facetInput{}, err
	}
	return facetInput{value: value, fixed: fixed}, nil
}

func requiredFacetValue(n *rawNode) (string, error) {
	value, ok := n.attr(xsdAttrValue)
	if !ok {
		return "", schemaCompileAt(n, ErrSchemaFacet, n.Name.Local+" missing value")
	}
	return value, nil
}

func compileSizeFacet(st *simpleType, node *rawNode, value string, fixed bool) error {
	name := node.Name.Local
	size, err := parseSizeFacetInteger(value)
	if err != nil {
		return schemaCompileAt(node, ErrSchemaFacet, "invalid "+name+" facet "+value)
	}
	if name == xsdFacetTotalDigits && size == 0 {
		return schemaCompileAt(node, ErrSchemaFacet, "totalDigits must be positive")
	}
	if size > maxUint32Value {
		return schemaCompileAt(node, ErrSchemaLimit, name+" facet exceeds uint32 limit")
	}
	v := uint32(size)
	var flag facetFlag
	switch name {
	case xsdFacetLength:
		st.Facets.Length = &v
		flag = facetFlagLength
	case xsdFacetMinLength:
		st.Facets.MinLength = &v
		flag = facetFlagMinLength
	case xsdFacetMaxLength:
		st.Facets.MaxLength = &v
		flag = facetFlagMaxLength
	case xsdFacetTotalDigits:
		st.Facets.TotalDigits = &v
		flag = facetFlagTotalDigits
	case xsdFacetFractionDigits:
		st.Facets.FractionDigits = &v
		flag = facetFlagFractionDigits
	}
	st.Facets.Present |= flag
	if fixed {
		st.Facets.Fixed |= flag
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
	var n uint64
	for i := digitStart; i < len(value); i++ {
		b := value[i]
		if b < '0' || b > '9' {
			return 0, strconv.ErrSyntax
		}
		if n <= maxUint32Value {
			n = n*10 + uint64(b-'0')
		}
	}
	return n, nil
}

func (c *compiler) compileBoundFacet(st *simpleType, base simpleTypeID, child *rawNode, value string, fixed bool, step *orderedFacetStep) error {
	lit, err := c.compileLiteral(base, value, c.schemaQNameResolver(child))
	if err != nil {
		return err
	}
	var flag facetFlag
	switch child.Name.Local {
	case xsdFacetMinInclusive:
		st.Facets.MinInclusive = &lit
		flag = facetFlagMinInclusive
		step.minInclusive = true
	case xsdFacetMaxInclusive:
		st.Facets.MaxInclusive = &lit
		flag = facetFlagMaxInclusive
		step.maxInclusive = true
	case xsdFacetMinExclusive:
		st.Facets.MinExclusive = &lit
		flag = facetFlagMinExclusive
		step.minExclusive = true
	case xsdFacetMaxExclusive:
		st.Facets.MaxExclusive = &lit
		flag = facetFlagMaxExclusive
		step.maxExclusive = true
	}
	st.Facets.Present |= flag
	if fixed {
		st.Facets.Fixed |= flag
	}
	return nil
}

func (c *compiler) compileWhitespaceFacet(st *simpleType, base simpleTypeID, n *rawNode, value string, fixed bool) error {
	mode, ok := parseWhitespaceChecked(value)
	if !ok {
		return schemaCompileAt(n, ErrSchemaFacet, "invalid whiteSpace facet "+value)
	}
	if !validWhitespaceRestriction(c.rt.SimpleTypes[base].Whitespace, mode) {
		return schemaCompileAt(n, ErrSchemaFacet, "whiteSpace cannot loosen base whiteSpace")
	}
	st.Whitespace = mode
	if fixed {
		st.Facets.Fixed |= facetFlagWhiteSpace
	}
	return nil
}

func validateCompiledFacets(st simpleType, base simpleType, orderedStep orderedFacetStep) error {
	baseFacets := base.Facets
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
	if err := validateFixedFacetRestrictions(st, base); err != nil {
		return schemaCompile(ErrSchemaFacet, err.Error())
	}
	return validatePrimitiveFacetRestrictions(st, baseFacets, orderedStep)
}

func validateFixedFacetRestrictions(st, base simpleType) error {
	fixed := base.Facets.Fixed
	if fixed&facetFlagLength != 0 && !uint32FacetEqual(st.Facets.Length, base.Facets.Length) {
		return fmt.Errorf("fixed length facet cannot change")
	}
	if fixed&facetFlagMinLength != 0 && !uint32FacetEqual(st.Facets.MinLength, base.Facets.MinLength) {
		return fmt.Errorf("fixed minLength facet cannot change")
	}
	if fixed&facetFlagMaxLength != 0 && !uint32FacetEqual(st.Facets.MaxLength, base.Facets.MaxLength) {
		return fmt.Errorf("fixed maxLength facet cannot change")
	}
	if fixed&facetFlagTotalDigits != 0 && !uint32FacetEqual(st.Facets.TotalDigits, base.Facets.TotalDigits) {
		return fmt.Errorf("fixed totalDigits facet cannot change")
	}
	if fixed&facetFlagFractionDigits != 0 && !uint32FacetEqual(st.Facets.FractionDigits, base.Facets.FractionDigits) {
		return fmt.Errorf("fixed fractionDigits facet cannot change")
	}
	if fixed&facetFlagMinInclusive != 0 && !literalFacetEqual(st.Facets.MinInclusive, base.Facets.MinInclusive) {
		return fmt.Errorf("fixed minInclusive facet cannot change")
	}
	if fixed&facetFlagMaxInclusive != 0 && !literalFacetEqual(st.Facets.MaxInclusive, base.Facets.MaxInclusive) {
		return fmt.Errorf("fixed maxInclusive facet cannot change")
	}
	if fixed&facetFlagMinExclusive != 0 && !literalFacetEqual(st.Facets.MinExclusive, base.Facets.MinExclusive) {
		return fmt.Errorf("fixed minExclusive facet cannot change")
	}
	if fixed&facetFlagMaxExclusive != 0 && !literalFacetEqual(st.Facets.MaxExclusive, base.Facets.MaxExclusive) {
		return fmt.Errorf("fixed maxExclusive facet cannot change")
	}
	if fixed&facetFlagWhiteSpace != 0 && st.Whitespace != base.Whitespace {
		return fmt.Errorf("fixed whiteSpace facet cannot change")
	}
	return nil
}

func uint32FacetEqual(a, b *uint32) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func literalFacetEqual(a, b *compiledLiteral) bool {
	if a == nil || b == nil {
		return a == b
	}
	return actualEqualsLiteral(a.Actual, a.Canonical, *b)
}

func validatePrimitiveFacetRestrictions(st simpleType, baseFacets facetSet, orderedStep orderedFacetStep) error {
	switch st.Primitive {
	case primDecimal:
		if err := validateDecimalFacetRestriction(st.Facets, baseFacets, orderedStep); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
		if err := validateDecimalFacetBounds(st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	case primFloat, primDouble:
		if err := validateFloatFacetBounds(st.Primitive, st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	case primDuration:
		if err := validateDurationFacetBounds(st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	case primGDay, primGMonthDay, primGMonth, primGYearMonth, primGYear:
		if err := validateGValueFacetBounds(st.Primitive, st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	case primDate, primDateTime:
		if err := validateTemporalFacetBounds(st.Primitive, st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	case primTime:
		if err := validateTimeFacetRestriction(st.Facets, baseFacets, orderedStep); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
		if err := validateTemporalFacetBounds(st.Primitive, st.Facets); err != nil {
			return schemaCompile(ErrSchemaFacet, err.Error())
		}
	default:
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
	baseLower, err := decimalLowerBound(base)
	if err != nil {
		return err
	}
	baseUpper, err := decimalUpperBound(base)
	if err != nil {
		return err
	}
	if step.minInclusive {
		if err := validateDecimalLowerRestriction(xsdFacetMinInclusive, f.MinInclusive, facetInclusive, baseLower); err != nil {
			return err
		}
	}
	if step.minExclusive {
		if err := validateDecimalLowerRestriction(xsdFacetMinExclusive, f.MinExclusive, facetExclusive, baseLower); err != nil {
			return err
		}
	}
	if step.maxInclusive {
		if err := validateDecimalUpperRestriction(xsdFacetMaxInclusive, f.MaxInclusive, facetInclusive, baseUpper); err != nil {
			return err
		}
	}
	if step.maxExclusive {
		if err := validateDecimalUpperRestriction(xsdFacetMaxExclusive, f.MaxExclusive, facetExclusive, baseUpper); err != nil {
			return err
		}
	}
	return nil
}

type facetBoundStyle uint8

const (
	facetInclusive facetBoundStyle = iota
	facetExclusive
)

func validateDecimalLowerRestriction(name string, lit *compiledLiteral, style facetBoundStyle, base orderedFacetBound[decimalValue]) error {
	if lit == nil || !base.present() {
		return nil
	}
	cmp := compareDecimalValues(literalDecimal(lit), base.value)
	if cmp < 0 || cmp == 0 && style == facetInclusive && base.exclusive() {
		return fmt.Errorf("%s cannot be less than base lower bound", name)
	}
	return nil
}

func validateDecimalUpperRestriction(name string, lit *compiledLiteral, style facetBoundStyle, base orderedFacetBound[decimalValue]) error {
	if lit == nil || !base.present() {
		return nil
	}
	cmp := compareDecimalValues(literalDecimal(lit), base.value)
	if cmp > 0 || cmp == 0 && style == facetInclusive && base.exclusive() {
		return fmt.Errorf("%s cannot exceed base upper bound", name)
	}
	return nil
}

func validateDecimalFacetBounds(f facetSet) error {
	lower, err := decimalLowerBound(f)
	if err != nil {
		return err
	}
	upper, err := decimalUpperBound(f)
	if err != nil {
		return err
	}
	if !lower.present() || !upper.present() {
		return nil
	}
	cmp := compareDecimalValues(lower.value, upper.value)
	if cmp > 0 || cmp == 0 && (lower.exclusive() || upper.exclusive()) {
		return fmt.Errorf("decimal lower bound cannot exceed upper bound")
	}
	return nil
}

func decimalLowerBound(f facetSet) (orderedFacetBound[decimalValue], error) {
	return facetBound(f.MinInclusive, f.MinExclusive, facetCanonical, parseDecimalValue, func(other, out decimalValue) bool {
		return compareDecimalValues(other, out) >= 0
	})
}

func decimalUpperBound(f facetSet) (orderedFacetBound[decimalValue], error) {
	return facetBound(f.MaxInclusive, f.MaxExclusive, facetCanonical, parseDecimalValue, func(other, out decimalValue) bool {
		return compareDecimalValues(other, out) <= 0
	})
}

func (c *compiler) compileLiteral(base simpleTypeID, lexical string, resolve qnameResolver) (compiledLiteral, error) {
	value, err := validateSimpleValueMode(&c.rt, base, lexical, resolve, simpleNeedCanonical)
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
	st, ok := rt.simpleType(id)
	if !ok || st.Variety != varietyAtomic {
		return actualValue{}
	}
	text := canonical
	if st.Primitive == primGMonthDay || st.Primitive == primGDay || st.Primitive == primGMonth || st.Primitive == primDuration {
		text = lexical
	}
	parsed, err := validatePrimitiveActual(rt, st, text, nil, primitiveNeedCanonical|primitiveNeedLength)
	if err != nil {
		return actualValue{}
	}
	return parsed.Actual
}
