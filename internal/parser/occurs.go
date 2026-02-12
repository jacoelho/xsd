package parser

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func parseOccursAttr(doc *xmltree.Document, elem xmltree.NodeID, attr string) (occurs.Occurs, error) {
	if !doc.HasAttribute(elem, attr) {
		return occurs.OccursFromInt(1), nil
	}

	return parseOccursValue(attr, doc.GetAttribute(elem, attr))
}

func parseOccursValue(attr, value string) (occurs.Occurs, error) {
	value = model.ApplyWhiteSpace(value, model.WhiteSpaceCollapse)
	if value == "" {
		return occurs.OccursFromInt(0), fmt.Errorf("%s attribute cannot be empty", attr)
	}
	if value == "unbounded" {
		if attr == "minOccurs" {
			return occurs.OccursFromInt(0), fmt.Errorf("minOccurs attribute cannot be 'unbounded'")
		}
		return occurs.OccursUnbounded, nil
	}
	u, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return occurs.OccursFromInt(0), fmt.Errorf("%w: %s attribute value '%s' overflows uint32", occurs.ErrOccursOverflow, attr, value)
		}
		return occurs.OccursFromInt(0), fmt.Errorf("invalid %s attribute value '%s'", attr, value)
	}
	return occurs.OccursFromUint64(u), nil
}
