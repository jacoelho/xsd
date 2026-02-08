package semanticcheck

import (
	facetengine "github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/types"
)

var errDateTimeNotComparable = facetengine.ErrDateTimeNotComparable
var errDurationNotComparable = facetengine.ErrDurationNotComparable
var errFloatNotComparable = facetengine.ErrFloatNotComparable

type rangeFacetInfo struct {
	minValue     string
	maxValue     string
	minInclusive bool
	maxInclusive bool
	hasMin       bool
	hasMax       bool
}

func builtinRangeFacetInfoFor(typeName string) (rangeFacetInfo, bool) {
	switch typeName {
	case "positiveInteger":
		return rangeFacetInfo{minValue: "1", minInclusive: true, hasMin: true}, true
	case "nonNegativeInteger":
		return rangeFacetInfo{minValue: "0", minInclusive: true, hasMin: true}, true
	case "negativeInteger":
		return rangeFacetInfo{maxValue: "-1", maxInclusive: true, hasMax: true}, true
	case "nonPositiveInteger":
		return rangeFacetInfo{maxValue: "0", maxInclusive: true, hasMax: true}, true
	case "byte":
		return rangeFacetInfo{minValue: "-128", minInclusive: true, maxValue: "127", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "short":
		return rangeFacetInfo{minValue: "-32768", minInclusive: true, maxValue: "32767", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "int":
		return rangeFacetInfo{minValue: "-2147483648", minInclusive: true, maxValue: "2147483647", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "long":
		return rangeFacetInfo{minValue: "-9223372036854775808", minInclusive: true, maxValue: "9223372036854775807", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "unsignedByte":
		return rangeFacetInfo{minValue: "0", minInclusive: true, maxValue: "255", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "unsignedShort":
		return rangeFacetInfo{minValue: "0", minInclusive: true, maxValue: "65535", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "unsignedInt":
		return rangeFacetInfo{minValue: "0", minInclusive: true, maxValue: "4294967295", maxInclusive: true, hasMin: true, hasMax: true}, true
	case "unsignedLong":
		return rangeFacetInfo{minValue: "0", minInclusive: true, maxValue: "18446744073709551615", maxInclusive: true, hasMin: true, hasMax: true}, true
	default:
		return rangeFacetInfo{}, false
	}
}

func implicitRangeFacetsForBuiltin(bt *types.BuiltinType) []types.Facet {
	info, ok := builtinRangeFacetInfoFor(bt.Name().Local)
	if !ok {
		return nil
	}
	var result []types.Facet
	if info.hasMin {
		if info.minInclusive {
			if facet, err := types.NewMinInclusive(info.minValue, bt); err == nil {
				result = append(result, facet)
			}
		} else if facet, err := types.NewMinExclusive(info.minValue, bt); err == nil {
			result = append(result, facet)
		}
	}
	if info.hasMax {
		if info.maxInclusive {
			if facet, err := types.NewMaxInclusive(info.maxValue, bt); err == nil {
				result = append(result, facet)
			}
		} else if facet, err := types.NewMaxExclusive(info.maxValue, bt); err == nil {
			result = append(result, facet)
		}
	}
	return result
}

func extractRangeFacetInfo(facetsList []types.Facet) rangeFacetInfo {
	var info rangeFacetInfo
	for _, facet := range facetsList {
		lex, ok := facet.(types.LexicalFacet)
		if !ok {
			continue
		}
		val := lex.GetLexical()
		if val == "" {
			continue
		}
		switch facet.Name() {
		case "minInclusive":
			info.minValue = val
			info.minInclusive = true
			info.hasMin = true
		case "minExclusive":
			info.minValue = val
			info.minInclusive = false
			info.hasMin = true
		case "maxInclusive":
			info.maxValue = val
			info.maxInclusive = true
			info.hasMax = true
		case "maxExclusive":
			info.maxValue = val
			info.maxInclusive = false
			info.hasMax = true
		}
	}
	return info
}
