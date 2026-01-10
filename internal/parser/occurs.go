package parser

import (
	"fmt"
	"math/big"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

func parseOccursAttr(doc *xsdxml.Document, elem xsdxml.NodeID, attr string, defaultValue int) (int, error) {
	if !doc.HasAttribute(elem, attr) {
		return defaultValue, nil
	}

	return parseOccursValue(attr, doc.GetAttribute(elem, attr), true, defaultValue)
}

func parseOccursValue(attr, value string, present bool, defaultValue int) (int, error) {
	if !present {
		return defaultValue, nil
	}
	if value == "" {
		return 0, fmt.Errorf("%s attribute cannot be empty", attr)
	}
	if value == "unbounded" {
		if attr == "minOccurs" {
			return 0, fmt.Errorf("minOccurs attribute cannot be 'unbounded'")
		}
		return types.UnboundedOccurs, nil
	}
	bi, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return 0, fmt.Errorf("invalid %s attribute value '%s'", attr, value)
	}
	if bi.Sign() < 0 {
		return 0, fmt.Errorf("invalid %s attribute value '%s'", attr, value)
	}
	maxInt := int(^uint(0) >> 1)
	if bi.Cmp(big.NewInt(int64(maxInt))) > 0 {
		return maxInt, nil
	}
	return int(bi.Int64()), nil
}
