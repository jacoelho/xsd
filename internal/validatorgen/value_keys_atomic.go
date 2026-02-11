package validatorgen

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/valuesemantics"
)

func (c *compiler) keyBytesAtomic(normalized string, typ model.Type, ctx map[string]string) (keyBytes, error) {
	primName, err := c.res.primitiveName(typ)
	if err != nil {
		return keyBytes{}, err
	}
	if primName == "decimal" && c.res.isIntegerDerived(typ) {
		intVal, parseErr := parseInt(normalized)
		if parseErr != nil {
			return keyBytes{}, parseErr
		}
		if validateErr := runtime.ValidateIntegerKind(c.integerKindForType(typ), intVal); validateErr != nil {
			return keyBytes{}, validateErr
		}
		return keyBytes{kind: runtime.VKDecimal, bytes: num.EncodeDecKey(nil, intVal.AsDec())}, nil
	}

	kind, bytes, err := valuesemantics.KeyForPrimitiveName(primName, normalized, ctx)
	if err != nil {
		return keyBytes{}, err
	}
	return keyBytes{kind: kind, bytes: bytes}, nil
}

func parseInt(normalized string) (num.Int, error) {
	val, perr := num.ParseInt([]byte(normalized))
	if perr != nil {
		return num.Int{}, fmt.Errorf("invalid integer")
	}
	return val, nil
}
