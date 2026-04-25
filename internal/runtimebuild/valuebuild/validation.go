package valuebuild

import (
	"bytes"
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/num"
)

func (c *artifactCompiler) validatePartialFacets(normalized string, spec schemair.SimpleTypeSpec, facets []schemair.FacetSpec, _ map[string]string) error {
	for _, facet := range facets {
		if shouldSkipLengthFacet(spec, facet) {
			continue
		}
		if op, ok := rangeFacetOp(facet.Kind); ok {
			if err := c.validateRangeFacet(op, normalized, spec, facet.Value); err != nil {
				return err
			}
			continue
		}
		switch facet.Kind {
		case schemair.FacetEnumeration:
			continue
		case schemair.FacetPattern:
			if err := validatePatternFacet(facet, normalized); err != nil {
				return err
			}
		case schemair.FacetLength, schemair.FacetMinLength, schemair.FacetMaxLength:
			if err := validateLengthFacet(facet, normalized, spec); err != nil {
				return err
			}
		case schemair.FacetTotalDigits, schemair.FacetFractionDigits:
			if err := validateDigitsFacet(facet, normalized, spec); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *artifactCompiler) validateMemberFacets(normalized string, spec schemair.SimpleTypeSpec, facets []schemair.FacetSpec, ctx map[string]string, includeEnum bool) error {
	for _, facet := range facets {
		if !includeEnum && facet.Kind == schemair.FacetEnumeration {
			continue
		}
		if facet.Kind == schemair.FacetEnumeration {
			if err := validateEnumerationFacet(c, facet, normalized, spec, ctx); err != nil {
				return err
			}
			continue
		}
		if err := c.validatePartialFacets(normalized, spec, []schemair.FacetSpec{facet}, ctx); err != nil {
			return err
		}
	}
	return nil
}

func validatePatternFacet(facet schemair.FacetSpec, normalized string) error {
	values := make([]string, 0, len(facet.Values))
	for _, value := range facet.Values {
		values = append(values, value.Lexical)
	}
	if len(values) == 0 {
		return nil
	}
	source := values[0]
	if len(values) > 1 {
		bodies := make([]string, 0, len(values))
		for _, value := range values {
			bodies = append(bodies, stripAnchors(value))
		}
		source = "^(?:" + stringsJoin(bodies, "|") + ")$"
	}
	re, err := regexp.Compile(source)
	if err != nil {
		return err
	}
	if !re.MatchString(normalized) {
		return fmt.Errorf("pattern violation")
	}
	return nil
}

func stringsJoin(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, part := range parts[1:] {
		out += sep + part
	}
	return out
}

func validateLengthFacet(facet schemair.FacetSpec, normalized string, spec schemair.SimpleTypeSpec) error {
	kind, err := validatorKind(spec)
	if err != nil {
		return err
	}
	length, err := valueLength(kind, normalized)
	if err != nil {
		return err
	}
	switch facet.Kind {
	case schemair.FacetLength:
		if length != int(facet.IntValue) {
			return fmt.Errorf("length violation")
		}
	case schemair.FacetMinLength:
		if length < int(facet.IntValue) {
			return fmt.Errorf("minLength violation")
		}
	case schemair.FacetMaxLength:
		if length > int(facet.IntValue) {
			return fmt.Errorf("maxLength violation")
		}
	}
	return nil
}

func validateDigitsFacet(facet schemair.FacetSpec, normalized string, spec schemair.SimpleTypeSpec) error {
	canon, err := canonicalizeAtomic(normalized, spec, nil)
	if err != nil {
		return err
	}
	total, fraction, err := digitCounts(spec, canon)
	if err != nil {
		return err
	}
	switch facet.Kind {
	case schemair.FacetTotalDigits:
		if total > int(facet.IntValue) {
			return fmt.Errorf("totalDigits violation")
		}
	case schemair.FacetFractionDigits:
		if fraction > int(facet.IntValue) {
			return fmt.Errorf("fractionDigits violation")
		}
	}
	return nil
}

func (c *artifactCompiler) validateRangeFacet(op runtime.FacetOp, normalized string, spec schemair.SimpleTypeSpec, boundLexical string) error {
	canon, err := canonicalizeAtomic(normalized, spec, nil)
	if err != nil {
		return err
	}
	boundNorm := c.normalizeLexical(boundLexical, spec)
	bound, err := canonicalizeAtomic(boundNorm, spec, nil)
	if err != nil {
		return err
	}
	kind, err := validatorKind(spec)
	if err != nil {
		return err
	}
	cmp, err := compareCanonical(kind, canon, bound)
	if err != nil {
		return err
	}
	ok := false
	switch op {
	case runtime.FMinInclusive:
		ok = cmp >= 0
	case runtime.FMaxInclusive:
		ok = cmp <= 0
	case runtime.FMinExclusive:
		ok = cmp > 0
	case runtime.FMaxExclusive:
		ok = cmp < 0
	}
	if !ok {
		return fmt.Errorf("range violation")
	}
	return nil
}

func validateEnumerationFacet(c *artifactCompiler, facet schemair.FacetSpec, normalized string, spec schemair.SimpleTypeSpec, ctx map[string]string) error {
	keys, err := c.irValueKeysForNormalized(normalized, normalized, spec, ctx)
	if err != nil {
		return err
	}
	for _, candidate := range keys {
		for _, value := range facet.Values {
			enumNorm := c.normalizeLexical(value.Lexical, spec)
			enumKeys, err := c.irValueKeysForNormalized(value.Lexical, enumNorm, spec, value.Context)
			if err != nil {
				return err
			}
			for _, enumKey := range enumKeys {
				if candidate.Kind == enumKey.Kind && bytes.Equal(candidate.Bytes, enumKey.Bytes) {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("value not in enumeration")
}

func (c *artifactCompiler) validateEnumSets(lexical, normalized string, spec schemair.SimpleTypeSpec, ctx map[string]string) error {
	validatorID, err := c.compileSpec(spec)
	if err != nil {
		return err
	}
	enumIDs := c.enumIDsForValidator(validatorID)
	if len(enumIDs) == 0 {
		return nil
	}
	keys, err := c.irValueKeysForNormalized(lexical, normalized, spec, ctx)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return fmt.Errorf("value not in enumeration")
	}
	table := c.enums.table()
	for _, key := range keys {
		kind, err := runtimeValueKind(key.Kind)
		if err != nil {
			return err
		}
		matched := true
		for _, enumID := range enumIDs {
			if !runtime.EnumContains(&table, enumID, kind, key.Bytes) {
				matched = false
				break
			}
		}
		if matched {
			return nil
		}
	}
	return fmt.Errorf("value not in enumeration")
}

func (c *artifactCompiler) enumIDsForValidator(id runtime.ValidatorID) []runtime.EnumID {
	if id == 0 || int(id) >= len(c.bundle.Meta) {
		return nil
	}
	meta := c.bundle.Meta[id]
	if meta.Facets.Len == 0 {
		return nil
	}
	start := meta.Facets.Off
	var out []runtime.EnumID
	for i := uint32(0); i < meta.Facets.Len; i++ {
		instr := c.facets[start+i]
		if instr.Op == runtime.FEnum {
			out = append(out, runtime.EnumID(instr.Arg0))
		}
	}
	return out
}

func shouldSkipLengthFacet(spec schemair.SimpleTypeSpec, facet schemair.FacetSpec) bool {
	if facet.Kind != schemair.FacetLength && facet.Kind != schemair.FacetMinLength && facet.Kind != schemair.FacetMaxLength {
		return false
	}
	if spec.Variety == schemair.TypeVarietyList {
		return false
	}
	return spec.QNameOrNotation
}

func valueLength(kind runtime.ValidatorKind, normalized string) (int, error) {
	switch kind {
	case runtime.VList:
		count := 0
		for range value.FieldsXMLWhitespaceStringSeq(normalized) {
			count++
		}
		return count, nil
	case runtime.VHexBinary:
		decoded, err := value.ParseHexBinary([]byte(normalized))
		if err != nil {
			return 0, fmt.Errorf("invalid hexBinary")
		}
		return len(decoded), nil
	case runtime.VBase64Binary:
		decoded, err := value.ParseBase64Binary([]byte(normalized))
		if err != nil {
			return 0, fmt.Errorf("invalid base64Binary")
		}
		return len(decoded), nil
	default:
		return utf8.RuneCountInString(normalized), nil
	}
}

func digitCounts(spec schemair.SimpleTypeSpec, canonical []byte) (int, int, error) {
	kind, err := validatorKind(spec)
	if err != nil {
		return 0, 0, err
	}
	switch kind {
	case runtime.VDecimal:
		val, err := num.ParseDec(canonical)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid decimal")
		}
		return len(val.Coef), int(val.Scale), nil
	case runtime.VInteger:
		val, err := num.ParseInt(canonical)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid integer")
		}
		return len(val.Digits), 0, nil
	default:
		return 0, 0, fmt.Errorf("digits facet not applicable")
	}
}

func compareCanonical(kind runtime.ValidatorKind, left, right []byte) (int, error) {
	switch kind {
	case runtime.VDecimal:
		l, err := num.ParseDec(left)
		if err != nil {
			return 0, fmt.Errorf("invalid decimal")
		}
		r, err := num.ParseDec(right)
		if err != nil {
			return 0, fmt.Errorf("invalid decimal")
		}
		return l.Compare(r), nil
	case runtime.VInteger:
		l, err := num.ParseInt(left)
		if err != nil {
			return 0, fmt.Errorf("invalid integer")
		}
		r, err := num.ParseInt(right)
		if err != nil {
			return 0, fmt.Errorf("invalid integer")
		}
		return l.Compare(r), nil
	case runtime.VFloat:
		l, lc, err := num.ParseFloat32(left)
		if err != nil {
			return 0, fmt.Errorf("invalid float")
		}
		r, rc, err := num.ParseFloat32(right)
		if err != nil {
			return 0, fmt.Errorf("invalid float")
		}
		if lc == num.FloatNaN || rc == num.FloatNaN {
			return 0, fmt.Errorf("range violation")
		}
		cmp, _ := num.CompareFloat(float64(l), lc, float64(r), rc)
		return cmp, nil
	case runtime.VDouble:
		l, lc, err := num.ParseFloat(left, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid double")
		}
		r, rc, err := num.ParseFloat(right, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid double")
		}
		if lc == num.FloatNaN || rc == num.FloatNaN {
			return 0, fmt.Errorf("range violation")
		}
		cmp, _ := num.CompareFloat(l, lc, r, rc)
		return cmp, nil
	case runtime.VDuration:
		l, err := value.ParseDuration(string(left))
		if err != nil {
			return 0, err
		}
		r, err := value.ParseDuration(string(right))
		if err != nil {
			return 0, err
		}
		return value.CompareDuration(l, r)
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		l, err := parseTemporal(kind, left)
		if err != nil {
			return 0, err
		}
		r, err := parseTemporal(kind, right)
		if err != nil {
			return 0, err
		}
		return value.Compare(l, r)
	default:
		return 0, fmt.Errorf("unsupported comparable type %d", kind)
	}
}

func parseTemporal(kind runtime.ValidatorKind, lexical []byte) (value.Value, error) {
	primitive := ""
	switch kind {
	case runtime.VDateTime:
		primitive = "dateTime"
	case runtime.VTime:
		primitive = "time"
	case runtime.VDate:
		primitive = "date"
	case runtime.VGYearMonth:
		primitive = "gYearMonth"
	case runtime.VGYear:
		primitive = "gYear"
	case runtime.VGMonthDay:
		primitive = "gMonthDay"
	case runtime.VGDay:
		primitive = "gDay"
	case runtime.VGMonth:
		primitive = "gMonth"
	default:
		return value.Value{}, fmt.Errorf("unsupported temporal kind %d", kind)
	}
	parsed, err := value.ParsePrimitive(primitive, lexical)
	if err != nil {
		return value.Value{}, err
	}
	return parsed, nil
}
