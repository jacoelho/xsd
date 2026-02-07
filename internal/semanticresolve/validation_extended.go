package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	schemacheck "github.com/jacoelho/xsd/internal/semanticcheck"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

func validateEnumerationFacetValues(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range sortedQNames(sch.TypeDefs) {
		st, ok := sch.TypeDefs[qname].(*types.SimpleType)
		if !ok || st == nil || st.Restriction == nil {
			continue
		}
		baseType := st.ResolvedBase
		if baseType == nil && !st.Restriction.Base.IsZero() {
			baseType = typeops.ResolveSimpleTypeReferenceAllowMissing(sch, st.Restriction.Base)
		}
		if baseType == nil {
			continue
		}
		for _, facet := range st.Restriction.Facets {
			enum, ok := facet.(*types.Enumeration)
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
				if err := validateDefaultOrFixedValueResolved(sch, val, baseType, ctx, make(map[types.Type]bool), idValuesAllowed); err != nil {
					errs = append(errs, fmt.Errorf("type %s: restriction: enumeration value %d (%q) is not valid for base type %s: %w", qname, i+1, val, baseType.Name().Local, err))
				}
			}
		}
	}

	return errs
}

func validateDeferredRangeFacetValues(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range sortedQNames(sch.TypeDefs) {
		st, ok := sch.TypeDefs[qname].(*types.SimpleType)
		if !ok || st == nil || st.Restriction == nil {
			continue
		}

		baseType := st.ResolvedBase
		if baseType == nil && !st.Restriction.Base.IsZero() {
			baseType = typeops.ResolveSimpleTypeReferenceAllowMissing(sch, st.Restriction.Base)
		}
		if baseType == nil {
			continue
		}

		var (
			rangeFacets  []types.Facet
			seenDeferred bool
		)

		for _, facet := range st.Restriction.Facets {
			switch f := facet.(type) {
			case types.Facet:
				if isRangeFacetName(f.Name()) {
					rangeFacets = append(rangeFacets, f)
				}
			case *types.DeferredFacet:
				if !isRangeFacetName(f.FacetName) {
					continue
				}
				seenDeferred = true
				resolved, err := typeops.DefaultDeferredFacetConverter(f, baseType)
				if err != nil {
					errs = append(errs, fmt.Errorf("type %s: restriction: %w", qname, err))
					continue
				}
				if resolved != nil {
					rangeFacets = append(rangeFacets, resolved)
				}
			}
		}

		if !seenDeferred || len(rangeFacets) == 0 {
			continue
		}

		baseQName := st.Restriction.Base
		if baseQName.IsZero() {
			baseQName = baseType.Name()
		}
		if err := schemacheck.ValidateFacetConstraints(sch, rangeFacets, baseType, baseQName); err != nil {
			errs = append(errs, fmt.Errorf("type %s: restriction: %w", qname, err))
		}
	}

	return errs
}

func isRangeFacetName(name string) bool {
	switch name {
	case "minInclusive", "maxInclusive", "minExclusive", "maxExclusive":
		return true
	default:
		return false
	}
}

func validateInlineTypeReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range sortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if decl.Type != nil && !decl.Type.IsBuiltin() {
			// skip if the type is a reference to a named type (already validated above)
			if _, exists := sch.TypeDefs[decl.Type.Name()]; !exists {
				if err := validateTypeReferences(sch, qname, decl.Type); err != nil {
					errs = append(errs, fmt.Errorf("element %s inline type: %w", qname, err))
				}
				// also validate attribute group references for inline complex types
				if ct, ok := decl.Type.(*types.ComplexType); ok {
					for _, agRef := range ct.AttrGroups {
						if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
							errs = append(errs, err)
						}
					}
					for _, attr := range ct.Attributes() {
						if attr.IsReference {
							if err := validateAttributeReference(sch, qname, attr, "element"); err != nil {
								errs = append(errs, err)
							}
						}
					}
				}
			}
		}
	}

	return errs
}

func validateComplexTypeReferences(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range sortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		ct, ok := typ.(*types.ComplexType)
		if !ok {
			continue
		}
		for _, agRef := range ct.AttrGroups {
			if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
				errs = append(errs, err)
			}
		}

		if cc, ok := ct.Content().(*types.ComplexContent); ok {
			if cc.Extension != nil {
				for _, agRef := range cc.Extension.AttrGroups {
					if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
						errs = append(errs, err)
					}
				}
			}
			if cc.Restriction != nil {
				for _, agRef := range cc.Restriction.AttrGroups {
					if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}
		if sc, ok := ct.Content().(*types.SimpleContent); ok {
			if sc.Extension != nil {
				for _, agRef := range sc.Extension.AttrGroups {
					if err := validateAttributeGroupReference(sch, agRef, qname); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}

		for _, attr := range ct.Attributes() {
			if attr.IsReference {
				if err := validateAttributeReference(sch, qname, attr, "type"); err != nil {
					errs = append(errs, err)
				}
			} else if attr.Type != nil {
				if err := validateTypeReferenceFromType(sch, attr.Type, qname.Namespace); err != nil {
					errs = append(errs, fmt.Errorf("type %s attribute: %w", qname, err))
				}
			}
		}

		origin := sch.TypeOrigins[qname]
		if err := validateContentReferences(sch, ct.Content(), origin); err != nil {
			errs = append(errs, fmt.Errorf("type %s: %w", qname, err))
		}
	}

	return errs
}

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

func validateLocalElementValueConstraints(sch *parser.Schema) []error {
	var errs []error

	seenLocal := make(map[*types.ElementDecl]bool)
	validateLocals := func(ct *types.ComplexType) {
		for _, elem := range schemacheck.CollectAllElementDeclarationsFromType(sch, ct) {
			if elem == nil || elem.IsReference {
				continue
			}
			if seenLocal[elem] {
				continue
			}
			seenLocal[elem] = true
			if err := validateElementValueConstraints(sch, elem); err != nil {
				errs = append(errs, fmt.Errorf("local element %s: %w", elem.Name, err))
			}
		}
	}
	for _, qname := range sortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[qname]
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			validateLocals(ct)
		}
	}
	for _, qname := range sortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		if ct, ok := typ.(*types.ComplexType); ok {
			validateLocals(ct)
		}
	}

	return errs
}

func validateGroupReferencesInSchema(sch *parser.Schema) []error {
	var errs []error

	for _, qname := range sortedQNames(sch.Groups) {
		group := sch.Groups[qname]
		if err := validateGroupReferences(sch, qname, group); err != nil {
			errs = append(errs, fmt.Errorf("group %s: %w", qname, err))
		}
	}

	return errs
}
