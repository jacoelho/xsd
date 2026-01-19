package validator

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/types"
)

func (r *streamRun) validateQNameEnumeration(value string, enum *types.Enumeration, scopeDepth int) error {
	if enum == nil {
		return nil
	}
	qname, err := r.parseQNameValue(value, scopeDepth)
	if err != nil {
		return err
	}
	allowedQNames, err := enumerationQNameValues(enum)
	if err != nil {
		return err
	}
	if slices.Contains(allowedQNames, qname) {
		return nil
	}
	return fmt.Errorf("value %s not in enumeration: %s", value, types.FormatEnumerationValues(enum.Values))
}

func enumerationQNameValues(enum *types.Enumeration) ([]types.QName, error) {
	if enum == nil || len(enum.Values) == 0 {
		return nil, nil
	}
	if len(enum.QNameValues) == len(enum.Values) {
		return enum.QNameValues, nil
	}
	return enum.ResolveQNameValues()
}
