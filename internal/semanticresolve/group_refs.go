package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/grouprefs"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
)

// ResolveGroupReferences expands group references across named groups and content models.
func ResolveGroupReferences(sch *parser.Schema) error {
	if sch == nil {
		return nil
	}

	options := grouprefs.ExpandGroupRefsOptions{
		Lookup: func(ref *model.GroupRef) *model.ModelGroup {
			if ref == nil {
				return nil
			}
			return sch.Groups[ref.RefQName]
		},
		MissingError: func(ref model.QName) error {
			return fmt.Errorf("group '%s' not found", ref)
		},
		CycleError: func(ref model.QName) error {
			return fmt.Errorf("circular group reference detected: %s", ref)
		},
		AllGroupMode: grouprefs.AllGroupKeep,
		LeafClone:    grouprefs.LeafReuse,
	}

	for _, qname := range traversal.SortedQNames(sch.Groups) {
		group := sch.Groups[qname]
		if group == nil {
			continue
		}
		expanded, err := grouprefs.ExpandGroupRefs(group, options)
		if err != nil {
			return fmt.Errorf("resolve group refs in group %s: %w", qname, err)
		}
		expandedGroup, ok := expanded.(*model.ModelGroup)
		if !ok || expandedGroup == nil {
			return fmt.Errorf("resolve group refs in group %s: expanded group is nil", qname)
		}
		sch.Groups[qname] = expandedGroup
	}

	for _, qname := range traversal.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		ct, ok := typ.(*model.ComplexType)
		if !ok {
			continue
		}
		if err := resolveGroupRefsInContent(ct.Content(), options); err != nil {
			return fmt.Errorf("resolve group refs in type %s: %w", ct.QName, err)
		}
	}

	for _, qname := range traversal.SortedQNames(sch.ElementDecls) {
		elem := sch.ElementDecls[qname]
		ct, ok := elem.Type.(*model.ComplexType)
		if !ok {
			continue
		}
		if err := resolveGroupRefsInContent(ct.Content(), options); err != nil {
			return fmt.Errorf("resolve group refs in element %s: %w", elem.Name, err)
		}
	}

	return nil
}

func resolveGroupRefsInContent(content model.Content, options grouprefs.ExpandGroupRefsOptions) error {
	switch c := content.(type) {
	case *model.ElementContent:
		if c.Particle == nil {
			return nil
		}
		expanded, err := grouprefs.ExpandGroupRefs(c.Particle, options)
		if err != nil {
			return err
		}
		if expanded != nil {
			c.Particle = expanded
		}
	case *model.ComplexContent:
		if c.Restriction != nil && c.Restriction.Particle != nil {
			expanded, err := grouprefs.ExpandGroupRefs(c.Restriction.Particle, options)
			if err != nil {
				return err
			}
			if expanded != nil {
				c.Restriction.Particle = expanded
			}
		}
		if c.Extension != nil && c.Extension.Particle != nil {
			expanded, err := grouprefs.ExpandGroupRefs(c.Extension.Particle, options)
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
