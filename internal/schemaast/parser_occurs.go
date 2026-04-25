package schemaast

import (
	"errors"
	"fmt"
	"strconv"
)

func parseOccursValue(attr, value string) (Occurs, error) {
	value = ApplyWhiteSpace(value, WhiteSpaceCollapse)
	if value == "" {
		return OccursFromInt(0), fmt.Errorf("%s attribute cannot be empty", attr)
	}
	if value == "unbounded" {
		if attr == "minOccurs" {
			return OccursFromInt(0), fmt.Errorf("minOccurs attribute cannot be 'unbounded'")
		}
		return OccursUnbounded, nil
	}
	u, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return OccursFromInt(0), fmt.Errorf("%w: %s attribute value '%s' overflows uint32", ErrOccursOverflow, attr, value)
		}
		return OccursFromInt(0), fmt.Errorf("invalid %s attribute value '%s'", attr, value)
	}
	return OccursFromUint64(u), nil
}
