package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateEnumerationFacetValuesWithIndex(sch *parser.Schema, index *iterationIndex) []error {
	var errs []error
	if index == nil {
		index = buildIterationIndex(sch)
	}

	for _, qname := range index.typeQNames {
		st, ok := sch.TypeDefs[qname].(*model.SimpleType)
		if !ok || st == nil || st.Restriction == nil {
			continue
		}
		baseType := st.ResolvedBase
		if baseType == nil && !st.Restriction.Base.IsZero() {
			baseType = parser.ResolveSimpleTypeReferenceAllowMissing(sch, st.Restriction.Base)
		}
		if baseType == nil {
			continue
		}
		for _, facet := range st.Restriction.Facets {
			enum, ok := facet.(*model.Enumeration)
			if !ok {
				continue
			}
			values := enum.Values()
			contexts := enum.ValueContexts()
			for i, val := range values {
				var ctx map[string]string
				if i < len(contexts) {
					ctx = contexts[i]
				}
				if err := validateDefaultOrFixedValueResolved(sch, val, baseType, ctx, idValuesAllowed); err != nil {
					errs = append(errs, fmt.Errorf("type %s: restriction: enumeration value %d (%q) is not valid for base type %s: %w", qname, i+1, val, baseType.Name().Local, err))
				}
			}
		}
	}

	return errs
}

func validateInlineTypeReferencesWithIndex(sch *parser.Schema, index *iterationIndex) []error {
	var errs []error
	if index == nil {
		index = buildIterationIndex(sch)
	}

	for _, qname := range index.elementQNames {
		errs = append(errs, validateInlineElementTypeReferences(sch, qname)...)
	}

	return errs
}

func validateInlineElementTypeReferences(sch *parser.Schema, qname model.QName) []error {
	decl := sch.ElementDecls[qname]
	if decl == nil || decl.Type == nil || decl.Type.IsBuiltin() {
		return nil
	}
	if _, exists := sch.TypeDefs[decl.Type.Name()]; exists {
		return nil
	}

	var errs []error
	if err := validateTypeReferences(sch, qname, decl.Type); err != nil {
		errs = append(errs, fmt.Errorf("element %s inline type: %w", qname, err))
	}
	if ct, ok := decl.Type.(*model.ComplexType); ok {
		errs = append(errs, validateInlineComplexTypeReferences(sch, qname, ct)...)
	}
	return errs
}

func validateInlineComplexTypeReferences(sch *parser.Schema, qname model.QName, ct *model.ComplexType) []error {
	var errs []error
	for _, agRef := range ct.AttrGroups {
		if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
			errs = append(errs, err)
		}
	}
	for _, attr := range ct.Attributes() {
		if !attr.IsReference {
			continue
		}
		if err := validateAttributeReference(sch, qname, attr, "element"); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
