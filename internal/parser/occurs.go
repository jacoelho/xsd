package parser

import (
	"fmt"
	"math/big"

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
	if value == "" {
		return types.OccursFromInt(0), fmt.Errorf("%s attribute cannot be empty", attr)
	}
	if value == "unbounded" {
		if attr == "minOccurs" {
			return types.OccursFromInt(0), fmt.Errorf("minOccurs attribute cannot be 'unbounded'")
		}
		return types.OccursUnbounded, nil
	}
	bi, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return types.OccursFromInt(0), fmt.Errorf("invalid %s attribute value '%s'", attr, value)
	}
	if bi.Sign() < 0 {
		return types.OccursFromInt(0), fmt.Errorf("invalid %s attribute value '%s'", attr, value)
	}
	occurs, err := types.OccursFromBig(bi)
	if err != nil {
		return types.OccursFromInt(0), fmt.Errorf("invalid %s attribute value '%s'", attr, value)
	}
	return occurs, nil
}
