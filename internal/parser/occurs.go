package parser

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/schemaxml"
)

func parseOccursAttr(doc *schemaxml.Document, elem schemaxml.NodeID, attr string) (model.Occurs, error) {
	if !doc.HasAttribute(elem, attr) {
		return model.OccursFromInt(1), nil
	}

	return parseOccursValue(attr, doc.GetAttribute(elem, attr))
}

func parseOccursValue(attr, value string) (model.Occurs, error) {
	value = model.ApplyWhiteSpace(value, model.WhiteSpaceCollapse)
	if value == "" {
		return model.OccursFromInt(0), fmt.Errorf("%s attribute cannot be empty", attr)
	}
	if value == "unbounded" {
		if attr == "minOccurs" {
			return model.OccursFromInt(0), fmt.Errorf("minOccurs attribute cannot be 'unbounded'")
		}
		return model.OccursUnbounded, nil
	}
	u, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return model.OccursFromInt(0), fmt.Errorf("%w: %s attribute value '%s' overflows uint32", model.ErrOccursOverflow, attr, value)
		}
		return model.OccursFromInt(0), fmt.Errorf("invalid %s attribute value '%s'", attr, value)
	}
	return model.OccursFromUint64(u), nil
}
