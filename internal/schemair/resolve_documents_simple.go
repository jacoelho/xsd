package schemair

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	ast "github.com/jacoelho/xsd/internal/schemaast"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/num"
	"github.com/jacoelho/xsd/internal/xsdlex"
)

func (r *docResolver) simpleBaseAndDerivation(decl *ast.SimpleTypeDecl) (TypeRef, Derivation, error) {
	switch decl.Kind {
	case ast.SimpleDerivationList:
		return r.builtinRef("anySimpleType"), DerivationList, nil
	case ast.SimpleDerivationUnion:
		return r.builtinRef("anySimpleType"), DerivationUnion, nil
	case ast.SimpleDerivationRestriction:
		if decl.InlineBase != nil {
			id, err := r.ensureSimpleType(decl.InlineBase, false)
			if err != nil {
				return TypeRef{}, 0, err
			}
			return TypeRef{ID: id, Name: nameFromQName(decl.InlineBase.Name)}, DerivationRestriction, nil
		}
		if !decl.Base.IsZero() {
			ref, err := r.typeRef(decl.Base)
			if err != nil {
				return TypeRef{}, 0, err
			}
			if err := r.validateSimpleRestrictionBase(ref); err != nil {
				return TypeRef{}, 0, err
			}
			return ref, DerivationRestriction, nil
		}
	}
	return TypeRef{}, DerivationRestriction, nil
}

func (r *docResolver) validateSimpleRestrictionBase(ref TypeRef) error {
	if ref.Builtin {
		switch ref.Name.Local {
		case "anyType":
			return fmt.Errorf("schema ir: simpleType restriction cannot have base type anyType")
		case "anySimpleType":
			return fmt.Errorf("schema ir: simpleType restriction cannot have base type anySimpleType")
		default:
			return nil
		}
	}
	info, ok, err := r.typeInfoForRef(ref)
	if err != nil || !ok {
		return err
	}
	if info.Kind == TypeComplex {
		return fmt.Errorf("schema ir: simpleType restriction cannot have complex base type '%s'", formatName(ref.Name))
	}
	return nil
}

func (r *docResolver) simpleSpec(id TypeID, name Name, decl *ast.SimpleTypeDecl) (SimpleTypeSpec, error) {
	base, _, err := r.simpleBaseAndDerivation(decl)
	if err != nil {
		return SimpleTypeSpec{}, err
	}
	spec := SimpleTypeSpec{
		TypeDecl:   id,
		Name:       name,
		Variety:    TypeVarietyAtomic,
		Base:       base,
		Whitespace: WhitespacePreserve,
	}
	if !isZeroTypeRef(base) {
		if baseSpec, ok := r.specForRef(base); ok {
			spec.Primitive = baseSpec.Primitive
			spec.BuiltinBase = baseSpec.BuiltinBase
			spec.Whitespace = baseSpec.Whitespace
			spec.QNameOrNotation = baseSpec.QNameOrNotation
			spec.IntegerDerived = baseSpec.IntegerDerived
			spec.Facets = append(spec.Facets, baseSpec.Facets...)
			if decl.Kind == ast.SimpleDerivationRestriction && baseSpec.Variety != TypeVarietyAtomic {
				spec.Variety = baseSpec.Variety
				spec.Item = baseSpec.Item
				spec.Members = append(spec.Members, baseSpec.Members...)
			}
		}
		if base.Builtin && ast.IsBuiltinListTypeName(base.Name.Local) {
			spec.Facets = append(spec.Facets, FacetSpec{Kind: FacetMinLength, Name: "minLength", IntValue: 1})
		}
	}
	switch decl.Kind {
	case ast.SimpleDerivationList:
		spec.Variety = TypeVarietyList
		item, err := r.simpleItemRef(decl)
		if err != nil {
			return SimpleTypeSpec{}, err
		}
		if err := r.validateListItemRef(item); err != nil {
			return SimpleTypeSpec{}, err
		}
		spec.Item = item
		spec.Primitive = "string"
		spec.BuiltinBase = "string"
		spec.Whitespace = WhitespaceCollapse
	case ast.SimpleDerivationUnion:
		spec.Variety = TypeVarietyUnion
		members, err := r.simpleMemberRefs(decl)
		if err != nil {
			return SimpleTypeSpec{}, err
		}
		for _, member := range members {
			if err := r.validateUnionMemberRef(member); err != nil {
				return SimpleTypeSpec{}, err
			}
		}
		spec.Members = members
		spec.Primitive = "string"
		spec.BuiltinBase = "string"
		spec.Whitespace = WhitespaceCollapse
	}
	return r.applyRestrictionFacets(spec, decl.Facets, restrictionFacetOptions{
		context: "restriction",
	})
}

type restrictionFacetOptions struct {
	context               string
	rejectAnySimpleFacets bool
}

func (r *docResolver) applyRestrictionFacets(
	spec SimpleTypeSpec,
	facets []ast.FacetDecl,
	opts restrictionFacetOptions,
) (SimpleTypeSpec, error) {
	baseWhitespace := spec.Whitespace
	ownWhitespace := false
	var ownFacets []FacetSpec
	for _, facet := range facets {
		if facet.Name == "whiteSpace" {
			spec.Whitespace = whitespaceModeFromString(facet.Lexical)
			ownWhitespace = true
			continue
		}
		converted, ok, err := r.facetSpec(facet)
		if err != nil {
			return SimpleTypeSpec{}, err
		}
		if ok {
			ownFacets = append(ownFacets, converted)
		}
	}
	ownFacets = coalesceFacetSpecs(ownFacets)
	if ownWhitespace && !validWhitespaceRestriction(baseWhitespace, spec.Whitespace) {
		return SimpleTypeSpec{}, fmt.Errorf("schema ir: %s: whiteSpace facet value '%s' cannot be less restrictive than base type's '%s'", opts.context, whitespaceModeString(spec.Whitespace), whitespaceModeString(baseWhitespace))
	}
	if opts.rejectAnySimpleFacets && len(ownFacets) > 0 && spec.Primitive == "anySimpleType" {
		return SimpleTypeSpec{}, fmt.Errorf("schema ir: %s cannot apply facets to base type anySimpleType", opts.context)
	}
	if err := validateFacetApplicability(spec, ownFacets); err != nil {
		return SimpleTypeSpec{}, err
	}
	if err := validateIRLengthFacetConsistency(spec, ownFacets); err != nil {
		return SimpleTypeSpec{}, err
	}
	if err := validateIRDigitsFacetConsistency(spec, ownFacets); err != nil {
		return SimpleTypeSpec{}, err
	}
	if err := validateRangeFacetConsistency(spec, ownFacets); err != nil {
		return SimpleTypeSpec{}, err
	}
	if err := validateIRFacetRestriction(spec, ownFacets); err != nil {
		return SimpleTypeSpec{}, err
	}
	if err := r.validateNotationRestriction(spec, ownFacets); err != nil {
		return SimpleTypeSpec{}, err
	}
	if err := r.validateEnumerationLexicalValues(spec, ownFacets); err != nil {
		return SimpleTypeSpec{}, err
	}
	if err := r.validateRestrictionEnumerations(spec, ownFacets); err != nil {
		return SimpleTypeSpec{}, err
	}
	spec.Facets = append(spec.Facets, ownFacets...)
	if spec.Primitive == "" {
		spec.Primitive = fallbackSpecName(spec)
	}
	if spec.BuiltinBase == "" {
		spec.BuiltinBase = spec.Primitive
	}
	return spec, nil
}

func (r *docResolver) validateNotationRestriction(spec SimpleTypeSpec, ownFacets []FacetSpec) error {
	if spec.Primitive != "NOTATION" {
		return nil
	}
	values := enumFacetValues(ownFacets)
	if len(values) == 0 && directNotationRestriction(spec.Base) {
		return fmt.Errorf("schema ir: NOTATION restriction must have enumeration facet")
	}
	if len(values) == 0 {
		return nil
	}
	for _, value := range values {
		qname, err := xsdlex.ParseQNameValue(value.Lexical, value.Context)
		if err != nil {
			return err
		}
		if _, ok := r.notations[nameFromQName(qname)]; !ok {
			return fmt.Errorf("schema ir: enumeration value %q does not reference a declared notation", value.Lexical)
		}
	}
	return nil
}

func directNotationRestriction(ref TypeRef) bool {
	return ref.Builtin && ref.Name.Namespace == ast.XSDNamespace && ref.Name.Local == "NOTATION"
}

func (r *docResolver) validateEnumerationLexicalValues(spec SimpleTypeSpec, ownFacets []FacetSpec) error {
	values := enumFacetValues(ownFacets)
	for i, value := range values {
		if err := r.validateSpecLexicalValue(spec, value.Lexical, value.Context, make(map[TypeRef]bool)); err != nil {
			base := cmp.Or(spec.BuiltinBase, spec.Primitive, spec.Name.Local)
			return fmt.Errorf("schema ir: restriction: enumeration value %d (%q) is not valid for base type %s: %w", i+1, value.Lexical, base, err)
		}
	}
	return nil
}

func (r *docResolver) validateSpecLexicalValue(spec SimpleTypeSpec, lexical string, ctx map[string]string, seen map[TypeRef]bool) error {
	return validateSpecLexicalValueWithResolver(spec, lexical, ctx, r.specForRef, seen)
}

func validateSpecLexicalValueWithResolver(
	spec SimpleTypeSpec,
	lexical string,
	ctx map[string]string,
	resolve ValueSpecResolver,
	seen map[TypeRef]bool,
) error {
	normalized := value.NormalizeWhitespace(valueWhitespaceMode(spec.Whitespace), []byte(lexical), nil)
	switch spec.Variety {
	case TypeVarietyList:
		if isZeroTypeRef(spec.Item) {
			return nil
		}
		item, ok := resolve(spec.Item)
		if !ok {
			return fmt.Errorf("list item type %s not found", formatName(spec.Item.Name))
		}
		count := 0
		for itemLex := range value.FieldsXMLWhitespaceStringSeq(string(normalized)) {
			count++
			if err := validateSpecLexicalValueWithResolver(item, itemLex, ctx, resolve, seen); err != nil {
				return err
			}
		}
		if err := validateListValueLength(spec, count); err != nil {
			return err
		}
		return validateSpecConstrainingFacets(spec, string(normalized), ctx, resolve)
	case TypeVarietyUnion:
		var lastErr error
		for _, memberRef := range spec.Members {
			if seen[memberRef] {
				continue
			}
			branchSeen := make(map[TypeRef]bool, len(seen)+1)
			for ref, ok := range seen {
				branchSeen[ref] = ok
			}
			branchSeen[memberRef] = true
			member, ok := resolve(memberRef)
			if !ok {
				lastErr = fmt.Errorf("union member type %s not found", formatName(memberRef.Name))
				continue
			}
			if err := validateSpecLexicalValueWithResolver(member, lexical, ctx, resolve, branchSeen); err != nil {
				lastErr = err
				continue
			}
			if err := validateSpecConstrainingFacets(spec, string(normalized), ctx, resolve); err != nil {
				return err
			}
			return nil
		}
		if lastErr != nil {
			return lastErr
		}
		return fmt.Errorf("value does not match any union member")
	default:
		if err := validateAtomicLexicalValue(validationBuiltinName(spec), string(normalized), ctx); err != nil {
			return err
		}
		return validateSpecConstrainingFacets(spec, string(normalized), ctx, resolve)
	}
}

func validateSpecConstrainingFacets(spec SimpleTypeSpec, normalized string, ctx map[string]string, resolve ValueSpecResolver) error {
	if err := validateSpecEnumerationFacets(spec, normalized, ctx, resolve); err != nil {
		return err
	}
	if err := validateSpecPatternFacets(spec, normalized); err != nil {
		return err
	}
	return validateSpecRangeFacets(spec, normalized)
}

func validateSpecEnumerationFacets(spec SimpleTypeSpec, normalized string, ctx map[string]string, resolve ValueSpecResolver) error {
	lookup, constrained, err := effectiveEnumValueKeys(spec, resolve)
	if err != nil {
		return err
	}
	if !constrained {
		return nil
	}
	keys, err := ValueKeysForNormalized(normalized, normalized, spec, ctx, resolve)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if _, ok := lookup[valueKeyLookup(key)]; ok {
			return nil
		}
	}
	return fmt.Errorf("value not in enumeration")
}

func validateSpecPatternFacets(spec SimpleTypeSpec, normalized string) error {
	for _, facet := range spec.Facets {
		if facet.Kind != FacetPattern || len(facet.Values) == 0 {
			continue
		}
		matched := false
		for _, value := range facet.Values {
			re, err := regexp.Compile(value.Lexical)
			if err != nil {
				return err
			}
			if re.MatchString(normalized) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("pattern violation")
		}
	}
	return nil
}

func validateSpecRangeFacets(spec SimpleTypeSpec, normalized string) error {
	for _, facet := range spec.Facets {
		switch facet.Kind {
		case FacetMinInclusive, FacetMinExclusive, FacetMaxInclusive, FacetMaxExclusive:
		default:
			continue
		}
		cmp, comparable, err := compareIRRangeFacetValues(spec, normalized, facet.Value)
		if !comparable {
			continue
		}
		if err != nil {
			return err
		}
		var ok bool
		switch facet.Kind {
		case FacetMinInclusive:
			ok = cmp >= 0
		case FacetMinExclusive:
			ok = cmp > 0
		case FacetMaxInclusive:
			ok = cmp <= 0
		case FacetMaxExclusive:
			ok = cmp < 0
		}
		if !ok {
			return fmt.Errorf("%s violation", facet.Name)
		}
	}
	return nil
}

func validateListValueLength(spec SimpleTypeSpec, count int) error {
	for _, facet := range spec.Facets {
		switch facet.Kind {
		case FacetLength:
			if count != int(facet.IntValue) {
				return fmt.Errorf("length violation")
			}
		case FacetMinLength:
			if count < int(facet.IntValue) {
				return fmt.Errorf("minLength violation")
			}
		case FacetMaxLength:
			if count > int(facet.IntValue) {
				return fmt.Errorf("maxLength violation")
			}
		}
	}
	return nil
}

func validationBuiltinName(spec SimpleTypeSpec) string {
	return cmp.Or(spec.BuiltinBase, spec.Primitive, spec.Name.Local, "string")
}

func validateAtomicLexicalValue(name, lexical string, ctx map[string]string) error {
	switch name {
	case "anyType", "anySimpleType", "string":
		return nil
	case "boolean":
		return value.ValidateXSDBoolean(lexical)
	case "decimal":
		return value.ValidateXSDDecimal(lexical)
	case "float":
		return value.ValidateXSDFloat(lexical)
	case "double":
		return value.ValidateXSDDouble(lexical)
	case "duration":
		return value.ValidateXSDDuration(lexical)
	case "dateTime":
		return value.ValidateXSDDateTime(lexical)
	case "time":
		return value.ValidateXSDTime(lexical)
	case "date":
		return value.ValidateXSDDate(lexical)
	case "gYearMonth":
		return value.ValidateXSDGYearMonth(lexical)
	case "gYear":
		return value.ValidateXSDGYear(lexical)
	case "gMonthDay":
		return value.ValidateXSDGMonthDay(lexical)
	case "gDay":
		return value.ValidateXSDGDay(lexical)
	case "gMonth":
		return value.ValidateXSDGMonth(lexical)
	case "hexBinary":
		return value.ValidateXSDHexBinary(lexical)
	case "base64Binary":
		return value.ValidateXSDBase64Binary(lexical)
	case "anyURI":
		return value.ValidateXSDAnyURI(lexical)
	case "QName", "NOTATION":
		if err := value.ValidateXSDQName(lexical); err != nil {
			return err
		}
		_, err := xsdlex.ParseQNameValue(lexical, ctx)
		return err
	case "normalizedString":
		return value.ValidateXSDNormalizedString(lexical)
	case "token":
		return value.ValidateXSDToken(lexical)
	case "language":
		return value.ValidateXSDLanguage(lexical)
	case "Name":
		return value.ValidateXSDName(lexical)
	case "NCName", "ID", "IDREF", "ENTITY":
		return value.ValidateXSDNCName(lexical)
	case "NMTOKEN":
		return value.ValidateXSDNMTOKEN(lexical)
	case "integer":
		return value.ValidateXSDInteger(lexical)
	case "long":
		return value.ValidateXSDLong(lexical)
	case "int":
		return value.ValidateXSDInt(lexical)
	case "short":
		return value.ValidateXSDShort(lexical)
	case "byte":
		return value.ValidateXSDByte(lexical)
	case "nonNegativeInteger":
		return value.ValidateXSDNonNegativeInteger(lexical)
	case "positiveInteger":
		return value.ValidateXSDPositiveInteger(lexical)
	case "unsignedLong":
		return value.ValidateXSDUnsignedLong(lexical)
	case "unsignedInt":
		return value.ValidateXSDUnsignedInt(lexical)
	case "unsignedShort":
		return value.ValidateXSDUnsignedShort(lexical)
	case "unsignedByte":
		return value.ValidateXSDUnsignedByte(lexical)
	case "nonPositiveInteger":
		return value.ValidateXSDNonPositiveInteger(lexical)
	case "negativeInteger":
		return value.ValidateXSDNegativeInteger(lexical)
	default:
		return nil
	}
}

func validateFacetApplicability(spec SimpleTypeSpec, facets []FacetSpec) error {
	for _, facet := range facets {
		if isLengthFacet(facet.Kind) && spec.Variety == TypeVarietyUnion {
			return fmt.Errorf("schema ir: restriction: facet %s is not applicable to union type %s", facet.Name, fallbackSpecName(spec))
		}
		if isDigitsFacet(facet.Kind) && (spec.Variety != TypeVarietyAtomic || spec.Primitive != "decimal") {
			return fmt.Errorf("schema ir: restriction: facet %s is only applicable to decimal-derived types, but base type %s is not decimal-derived", facet.Name, fallbackSpecName(spec))
		}
		if isRangeFacet(facet.Kind) && !specAllowsRangeFacet(spec) {
			return fmt.Errorf("schema ir: facet %s is only applicable to ordered types, but base type %s is not ordered", facet.Name, formatName(spec.Name))
		}
	}
	return nil
}

func isLengthFacet(kind FacetKind) bool {
	switch kind {
	case FacetLength, FacetMinLength, FacetMaxLength:
		return true
	default:
		return false
	}
}

func isDigitsFacet(kind FacetKind) bool {
	switch kind {
	case FacetTotalDigits, FacetFractionDigits:
		return true
	default:
		return false
	}
}

func validateIRLengthFacetConsistency(spec SimpleTypeSpec, facets []FacetSpec) error {
	var (
		length    uint32
		minLength uint32
		maxLength uint32
		hasLength bool
		hasMin    bool
		hasMax    bool
	)
	for _, facet := range facets {
		switch facet.Kind {
		case FacetLength:
			length, hasLength = facet.IntValue, true
		case FacetMinLength:
			minLength, hasMin = facet.IntValue, true
		case FacetMaxLength:
			maxLength, hasMax = facet.IntValue, true
		}
	}
	if hasLength && (hasMin || hasMax) {
		if spec.Variety != TypeVarietyList {
			return fmt.Errorf("schema ir: restriction: length facet cannot be used together with minLength or maxLength")
		}
		if hasMax {
			return fmt.Errorf("schema ir: restriction: length facet cannot be used together with maxLength for list types")
		}
	}
	if hasMin && hasMax && minLength > maxLength {
		return fmt.Errorf("schema ir: restriction: minLength (%d) must be <= maxLength (%d)", minLength, maxLength)
	}
	if spec.Variety == TypeVarietyList {
		name := spec.Name.Local
		if name == "" {
			name = spec.BuiltinBase
		}
		if hasLength && length < 1 {
			return fmt.Errorf("schema ir: restriction: length (%d) must be >= 1 for list type %s", length, name)
		}
		if hasMin && minLength < 1 {
			return fmt.Errorf("schema ir: restriction: minLength (%d) must be >= 1 for list type %s", minLength, name)
		}
		if hasMax && maxLength < 1 {
			return fmt.Errorf("schema ir: restriction: maxLength (%d) must be >= 1 for list type %s", maxLength, name)
		}
	}
	return nil
}

func validateIRDigitsFacetConsistency(spec SimpleTypeSpec, facets []FacetSpec) error {
	var (
		total    uint32
		fraction uint32
		hasTotal bool
		hasFrac  bool
	)
	for _, facet := range facets {
		switch facet.Kind {
		case FacetTotalDigits:
			total, hasTotal = facet.IntValue, true
		case FacetFractionDigits:
			fraction, hasFrac = facet.IntValue, true
		}
	}
	if hasTotal && hasFrac && fraction > total {
		return fmt.Errorf("schema ir: restriction: fractionDigits (%d) must be <= totalDigits (%d)", fraction, total)
	}
	if hasFrac && fraction != 0 && spec.IntegerDerived {
		name := spec.Name.Local
		if name == "" {
			name = spec.BuiltinBase
		}
		return fmt.Errorf("schema ir: restriction: fractionDigits must be 0 for integer type %s, got %d", name, fraction)
	}
	return nil
}

func isRangeFacet(kind FacetKind) bool {
	switch kind {
	case FacetMinInclusive, FacetMaxInclusive, FacetMinExclusive, FacetMaxExclusive:
		return true
	default:
		return false
	}
}

func specAllowsRangeFacet(spec SimpleTypeSpec) bool {
	if spec.Variety == TypeVarietyList || spec.Variety == TypeVarietyUnion {
		return false
	}
	switch spec.Primitive {
	case "decimal", "float", "double", "duration", "dateTime", "time", "date", "gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth":
		return true
	default:
		return false
	}
}

func validateRangeFacetConsistency(spec SimpleTypeSpec, facets []FacetSpec) error {
	var minExclusive, maxExclusive, minInclusive, maxInclusive *string
	for i := range facets {
		facet := &facets[i]
		if !isRangeFacet(facet.Kind) {
			continue
		}
		if err := validateRangeFacetLexical(spec, *facet); err != nil {
			return err
		}
		value := facet.Value
		switch facet.Kind {
		case FacetMinExclusive:
			minExclusive = &value
		case FacetMaxExclusive:
			maxExclusive = &value
		case FacetMinInclusive:
			minInclusive = &value
		case FacetMaxInclusive:
			maxInclusive = &value
		}
	}
	if minExclusive == nil && maxExclusive == nil && minInclusive == nil && maxInclusive == nil {
		return nil
	}
	if maxInclusive != nil && maxExclusive != nil {
		return fmt.Errorf("maxInclusive and maxExclusive cannot both be specified")
	}
	if minInclusive != nil && minExclusive != nil {
		return fmt.Errorf("minInclusive and minExclusive cannot both be specified")
	}
	compare := func(left, right string) (int, bool, error) {
		cmp, err := compareRangeFacetValues(spec, left, right)
		if errors.Is(err, ErrDateTimeNotComparable) || errors.Is(err, ErrDurationNotComparable) || errors.Is(err, ErrFloatNotComparable) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		return cmp, true, nil
	}
	return checkRangeConstraints(minExclusive, maxExclusive, minInclusive, maxInclusive, compare)
}

type rangeConstraintCompare func(left, right string) (cmp int, comparable bool, err error)

func checkRangeConstraints(
	minExclusive, maxExclusive, minInclusive, maxInclusive *string,
	compare rangeConstraintCompare,
) error {
	minName, minValue, minOpen := activeMinRange(minExclusive, minInclusive)
	maxName, maxValue, maxOpen := activeMaxRange(maxExclusive, maxInclusive)
	if minValue == nil || maxValue == nil {
		return nil
	}
	cmp, comparable, err := compare(*minValue, *maxValue)
	if err != nil || !comparable {
		return err
	}
	if cmp > 0 || (cmp == 0 && (minOpen || maxOpen)) {
		op := "<="
		if minOpen || maxOpen {
			op = "<"
		}
		return fmt.Errorf("%s (%s) must be %s %s (%s)", minName, *minValue, op, maxName, *maxValue)
	}
	return nil
}

func activeMinRange(minExclusive, minInclusive *string) (string, *string, bool) {
	if minExclusive != nil {
		return "minExclusive", minExclusive, true
	}
	return "minInclusive", minInclusive, false
}

func activeMaxRange(maxExclusive, maxInclusive *string) (string, *string, bool) {
	if maxExclusive != nil {
		return "maxExclusive", maxExclusive, true
	}
	return "maxInclusive", maxInclusive, false
}

func validateRangeFacetLexical(spec SimpleTypeSpec, facet FacetSpec) error {
	_, err := compareRangeFacetValues(spec, facet.Value, facet.Value)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrDateTimeNotComparable) || errors.Is(err, ErrDurationNotComparable) || errors.Is(err, ErrFloatNotComparable) {
		return nil
	}
	return fmt.Errorf("schema ir: restriction: invalid %s facet value %q: %w", facet.Name, facet.Value, err)
}

func validateIRFacetRestriction(spec SimpleTypeSpec, ownFacets []FacetSpec) error {
	if len(ownFacets) == 0 {
		return nil
	}
	if err := validateIRRangeFacetRestriction(spec, ownFacets); err != nil {
		return err
	}
	if len(spec.Facets) == 0 {
		return nil
	}

	baseFacets := make(map[FacetKind]FacetSpec, len(spec.Facets))
	for _, facet := range spec.Facets {
		baseFacets[facet.Kind] = facet
	}
	for _, derived := range ownFacets {
		if isRangeFacet(derived.Kind) {
			continue
		}
		base, ok := baseFacets[derived.Kind]
		if !ok {
			continue
		}
		if err := validateIRIntFacetRestriction(base, derived); err != nil {
			return err
		}
	}
	return nil
}

func validateIRRangeFacetRestriction(spec SimpleTypeSpec, ownFacets []FacetSpec) error {
	base := baseIRRangeFacetInfo(spec)
	derived := extractIRRangeFacetInfo(ownFacets)
	checks := [...]irRangeFacetCheck{
		{
			active:             base.hasMin && derived.hasMin,
			errPrefix:          "min facet",
			left:               derived.minValue,
			right:              base.minValue,
			invalidOrdering:    func(cmp int) bool { return cmp < 0 },
			invalidComparison:  "derived value (%s) must be >= base value (%s) to be a valid restriction",
			invalidEquality:    func(cmp int) bool { return cmp == 0 },
			invalidEqualBounds: func() bool { return !base.minInclusive && derived.minInclusive },
			invalidEqualMsg:    "derived inclusive value (%s) cannot relax base exclusive bound",
		},
		{
			active:             base.hasMax && derived.hasMax,
			errPrefix:          "max facet",
			left:               derived.maxValue,
			right:              base.maxValue,
			invalidOrdering:    func(cmp int) bool { return cmp > 0 },
			invalidComparison:  "derived value (%s) must be <= base value (%s) to be a valid restriction",
			invalidEquality:    func(cmp int) bool { return cmp == 0 },
			invalidEqualBounds: func() bool { return !base.maxInclusive && derived.maxInclusive },
			invalidEqualMsg:    "derived inclusive value (%s) cannot relax base exclusive bound",
		},
		{
			active:             base.hasMax && derived.hasMin,
			errPrefix:          "min/max facet",
			left:               derived.minValue,
			right:              base.maxValue,
			invalidOrdering:    func(cmp int) bool { return cmp > 0 },
			invalidComparison:  "derived min (%s) must be <= base max (%s)",
			invalidEquality:    func(cmp int) bool { return cmp == 0 },
			invalidEqualBounds: func() bool { return !base.maxInclusive || !derived.minInclusive },
			invalidEqualMsg:    "derived min (%s) cannot relax base max bound",
		},
		{
			active:             base.hasMin && derived.hasMax,
			errPrefix:          "min/max facet",
			left:               derived.maxValue,
			right:              base.minValue,
			invalidOrdering:    func(cmp int) bool { return cmp < 0 },
			invalidComparison:  "derived max (%s) must be >= base min (%s)",
			invalidEquality:    func(cmp int) bool { return cmp == 0 },
			invalidEqualBounds: func() bool { return !base.minInclusive || !derived.maxInclusive },
			invalidEqualMsg:    "derived max (%s) cannot relax base min bound",
		},
	}
	for _, check := range checks {
		if err := validateIRRangeFacetCheck(spec, check); err != nil {
			return err
		}
	}
	return nil
}

func baseIRRangeFacetInfo(spec SimpleTypeSpec) rangeFacetInfo {
	info := extractIRRangeFacetInfo(spec.Facets)
	if !spec.Base.Builtin {
		return info
	}
	implicit, ok := builtinRangeFacetInfoFor(spec.Base.Name.Local)
	if !ok {
		return info
	}
	return mergeIRRangeFacetInfo(implicit, info)
}

func builtinRangeFacetInfoFor(local string) (rangeFacetInfo, bool) {
	switch local {
	case "nonNegativeInteger":
		return rangeFacetInfo{minValue: "0", minInclusive: true, hasMin: true}, true
	case "positiveInteger":
		return rangeFacetInfo{minValue: "1", minInclusive: true, hasMin: true}, true
	case "nonPositiveInteger":
		return rangeFacetInfo{maxValue: "0", maxInclusive: true, hasMax: true}, true
	case "negativeInteger":
		return rangeFacetInfo{maxValue: "-1", maxInclusive: true, hasMax: true}, true
	case "long":
		return closedRangeFacetInfo("-9223372036854775808", "9223372036854775807"), true
	case "int":
		return closedRangeFacetInfo("-2147483648", "2147483647"), true
	case "short":
		return closedRangeFacetInfo("-32768", "32767"), true
	case "byte":
		return closedRangeFacetInfo("-128", "127"), true
	case "unsignedLong":
		return closedRangeFacetInfo("0", "18446744073709551615"), true
	case "unsignedInt":
		return closedRangeFacetInfo("0", "4294967295"), true
	case "unsignedShort":
		return closedRangeFacetInfo("0", "65535"), true
	case "unsignedByte":
		return closedRangeFacetInfo("0", "255"), true
	default:
		return rangeFacetInfo{}, false
	}
}

func closedRangeFacetInfo(minValue, maxValue string) rangeFacetInfo {
	return rangeFacetInfo{
		minValue:     minValue,
		maxValue:     maxValue,
		hasMin:       true,
		hasMax:       true,
		minInclusive: true,
		maxInclusive: true,
	}
}

type rangeFacetInfo struct {
	minValue     string
	maxValue     string
	hasMin       bool
	hasMax       bool
	minInclusive bool
	maxInclusive bool
}

func mergeIRRangeFacetInfo(base, override rangeFacetInfo) rangeFacetInfo {
	out := base
	if override.hasMin {
		out.minValue = override.minValue
		out.minInclusive = override.minInclusive
		out.hasMin = true
	}
	if override.hasMax {
		out.maxValue = override.maxValue
		out.maxInclusive = override.maxInclusive
		out.hasMax = true
	}
	return out
}

type irRangeFacetCheck struct {
	active             bool
	errPrefix          string
	left               string
	right              string
	invalidOrdering    func(int) bool
	invalidComparison  string
	invalidEquality    func(int) bool
	invalidEqualBounds func() bool
	invalidEqualMsg    string
}

func validateIRRangeFacetCheck(spec SimpleTypeSpec, check irRangeFacetCheck) error {
	if !check.active {
		return nil
	}
	cmp, comparable, err := compareIRRangeFacetValues(spec, check.left, check.right)
	if !comparable {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: cannot compare values: %w", check.errPrefix, err)
	}
	if check.invalidOrdering(cmp) {
		return fmt.Errorf("%s: "+check.invalidComparison, check.errPrefix, check.left, check.right)
	}
	if check.invalidEquality(cmp) && check.invalidEqualBounds() {
		return fmt.Errorf("%s: "+check.invalidEqualMsg, check.errPrefix, check.left)
	}
	return nil
}

func compareIRRangeFacetValues(spec SimpleTypeSpec, left, right string) (int, bool, error) {
	cmp, err := compareRangeFacetValues(spec, left, right)
	if errors.Is(err, ErrDurationNotComparable) || errors.Is(err, ErrFloatNotComparable) {
		return 0, false, nil
	}
	if err != nil {
		return 0, true, err
	}
	return cmp, true, nil
}

func extractIRRangeFacetInfo(facets []FacetSpec) rangeFacetInfo {
	var info rangeFacetInfo
	for _, facet := range facets {
		switch facet.Kind {
		case FacetMinInclusive:
			info.minValue = facet.Value
			info.minInclusive = true
			info.hasMin = true
		case FacetMinExclusive:
			info.minValue = facet.Value
			info.minInclusive = false
			info.hasMin = true
		case FacetMaxInclusive:
			info.maxValue = facet.Value
			info.maxInclusive = true
			info.hasMax = true
		case FacetMaxExclusive:
			info.maxValue = facet.Value
			info.maxInclusive = false
			info.hasMax = true
		}
	}
	return info
}

func validateIRIntFacetRestriction(base, derived FacetSpec) error {
	switch derived.Kind {
	case FacetMaxLength, FacetTotalDigits, FacetFractionDigits:
		if derived.IntValue <= base.IntValue {
			return nil
		}
		return fmt.Errorf("facet %s: derived value (%d) must be <= base value (%d) to be a valid restriction", derived.Name, derived.IntValue, base.IntValue)
	case FacetMinLength:
		if derived.IntValue >= base.IntValue {
			return nil
		}
		return fmt.Errorf("facet %s: derived value (%d) must be >= base value (%d) to be a valid restriction", derived.Name, derived.IntValue, base.IntValue)
	case FacetLength:
		if derived.IntValue == base.IntValue {
			return nil
		}
		return fmt.Errorf("facet %s: derived value (%d) must equal base value (%d) in a restriction", derived.Name, derived.IntValue, base.IntValue)
	default:
		return nil
	}
}

func compareRangeFacetValues(spec SimpleTypeSpec, left, right string) (int, error) {
	switch spec.Primitive {
	case "duration":
		return compareDurationValues(left, right)
	case "float":
		return CompareFloatFacetValues(left, right)
	case "double":
		return compareDoubleFacetValues(left, right)
	case "dateTime", "time", "date", "gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth":
		return compareDateTimeValues(left, right, spec.Primitive)
	default:
		return compareNumericFacetValues(left, right)
	}
}

var (
	ErrDateTimeNotComparable = errors.New("date/time comparison indeterminate")
	ErrDurationNotComparable = errors.New("duration comparison indeterminate")
	ErrFloatNotComparable    = errors.New("float comparison unordered")
)

func compareDurationValues(left, right string) (int, error) {
	lv, err := value.ParseDuration(left)
	if err != nil {
		return 0, err
	}
	rv, err := value.ParseDuration(right)
	if err != nil {
		return 0, err
	}
	cmp, err := value.CompareDuration(lv, rv)
	if errors.Is(err, value.ErrIndeterminateDurationComparison) {
		return 0, ErrDurationNotComparable
	}
	return cmp, err
}

func CompareFloatFacetValues(left, right string) (int, error) {
	return compareFloatFacetValues(left, right, 32)
}

func compareDoubleFacetValues(left, right string) (int, error) {
	return compareFloatFacetValues(left, right, 64)
}

func compareFloatFacetValues(left, right string, bits int) (int, error) {
	lv, lc, err := num.ParseFloat([]byte(left), bits)
	if err != nil {
		return 0, err
	}
	rv, rc, err := num.ParseFloat([]byte(right), bits)
	if err != nil {
		return 0, err
	}
	cmp, ok := num.CompareFloat(lv, lc, rv, rc)
	if !ok {
		return 0, ErrFloatNotComparable
	}
	return cmp, nil
}

func compareDateTimeValues(left, right, primitive string) (int, error) {
	lv, err := value.ParsePrimitive(primitive, []byte(left))
	if err != nil {
		return 0, err
	}
	rv, err := value.ParsePrimitive(primitive, []byte(right))
	if err != nil {
		return 0, err
	}
	cmp, err := value.Compare(lv, rv)
	if errors.Is(err, value.ErrIndeterminateComparison) {
		return 0, ErrDateTimeNotComparable
	}
	return cmp, err
}

func compareNumericFacetValues(left, right string) (int, error) {
	lv, err := num.ParseDec([]byte(left))
	if err != nil {
		return 0, err
	}
	rv, err := num.ParseDec([]byte(right))
	if err != nil {
		return 0, err
	}
	return lv.Compare(rv), nil
}

func enumFacetValues(facets []FacetSpec) []FacetValue {
	var values []FacetValue
	for _, facet := range facets {
		if facet.Kind == FacetEnumeration {
			values = append(values, facet.Values...)
		}
	}
	return values
}

func enumFacetValueGroups(facets []FacetSpec) [][]FacetValue {
	var groups [][]FacetValue
	for _, facet := range facets {
		if facet.Kind == FacetEnumeration && len(facet.Values) > 0 {
			groups = append(groups, facet.Values)
		}
	}
	return groups
}

func coalesceFacetSpecs(facets []FacetSpec) []FacetSpec {
	var out []FacetSpec
	var pattern *FacetSpec
	enumIndex := -1
	for _, facet := range facets {
		switch facet.Kind {
		case FacetEnumeration:
			if enumIndex == -1 {
				enumIndex = len(out)
				out = append(out, FacetSpec{Kind: FacetEnumeration, Name: facet.Name})
			}
			out[enumIndex].Values = append(out[enumIndex].Values, facet.Values...)
		case FacetPattern:
			if pattern == nil {
				pattern = &FacetSpec{Kind: FacetPattern, Name: facet.Name}
			}
			pattern.Values = append(pattern.Values, facet.Values...)
		default:
			out = append(out, facet)
		}
	}
	if pattern != nil {
		out = append(out, *pattern)
	}
	return out
}

func (r *docResolver) validateRestrictionEnumerations(spec SimpleTypeSpec, ownFacets []FacetSpec) error {
	values := enumFacetValues(ownFacets)
	if len(values) == 0 || isZeroTypeRef(spec.Base) {
		return nil
	}
	allowed, constrained, err := r.allowedEnumValueKeys(spec.Base, make(map[TypeRef]bool))
	if err != nil {
		return err
	}
	if !constrained {
		return nil
	}
	for _, value := range values {
		keys, err := ValueKeysForLexical(value.Lexical, spec, value.Context, r.specForRef)
		if err != nil {
			return err
		}
		if !valueKeysIntersectLookup(keys, allowed) {
			return fmt.Errorf("schema ir: enumeration value %q is not valid for base type %s", value.Lexical, formatName(spec.Base.Name))
		}
	}
	return nil
}

func (r *docResolver) allowedEnumValueKeys(ref TypeRef, seen map[TypeRef]bool) (map[string]struct{}, bool, error) {
	if seen[ref] {
		return nil, false, nil
	}
	seen[ref] = true
	spec, ok := r.specForRef(ref)
	if !ok {
		return nil, false, nil
	}
	if len(enumFacetValueGroups(spec.Facets)) > 0 {
		out, _, err := effectiveEnumValueKeys(spec, r.specForRef)
		return out, true, err
	}
	if spec.Variety != TypeVarietyUnion {
		return nil, false, nil
	}
	out := make(map[string]struct{})
	for _, member := range spec.Members {
		values, constrained, err := r.allowedEnumValueKeys(member, seen)
		if err != nil || !constrained {
			return nil, constrained, err
		}
		for value := range values {
			out[value] = struct{}{}
		}
	}
	return out, true, nil
}

func effectiveEnumValueKeys(spec SimpleTypeSpec, resolve ValueSpecResolver) (map[string]struct{}, bool, error) {
	groups := enumFacetValueGroups(spec.Facets)
	if len(groups) == 0 {
		return nil, false, nil
	}
	var out map[string]struct{}
	for _, group := range groups {
		groupLookup, err := enumFacetGroupValueKeys(group, spec, resolve)
		if err != nil {
			return nil, false, err
		}
		if out == nil {
			out = groupLookup
			continue
		}
		intersectEnumKeySets(out, groupLookup)
	}
	return out, true, nil
}

func enumFacetGroupValueKeys(group []FacetValue, spec SimpleTypeSpec, resolve ValueSpecResolver) (map[string]struct{}, error) {
	out := make(map[string]struct{}, len(group))
	for _, value := range group {
		keys, err := ValueKeysForLexical(value.Lexical, spec, value.Context, resolve)
		if err != nil {
			return nil, err
		}
		addValueKeysToLookup(out, keys)
	}
	return out, nil
}

func intersectEnumKeySets(dst, allowed map[string]struct{}) {
	for key := range dst {
		if _, ok := allowed[key]; !ok {
			delete(dst, key)
		}
	}
}

func addValueKeysToLookup(dst map[string]struct{}, keys []ValueKey) {
	for _, key := range keys {
		dst[valueKeyLookup(key)] = struct{}{}
	}
}

func valueKeysIntersectLookup(keys []ValueKey, lookup map[string]struct{}) bool {
	for _, key := range keys {
		if _, ok := lookup[valueKeyLookup(key)]; ok {
			return true
		}
	}
	return false
}

func valueKeyLookup(key ValueKey) string {
	buf := make([]byte, 1, 1+len(key.Bytes))
	buf[0] = byte(key.Kind)
	buf = append(buf, key.Bytes...)
	return string(buf)
}

func (r *docResolver) simpleItemRef(decl *ast.SimpleTypeDecl) (TypeRef, error) {
	if decl.InlineItem != nil {
		id, err := r.ensureSimpleType(decl.InlineItem, false)
		if err != nil {
			return TypeRef{}, err
		}
		return TypeRef{ID: id, Name: nameFromQName(decl.InlineItem.Name)}, nil
	}
	if decl.ItemType.IsZero() {
		return TypeRef{}, nil
	}
	return r.typeRef(decl.ItemType)
}

func (r *docResolver) simpleMemberRefs(decl *ast.SimpleTypeDecl) ([]TypeRef, error) {
	var refs []TypeRef
	for _, member := range decl.MemberTypes {
		ref, err := r.typeRef(member)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	for i := range decl.InlineMembers {
		id, err := r.ensureSimpleType(&decl.InlineMembers[i], false)
		if err != nil {
			return nil, err
		}
		refs = append(refs, TypeRef{ID: id, Name: nameFromQName(decl.InlineMembers[i].Name)})
	}
	return refs, nil
}

func (r *docResolver) validateListItemRef(ref TypeRef) error {
	if isZeroTypeRef(ref) || ref.Builtin {
		if ref.Builtin && ref.Name.Local == "anyType" {
			return fmt.Errorf("schema ir: list itemType must be a simple type, got %s", formatName(ref.Name))
		}
		return nil
	}
	info, ok, err := r.typeInfoForRef(ref)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("schema ir: list itemType %s not found", formatName(ref.Name))
	}
	if info.Kind != TypeSimple {
		return fmt.Errorf("schema ir: list itemType must be a simple type, got %s", formatName(ref.Name))
	}
	spec, ok := r.specForRef(ref)
	if !ok {
		return fmt.Errorf("schema ir: list itemType %s missing simple type spec", formatName(ref.Name))
	}
	if spec.Variety == TypeVarietyList {
		return fmt.Errorf("schema ir: list itemType must be atomic or union, got list")
	}
	return nil
}

func (r *docResolver) validateUnionMemberRef(ref TypeRef) error {
	if isZeroTypeRef(ref) {
		return nil
	}
	if err := r.requireSimpleTypeRef(ref, fmt.Sprintf("union memberType %s", formatName(ref.Name))); err != nil {
		return err
	}
	return nil
}

func (r *docResolver) facetSpec(facet ast.FacetDecl) (FacetSpec, bool, error) {
	kind := facetKind(facet.Name)
	if kind == FacetUnknown {
		return FacetSpec{}, false, nil
	}
	spec := FacetSpec{
		Kind:  kind,
		Name:  facet.Name,
		Value: facet.Lexical,
	}
	switch kind {
	case FacetEnumeration:
		spec.Values = []FacetValue{{
			Lexical: facet.Lexical,
			Context: r.contextMap(facet.NamespaceContextID),
		}}
	case FacetPattern:
		pattern, err := ast.TranslateXSDPatternToGo(facet.Lexical)
		if err != nil {
			return FacetSpec{}, false, fmt.Errorf("schema ir: invalid pattern facet %q: %w", facet.Lexical, err)
		}
		spec.Values = []FacetValue{{Lexical: pattern}}
	case FacetLength, FacetMinLength, FacetMaxLength, FacetTotalDigits, FacetFractionDigits:
		value, err := strconv.ParseUint(facet.Lexical, 10, 32)
		if err != nil {
			return FacetSpec{}, false, fmt.Errorf("schema ir: invalid %s facet value %q", facet.Name, facet.Lexical)
		}
		spec.IntValue = uint32(value)
	}
	return spec, true, nil
}
