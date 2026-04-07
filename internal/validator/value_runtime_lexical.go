package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func parseTemporal(kind runtime.ValidatorKind, lexical []byte) (value.Value, error) {
	spec, ok := runtime.TemporalSpecForValidatorKind(kind)
	if !ok {
		return value.Value{}, fmt.Errorf("unsupported temporal kind %d", kind)
	}
	return value.Parse(spec.Kind, lexical)
}
