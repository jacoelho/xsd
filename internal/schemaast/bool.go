package schemaast

import (
	"fmt"
)

func parseBoolValue(name, value string) (bool, error) {
	value = ApplyWhiteSpace(value, WhiteSpaceCollapse)
	switch value {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid %s attribute value '%s': must be 'true', 'false', '1', or '0'", name, value)
	}
}
