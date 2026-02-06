package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/types"
)

// ResolveGroupReferences expands group references across named groups and content models.
func ResolveGroupReferences(sch *parser.Schema) error {
	if sch == nil {
		return nil
	}
	detector := NewCycleDetector[types.QName]()

	for _, qname := range schema.SortedQNames(sch.Groups) {
		group := sch.Groups[qname]
		if group == nil {
			continue
		}
		if err := detector.WithScope(qname, func() error {
			return resolveGroupRefsInModelGroupWithCycleDetection(group, sch, detector)
		}); err != nil {
			return fmt.Errorf("resolve group refs in group %s: %w", qname, err)
		}
	}

	for _, qname := range schema.SortedQNames(sch.TypeDefs) {
		typ := sch.TypeDefs[qname]
		ct, ok := typ.(*types.ComplexType)
		if !ok {
			continue
		}
		if err := resolveGroupRefsInContentWithVisited(ct.Content(), sch, detector); err != nil {
			return fmt.Errorf("resolve group refs in type %s: %w", ct.QName, err)
		}
	}

	for _, qname := range schema.SortedQNames(sch.ElementDecls) {
		elem := sch.ElementDecls[qname]
		ct, ok := elem.Type.(*types.ComplexType)
		if !ok {
			continue
		}
		if err := resolveGroupRefsInContentWithVisited(ct.Content(), sch, detector); err != nil {
			return fmt.Errorf("resolve group refs in element %s: %w", elem.Name, err)
		}
	}

	return nil
}

func resolveGroupRefsInContentWithVisited(content types.Content, sch *parser.Schema, detector *CycleDetector[types.QName]) error {
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle == nil {
			return nil
		}
		resolved, err := resolveGroupRefsInParticleWithVisited(c.Particle, sch, detector)
		if err != nil {
			return err
		}
		if resolved != nil {
			c.Particle = resolved
		}
	case *types.ComplexContent:
		if c.Restriction != nil && c.Restriction.Particle != nil {
			resolved, err := resolveGroupRefsInParticleWithVisited(c.Restriction.Particle, sch, detector)
			if err != nil {
				return err
			}
			if resolved != nil {
				c.Restriction.Particle = resolved
			}
		}
		if c.Extension != nil && c.Extension.Particle != nil {
			resolved, err := resolveGroupRefsInParticleWithVisited(c.Extension.Particle, sch, detector)
			if err != nil {
				return err
			}
			if resolved != nil {
				c.Extension.Particle = resolved
			}
		}
	}
	return nil
}

func resolveGroupRefsInParticleWithVisited(particle types.Particle, sch *parser.Schema, detector *CycleDetector[types.QName]) (types.Particle, error) {
	groupRef, ok := particle.(*types.GroupRef)
	if ok {
		groupDef, found := sch.Groups[groupRef.RefQName]
		if !found {
			return nil, fmt.Errorf("group '%s' not found", groupRef.RefQName)
		}
		groupCopy := types.CloneModelGroupTree(groupDef)
		groupCopy.MinOccurs = groupRef.MinOccurs
		groupCopy.MaxOccurs = groupRef.MaxOccurs
		return groupCopy, nil
	}

	mg, ok := particle.(*types.ModelGroup)
	if !ok {
		return nil, nil
	}
	if err := resolveGroupRefsInModelGroupWithCycleDetection(mg, sch, detector); err != nil {
		return nil, err
	}
	return nil, nil
}

func resolveGroupRefsInModelGroupWithCycleDetection(mg *types.ModelGroup, sch *parser.Schema, detector *CycleDetector[types.QName]) error {
	return resolveGroupRefsInModelGroupWithPointerCycleDetection(mg, sch, detector, make(map[*types.ModelGroup]bool))
}

func resolveGroupRefsInModelGroupWithPointerCycleDetection(mg *types.ModelGroup, sch *parser.Schema, detector *CycleDetector[types.QName], visitedMGs map[*types.ModelGroup]bool) error {
	if mg == nil || visitedMGs[mg] {
		return nil
	}
	visitedMGs[mg] = true

	for i, particle := range mg.Particles {
		switch typed := particle.(type) {
		case *types.GroupRef:
			if err := detector.Enter(typed.RefQName); err != nil {
				return fmt.Errorf("circular group reference detected: %s", typed.RefQName)
			}
			groupDef, ok := sch.Groups[typed.RefQName]
			if !ok {
				detector.Leave(typed.RefQName)
				return fmt.Errorf("group '%s' not found", typed.RefQName)
			}

			if detector.IsVisited(typed.RefQName) {
				groupCopy := types.CloneModelGroupTree(groupDef)
				groupCopy.MinOccurs = typed.MinOccurs
				groupCopy.MaxOccurs = typed.MaxOccurs
				mg.Particles[i] = groupCopy
				detector.Leave(typed.RefQName)
				continue
			}

			groupCopy := types.CloneModelGroupTree(groupDef)
			groupCopy.MinOccurs = typed.MinOccurs
			groupCopy.MaxOccurs = typed.MaxOccurs
			if err := resolveGroupRefsInModelGroupWithPointerCycleDetection(groupCopy, sch, detector, make(map[*types.ModelGroup]bool)); err != nil {
				detector.Leave(typed.RefQName)
				return err
			}
			mg.Particles[i] = groupCopy
			detector.Leave(typed.RefQName)
		case *types.ModelGroup:
			if err := resolveGroupRefsInModelGroupWithPointerCycleDetection(typed, sch, detector, visitedMGs); err != nil {
				return err
			}
		}
	}
	return nil
}
