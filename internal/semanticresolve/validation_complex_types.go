package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

func validateComplexTypeReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range traversal.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		ct, ok := typ.(*types.ComplexType)
		if !ok {
			continue
		}
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
			} else if attr.Type != nil {
				if err := validateTypeReferenceFromTypeAtLocation(sch, attr.Type, qname.Namespace, noOriginLocation); err != nil {
					errs = append(errs, fmt.Errorf("type %s attribute: %w", qname, err))
				}
			}
		}

		origin := sch.TypeOrigins[qname]
		if err := traversal.WalkContentParticles(ct.Content(), func(particle types.Particle) error {
			return validateParticleReferences(sch, particle, origin)
		}); err != nil {
			errs = append(errs, fmt.Errorf("type %s: %w", qname, err))
		}
	}

	return errs
}

func collectComplexTypeAttrGroupRefs(ct *types.ComplexType) []types.QName {
	if ct == nil {
		return nil
	}
	out := make([]types.QName, 0, len(ct.AttrGroups))
	out = append(out, ct.AttrGroups...)

	if cc, ok := ct.Content().(*types.ComplexContent); ok {
		if cc.Extension != nil {
			out = append(out, cc.Extension.AttrGroups...)
		}
		if cc.Restriction != nil {
			out = append(out, cc.Restriction.AttrGroups...)
		}
	}
	if sc, ok := ct.Content().(*types.SimpleContent); ok {
		if sc.Extension != nil {
			out = append(out, sc.Extension.AttrGroups...)
		}
	}
	return out
}
