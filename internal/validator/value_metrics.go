package validator

import (
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
)

// ValueMetrics captures cached parsed values and derived keys for one validation.
type ValueMetrics struct {
	intVal          num.Int
	keyBytes        []byte
	decVal          num.Dec
	fractionDigits  int
	totalDigits     int
	listCount       int
	length          int
	float64Val      float64
	float32Val      float32
	actualTypeID    runtime.TypeID
	actualValidator runtime.ValidatorID
	patternChecked  bool
	enumChecked     bool
	keySet          bool
	decSet          bool
	intSet          bool
	float32Set      bool
	float64Set      bool
	listSet         bool
	digitsSet       bool
	lengthSet       bool
	float32Class    num.FloatClass
	keyKind         runtime.ValueKind
	float64Class    num.FloatClass
}

type valueOptions struct {
	applyWhitespace  bool
	trackIDs         bool
	requireCanonical bool
	storeValue       bool
	needKey          bool
}
