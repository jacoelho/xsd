package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateComplexTypeReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range model.SortedMapKeys(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		ct, ok := typ.(*model.ComplexType)
		if !ok {
			continue
		}
		errs = append(errs, validateComplexTypeAttributeReferences(sch, qname, ct)...)
		if err := validateComplexTypeParticleReferences(sch, qname, ct, sch.TypeOrigins[qname]); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func validateComplexTypeAttributeReferences(sch *parser.Schema, qname model.QName, ct *model.ComplexType) []error {
	var errs []error

	for _, agRef := range collectComplexTypeAttrGroupRefs(ct) {
		if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
			errs = append(errs, err)
		}
	}

	for _, attr := range ct.Attributes() {
		if attr.IsReference {
			if err := validateAttributeReference(sch, qname, attr, "type"); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if attr.Type == nil {
			continue
		}
		if err := validateTypeReferenceFromTypeAtLocation(sch, attr.Type, qname.Namespace, noOriginLocation); err != nil {
			errs = append(errs, fmt.Errorf("type %s attribute: %w", qname, err))
		}
	}

	return errs
}

func validateComplexTypeParticleReferences(sch *parser.Schema, qname model.QName, ct *model.ComplexType, origin string) error {
	if err := model.WalkContentParticles(ct.Content(), func(particle model.Particle) error {
		return validateParticleReferences(sch, particle, origin)
	}); err != nil {
		return fmt.Errorf("type %s: %w", qname, err)
	}

	return nil
}

func collectComplexTypeAttrGroupRefs(ct *model.ComplexType) []model.QName {
	if ct == nil {
		return nil
	}
	out := make([]model.QName, 0, len(ct.AttrGroups))
	out = append(out, ct.AttrGroups...)

	if cc, ok := ct.Content().(*model.ComplexContent); ok {
		if cc.Extension != nil {
			out = append(out, cc.Extension.AttrGroups...)
		}
		if cc.Restriction != nil {
			out = append(out, cc.Restriction.AttrGroups...)
		}
	}
	if sc, ok := ct.Content().(*model.SimpleContent); ok {
		if sc.Extension != nil {
			out = append(out, sc.Extension.AttrGroups...)
		}
	}
	return out
}
