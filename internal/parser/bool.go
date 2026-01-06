package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/xml"
)

func parseBoolAttribute(elem xml.Element, name string) (bool, bool, error) {
	if !elem.HasAttribute(name) {
		return false, false, nil
	}
	value := elem.GetAttribute(name)
	switch value {
	case "true", "1":
		return true, true, nil
	case "false", "0":
		return true, false, nil
	default:
		return false, false, fmt.Errorf("invalid %s attribute value '%s': must be 'true', 'false', '1', or '0'", name, value)
	}
}
