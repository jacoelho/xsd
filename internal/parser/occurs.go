package parser

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

func parseOccursAttr(doc *xsdxml.Document, elem xsdxml.NodeID, attr string) (types.Occurs, error) {
	if !doc.HasAttribute(elem, attr) {
		return types.OccursFromInt(1), nil
	}

	return parseOccursValue(attr, doc.GetAttribute(elem, attr))
}

func parseOccursValue(attr, value string) (types.Occurs, error) {
	value = types.ApplyWhiteSpace(value, types.WhiteSpaceCollapse)
	if value == "" {
		return types.OccursFromInt(0), fmt.Errorf("%s attribute cannot be empty", attr)
	}
	if value == "unbounded" {
		if attr == "minOccurs" {
			return types.OccursFromInt(0), fmt.Errorf("minOccurs attribute cannot be 'unbounded'")
		}
		return types.OccursUnbounded, nil
	}
	u, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return types.OccursFromInt(0), fmt.Errorf("%w: %s attribute value '%s' overflows uint32", types.ErrOccursOverflow, attr, value)
		}
		return types.OccursFromInt(0), fmt.Errorf("invalid %s attribute value '%s'", attr, value)
	}
	return types.OccursFromInt(int(u)), nil
}
