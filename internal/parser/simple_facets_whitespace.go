package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func applyWhiteSpaceFacet(doc *xsdxml.Document, elem xsdxml.NodeID, st *types.SimpleType) error {
	if err := validateOnlyAnnotationChildren(doc, elem, "whiteSpace"); err != nil {
		return err
	}
	value := doc.GetAttribute(elem, "value")
	if value == "" {
		return fmt.Errorf("whiteSpace facet missing value")
	}
	switch value {
	case "preserve":
		st.SetWhiteSpaceExplicit(types.WhiteSpacePreserve)
	case "replace":
		st.SetWhiteSpaceExplicit(types.WhiteSpaceReplace)
	case "collapse":
		st.SetWhiteSpaceExplicit(types.WhiteSpaceCollapse)
	default:
		return fmt.Errorf("invalid whiteSpace value: %s", value)
	}
	return nil
}
