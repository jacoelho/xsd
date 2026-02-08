package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

func validateAttributeGroupReferencesInSchema(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range sortedQNames(sch.AttributeGroups) {
		ag := sch.AttributeGroups[qname]
		for _, agRef := range ag.AttrGroups {
			if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
				errs = append(errs, err)
			}
		}

		for _, attr := range ag.Attributes {
			if attr.IsReference {
				if err := validateAttributeReference(sch, qname, attr, "attributeGroup"); err != nil {
					errs = append(errs, err)
				}
			}
		}

		for _, attr := range ag.Attributes {
			if attr.Type != nil {
				if err := validateTypeReferenceFromType(sch, attr.Type, qname.Namespace); err != nil {
					errs = append(errs, fmt.Errorf("attributeGroup %s attribute %s: %w", qname, attr.Name, err))
				}
			}
		}
	}

	return errs
}
