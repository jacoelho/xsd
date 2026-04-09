package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// ResolveGroupReferences expands group references across named groups and content models.
func ResolveGroupReferences(sch *parser.Schema) error {
	if sch == nil {
		return nil
	}

	options := contentmodel.ExpandGroupRefsOptions{
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
		AllGroupMode: contentmodel.AllGroupKeep,
		LeafClone:    contentmodel.LeafReuse,
	}

	for _, name := range model.SortedMapKeys(sch.Groups) {
		group := sch.Groups[name]
		if group == nil {
			continue
		}
		expanded, err := contentmodel.ExpandGroupRefs(group, options)
		if err != nil {
			return fmt.Errorf("resolve group refs in group %s: %w", name, err)
		}
		expandedGroup, ok := expanded.(*model.ModelGroup)
		if !ok || expandedGroup == nil {
			return fmt.Errorf("resolve group refs in group %s: expanded group is nil", name)
		}
		sch.Groups[name] = expandedGroup
	}

	for _, name := range model.SortedMapKeys(sch.TypeDefs) {
		typ := sch.TypeDefs[name]
		ct, ok := typ.(*model.ComplexType)
		if !ok {
			continue
		}
		if err := resolveGroupRefsInContent(ct.Content(), options); err != nil {
			return fmt.Errorf("resolve group refs in type %s: %w", ct.QName, err)
		}
	}

	for _, name := range model.SortedMapKeys(sch.ElementDecls) {
		elem := sch.ElementDecls[name]
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

func resolveGroupRefsInContent(content model.Content, options contentmodel.ExpandGroupRefsOptions) error {
	switch c := content.(type) {
	case *model.ElementContent:
		if c.Particle == nil {
			return nil
		}
		expanded, err := contentmodel.ExpandGroupRefs(c.Particle, options)
		if err != nil {
			return err
		}
		if expanded != nil {
			c.Particle = expanded
		}
	case *model.ComplexContent:
		if c.Restriction != nil && c.Restriction.Particle != nil {
			expanded, err := contentmodel.ExpandGroupRefs(c.Restriction.Particle, options)
			if err != nil {
				return err
			}
			if expanded != nil {
				c.Restriction.Particle = expanded
			}
		}
		if c.Extension != nil && c.Extension.Particle != nil {
			expanded, err := contentmodel.ExpandGroupRefs(c.Extension.Particle, options)
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
