package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateAttributeGroupReferencesInSchema(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range model.SortedMapKeys(sch.AttributeGroups) {
		ag := sch.AttributeGroups[qname]
		errs = append(errs, validateAttributeGroupReferences(sch, qname, ag)...)
		errs = append(errs, validateAttributeGroupAttributeReferences(sch, qname, ag)...)
		errs = append(errs, validateAttributeGroupAttributeTypes(sch, qname, ag)...)
	}

	return errs
}

func validateAttributeGroupReferences(sch *parser.Schema, qname model.QName, ag *model.AttributeGroup) []error {
	var errs []error
	for _, agRef := range ag.AttrGroups {
		if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func validateAttributeGroupAttributeReferences(sch *parser.Schema, qname model.QName, ag *model.AttributeGroup) []error {
	var errs []error
	for _, attr := range ag.Attributes {
		if !attr.IsReference {
			continue
		}
		if err := validateAttributeReference(sch, qname, attr, "attributeGroup"); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func validateAttributeGroupAttributeTypes(sch *parser.Schema, qname model.QName, ag *model.AttributeGroup) []error {
	var errs []error
	for _, attr := range ag.Attributes {
		if attr.Type == nil {
			continue
		}
		if err := validateTypeReferenceFromTypeAtLocation(sch, attr.Type, qname.Namespace, noOriginLocation); err != nil {
			errs = append(errs, fmt.Errorf("attributeGroup %s attribute %s: %w", qname, attr.Name, err))
		}
	}
	return errs
}
