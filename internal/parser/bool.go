package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/xml"
)

func parseBoolAttribute(doc *xsdxml.Document, elem xsdxml.NodeID, name string) (bool, bool, error) {
	if !doc.HasAttribute(elem, name) {
		return false, false, nil
	}
	return parseBoolValue(name, doc.GetAttribute(elem, name), true)
}

func parseBoolValue(name, value string, present bool) (bool, bool, error) {
	if !present {
		return false, false, nil
	}
	switch value {
	case "true", "1":
		return true, true, nil
	case "false", "0":
		return true, false, nil
	default:
		return false, false, fmt.Errorf("invalid %s attribute value '%s': must be 'true', 'false', '1', or '0'", name, value)
	}
}
