package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemaops"
	"github.com/jacoelho/xsd/internal/types"
)

// ResolveGroupReferences expands group references across named groups and content models.
func ResolveGroupReferences(sch *parser.Schema) error {
	if sch == nil {
		return nil
	}

	options := schemaops.ExpandGroupRefsOptions{
		Lookup: func(ref *types.GroupRef) *types.ModelGroup {
			if ref == nil {
				return nil
			}
			return sch.Groups[ref.RefQName]
		},
		MissingError: func(ref types.QName) error {
			return fmt.Errorf("group '%s' not found", ref)
		},
		CycleError: func(ref types.QName) error {
			return fmt.Errorf("circular group reference detected: %s", ref)
		},
		AllGroupMode: schemaops.AllGroupKeep,
		LeafClone:    schemaops.LeafReuse,
	}

	for _, qname := range sortedQNames(sch.Groups) {
		group := sch.Groups[qname]
		if group == nil {
			continue
		}
		expanded, err := schemaops.ExpandGroupRefs(group, options)
		if err != nil {
			return fmt.Errorf("resolve group refs in group %s: %w", qname, err)
		}
		expandedGroup, ok := expanded.(*types.ModelGroup)
		if !ok || expandedGroup == nil {
			return fmt.Errorf("resolve group refs in group %s: expanded group is nil", qname)
		}
		sch.Groups[qname] = expandedGroup
	}

	for _, qname := range sortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		ct, ok := typ.(*types.ComplexType)
		if !ok {
			continue
		}
		if err := resolveGroupRefsInContent(ct.Content(), options); err != nil {
			return fmt.Errorf("resolve group refs in type %s: %w", ct.QName, err)
		}
	}

	for _, qname := range sortedQNames(sch.ElementDecls) {
		elem := sch.ElementDecls[qname]
		ct, ok := elem.Type.(*types.ComplexType)
		if !ok {
			continue
		}
		if err := resolveGroupRefsInContent(ct.Content(), options); err != nil {
			return fmt.Errorf("resolve group refs in element %s: %w", elem.Name, err)
		}
	}

	return nil
}

func resolveGroupRefsInContent(content types.Content, options schemaops.ExpandGroupRefsOptions) error {
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle == nil {
			return nil
		}
		expanded, err := schemaops.ExpandGroupRefs(c.Particle, options)
		if err != nil {
			return err
		}
		if expanded != nil {
			c.Particle = expanded
		}
	case *types.ComplexContent:
		if c.Restriction != nil && c.Restriction.Particle != nil {
			expanded, err := schemaops.ExpandGroupRefs(c.Restriction.Particle, options)
			if err != nil {
				return err
			}
			if expanded != nil {
				c.Restriction.Particle = expanded
			}
		}
		if c.Extension != nil && c.Extension.Particle != nil {
			expanded, err := schemaops.ExpandGroupRefs(c.Extension.Particle, options)
			if err != nil {
				return err
			}
			if expanded != nil {
				c.Extension.Particle = expanded
			}
		}
	}
	return nil
}
