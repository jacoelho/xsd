package schema

import (
	"strconv"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"

	"github.com/jacoelho/xsd/xsderrors"
)

const maxUint32Value = uint64(^uint32(0))
const facetTotalDigits = "totalDigits"

// facetLabel renders a facet name for diagnostics. Recognised facet names are
// vocabulary constants; anything else is schema text and is quoted.
func facetLabel(name string) string {
	if _, ok := facetMaskForLocal(name); ok {
		return name
	}
	return diagnostictext.Quote(name)
}

// parseSizeFacetValue parses length/minLength/maxLength/totalDigits/
// fractionDigits facet values.
func parseSizeFacetValue(name, value string) (uint32, error) {
	value = lex.CollapseXMLWhitespace(value)
	size, err := parseSchemaNonNegativeInteger(value)
	if err != nil {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "invalid "+facetLabel(name)+" facet "+diagnostictext.Quote(value))
	}
	if name == facetTotalDigits && size == 0 {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "totalDigits must be positive")
	}
	if size > maxUint32Value {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, facetLabel(name)+" facet exceeds uint32 limit")
	}
	return uint32(size), nil
}

// parseSchemaNonNegativeInteger is exact through uint32. Larger results signal
// overflow; every remaining byte is still checked for valid integer syntax.
func parseSchemaNonNegativeInteger(value string) (uint64, error) {
	start, negative, err := parseSizeFacetSign(value)
	if err != nil {
		return 0, err
	}
	if start == len(value) {
		return 0, strconv.ErrSyntax
	}
	digitStart := skipSchemaLeadingZeros(value, start, len(value))
	if negative && digitStart != len(value) {
		return 0, strconv.ErrSyntax
	}
	if digitStart == len(value) {
		return 0, nil
	}
	return parseSizeFacetDigits(value, digitStart)
}

func parseSizeFacetSign(value string) (int, bool, error) {
	if value == "" {
		return 0, false, strconv.ErrSyntax
	}
	if value[0] == '+' {
		return 1, false, nil
	}
	if value[0] == '-' {
		return 1, true, nil
	}
	return 0, false, nil
}

func parseSizeFacetDigits(value string, start int) (uint64, error) {
	var n uint64
	for i := start; i < len(value); i++ {
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

func skipSchemaLeadingZeros(s string, start, end int) int {
	for start < end && s[start] == '0' {
		start++
	}
	return start
}
