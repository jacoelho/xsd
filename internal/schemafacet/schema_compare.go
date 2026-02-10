package schemafacet

import (
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/durationlex"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

// ErrDateTimeNotComparable reports partially ordered date/time comparisons.
var ErrDateTimeNotComparable = errors.New("date/time values are not comparable")

// ErrDurationNotComparable reports partially ordered duration comparisons.
var ErrDurationNotComparable = errors.New("duration values are not comparable")

// ErrFloatNotComparable reports NaN-involved float comparisons.
var ErrFloatNotComparable = errors.New("float values are not comparable")

func isNumericTypeName(typeName string) bool {
	numericTypes := []string{
		"decimal", "float", "double", "integer", "long", "int", "short", "byte",
		"nonNegativeInteger", "positiveInteger", "unsignedLong", "unsignedInt",
		"unsignedShort", "unsignedByte", "nonPositiveInteger", "negativeInteger",
	}
	return slices.Contains(numericTypes, typeName)
}

func isDateTimeTypeName(typeName string) bool {
	switch typeName {
	case "dateTime", "date", "time", "gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth":
		return true
	default:
		return false
	}
}

// CompareFacetValues compares two facet lexical values for a base type.
func CompareFacetValues(val1, val2 string, baseType model.Type) (int, error) {
	var primitiveType model.Type
	if st, ok := baseType.(*model.SimpleType); ok {
		primitiveType = st.PrimitiveType()
		if primitiveType == nil {
			primitiveType = baseType
		}
	} else {
		primitiveType = baseType
	}

	if st, ok := primitiveType.(*model.SimpleType); ok {
		typeName := st.QName.Local
		switch typeName {
		case "duration":
			return compareDurationValues(val1, val2)
		case "float":
			return CompareFloatFacetValues(val1, val2)
		case "double":
			return compareDoubleFacetValues(val1, val2)
		}
		if isNumericTypeName(typeName) {
			return compareNumericFacetValues(val1, val2)
		}
		if facets := st.FundamentalFacets(); facets != nil {
			if facets.Numeric {
				return compareNumericFacetValues(val1, val2)
			}
			if isDateTimeTypeName(typeName) {
				return compareDateTimeValues(val1, val2, typeName)
			}
			if facets.Ordered == model.OrderedTotal {
				return strings.Compare(val1, val2), nil
			}
		}
	}
	if bt, ok := primitiveType.(*model.BuiltinType); ok {
		typeName := bt.Name().Local
		switch typeName {
		case "duration":
			return compareDurationValues(val1, val2)
		case "float":
			return CompareFloatFacetValues(val1, val2)
		case "double":
			return compareDoubleFacetValues(val1, val2)
		}
		if isDateTimeTypeName(typeName) {
			return compareDateTimeValues(val1, val2, typeName)
		}
		if bt.FundamentalFacets() != nil && bt.FundamentalFacets().Numeric {
			return compareNumericFacetValues(val1, val2)
		}
		if bt.FundamentalFacets() != nil && bt.FundamentalFacets().Ordered == model.OrderedTotal {
			return strings.Compare(val1, val2), nil
		}
	}

	if cmp, err := compareNumericFacetValues(val1, val2); err == nil {
		return cmp, nil
	}
	if cmp, err := compareDurationValues(val1, val2); err == nil {
		return cmp, nil
	}
	return strings.Compare(val1, val2), nil
}

func compareNumericFacetValues(val1, val2 string) (int, error) {
	d1, err := value.ParseDecimal([]byte(val1))
	if err != nil {
		return 0, err
	}
	d2, err := value.ParseDecimal([]byte(val2))
	if err != nil {
		return 0, err
	}
	return d1.Compare(d2), nil
}

// CompareFloatFacetValues compares float lexical values with NaN handling.
func CompareFloatFacetValues(val1, val2 string) (int, error) {
	f1, err := value.ParseFloat([]byte(val1))
	if err != nil {
		return 0, err
	}
	f2, err := value.ParseFloat([]byte(val2))
	if err != nil {
		return 0, err
	}
	return compareFloatValues(float64(f1), float64(f2))
}

func compareDoubleFacetValues(val1, val2 string) (int, error) {
	f1, err := value.ParseDouble([]byte(val1))
	if err != nil {
		return 0, err
	}
	f2, err := value.ParseDouble([]byte(val2))
	if err != nil {
		return 0, err
	}
	return compareFloatValues(f1, f2)
}

func compareFloatValues(v1, v2 float64) (int, error) {
	if math.IsNaN(v1) || math.IsNaN(v2) {
		if math.IsNaN(v1) && math.IsNaN(v2) {
			return 0, nil
		}
		return 0, ErrFloatNotComparable
	}
	if v1 < v2 {
		return -1, nil
	}
	if v1 > v2 {
		return 1, nil
	}
	return 0, nil
}

func compareDateTimeValues(v1, v2, baseTypeName string) (int, error) {
	switch baseTypeName {
	case "date":
		t1, err := temporal.Parse(temporal.KindDate, []byte(v1))
		if err != nil {
			return 0, err
		}
		t2, err := temporal.Parse(temporal.KindDate, []byte(v2))
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "dateTime":
		t1, err := temporal.Parse(temporal.KindDateTime, []byte(v1))
		if err != nil {
			return 0, err
		}
		t2, err := temporal.Parse(temporal.KindDateTime, []byte(v2))
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "time":
		t1, err := temporal.Parse(temporal.KindTime, []byte(v1))
		if err != nil {
			return 0, err
		}
		t2, err := temporal.Parse(temporal.KindTime, []byte(v2))
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "gYear":
		t1, err := temporal.Parse(temporal.KindGYear, []byte(v1))
		if err != nil {
			return 0, err
		}
		t2, err := temporal.Parse(temporal.KindGYear, []byte(v2))
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "gYearMonth":
		t1, err := temporal.Parse(temporal.KindGYearMonth, []byte(v1))
		if err != nil {
			return 0, err
		}
		t2, err := temporal.Parse(temporal.KindGYearMonth, []byte(v2))
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "gMonth":
		t1, err := temporal.Parse(temporal.KindGMonth, []byte(v1))
		if err != nil {
			return 0, err
		}
		t2, err := temporal.Parse(temporal.KindGMonth, []byte(v2))
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "gMonthDay":
		t1, err := temporal.Parse(temporal.KindGMonthDay, []byte(v1))
		if err != nil {
			return 0, err
		}
		t2, err := temporal.Parse(temporal.KindGMonthDay, []byte(v2))
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	case "gDay":
		t1, err := temporal.Parse(temporal.KindGDay, []byte(v1))
		if err != nil {
			return 0, err
		}
		t2, err := temporal.Parse(temporal.KindGDay, []byte(v2))
		if err != nil {
			return 0, err
		}
		return compareDateTimeOrder(t1, t2)
	}

	if v1 < v2 {
		return -1, nil
	}
	if v1 > v2 {
		return 1, nil
	}
	return 0, nil
}

func compareDateTimeOrder(t1, t2 temporal.Value) (int, error) {
	cmp, err := temporal.Compare(t1, t2)
	if err != nil {
		return 0, ErrDateTimeNotComparable
	}
	return cmp, nil
}

func compareDurationValues(v1, v2 string) (int, error) {
	left, err := durationlex.Parse(v1)
	if err != nil {
		return 0, err
	}
	right, err := durationlex.Parse(v2)
	if err != nil {
		return 0, err
	}
	cmp, err := model.ComparableXSDDuration{Value: left}.Compare(model.ComparableXSDDuration{Value: right})
	if err != nil {
		return 0, ErrDurationNotComparable
	}
	return cmp, nil
}
