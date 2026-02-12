package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func parseBoolAttribute(doc *xmltree.Document, elem xmltree.NodeID, name string) (bool, bool, error) {
	if !doc.HasAttribute(elem, name) {
		return false, false, nil
	}
	value, err := parseBoolValue(name, doc.GetAttribute(elem, name))
	if err != nil {
		return false, false, err
	}
	return true, value, nil
}

func parseBoolValue(name, value string) (bool, error) {
	value = model.ApplyWhiteSpace(value, model.WhiteSpaceCollapse)
	switch value {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid %s attribute value '%s': must be 'true', 'false', '1', or '0'", name, value)
	}
}
