package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func applyWhiteSpaceFacet(doc *xmltree.Document, elem xmltree.NodeID, st *model.SimpleType) error {
	if err := validateOnlyAnnotationChildren(doc, elem, "whiteSpace"); err != nil {
		return err
	}
	value := doc.GetAttribute(elem, "value")
	if value == "" {
		return fmt.Errorf("whiteSpace facet missing value")
	}
	switch value {
	case "preserve":
		st.SetWhiteSpaceExplicit(model.WhiteSpacePreserve)
	case "replace":
		st.SetWhiteSpaceExplicit(model.WhiteSpaceReplace)
	case "collapse":
		st.SetWhiteSpaceExplicit(model.WhiteSpaceCollapse)
	default:
		return fmt.Errorf("invalid whiteSpace value: %s", value)
	}
	return nil
}
