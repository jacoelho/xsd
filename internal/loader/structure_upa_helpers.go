package loader

import "slices"

import "github.com/jacoelho/xsd/internal/types"

// wildcardsOverlap checks if two wildcards have overlapping namespace constraints
func wildcardsOverlap(w1, w2 *types.AnyElement) bool {
	// two wildcards overlap if there's at least one namespace that matches both
	// this is a simplified check - for exact UPA validation, we'd need to check
	// if they're in a choice group and can both match the same element

	// if either wildcard is ##any, they overlap (##any matches everything)
	if w1.Namespace == types.NSCAny || w2.Namespace == types.NSCAny {
		return true
	}
	// two ##other wildcards always overlap (intersection excludes both target namespaces).
	if w1.Namespace == types.NSCOther && w2.Namespace == types.NSCOther {
		return true
	}

	// check if the intersection of the two wildcards is non-empty
	// if intersection exists, they overlap
	intersected := types.IntersectAnyElement(w1, w2)
	return intersected != nil
}

// collectPossibleLastLeafParticles collects particles that could be the last leaf particles
// in a particle structure (used for UPA validation in sequences)
func collectPossibleLastLeafParticles(particle types.Particle, visited map[*types.ModelGroup]bool) []types.Particle {
	switch p := particle.(type) {
	case *types.ElementDecl, *types.AnyElement:
		return []types.Particle{p}
	case *types.ModelGroup:
		if visited[p] {
			return nil
		}
		visited[p] = true
		var out []types.Particle
		switch p.Kind {
		case types.Sequence:
			for i := len(p.Particles) - 1; i >= 0; i-- {
				child := p.Particles[i]
				out = append(out, collectPossibleLastLeafParticles(child, visited)...)
				if child.MinOcc() > 0 {
					break
				}
			}
		case types.Choice, types.AllGroup:
			for _, child := range p.Particles {
				out = append(out, collectPossibleLastLeafParticles(child, visited)...)
			}
		}
		return out
	}
	return nil
}

// collectPossibleFirstLeafParticles collects particles that could be the first leaf particles
// in a particle structure (used for UPA validation in sequences).
func collectPossibleFirstLeafParticles(particle types.Particle, visited map[*types.ModelGroup]bool) []types.Particle {
	switch p := particle.(type) {
	case *types.ElementDecl, *types.AnyElement:
		return []types.Particle{p}
	case *types.ModelGroup:
		if visited[p] {
			return nil
		}
		visited[p] = true
		var out []types.Particle
		switch p.Kind {
		case types.Sequence:
			for _, child := range p.Particles {
				out = append(out, collectPossibleFirstLeafParticles(child, visited)...)
				if child.MinOcc() > 0 {
					break
				}
			}
		case types.Choice, types.AllGroup:
			for _, child := range p.Particles {
				out = append(out, collectPossibleFirstLeafParticles(child, visited)...)
			}
		}
		return out
	}
	return nil
}

// wildcardOverlapsElement checks if a wildcard's namespace constraint overlaps with an explicit element's namespace
func wildcardOverlapsElement(wildcard *types.AnyElement, elemDecl *types.ElementDecl) bool {
	elemNS := elemDecl.Name.Namespace

	// check if element's namespace matches wildcard's namespace constraint
	return namespaceMatchesWildcard(elemNS, wildcard.Namespace, wildcard.NamespaceList, wildcard.TargetNamespace)
}

// namespaceMatchesWildcard checks if a namespace matches a wildcard namespace constraint
// This is used for schema validation (UPA checking), not instance validation
func namespaceMatchesWildcard(ns types.NamespaceURI, constraint types.NamespaceConstraint, namespaceList []types.NamespaceURI, targetNS types.NamespaceURI) bool {
	switch constraint {
	case types.NSCAny:
		return true // matches any namespace
	case types.NSCOther:
		return !ns.IsEmpty() && ns != targetNS // matches any non-empty namespace except target namespace
	case types.NSCTargetNamespace:
		return ns == targetNS // matches only target namespace
	case types.NSCLocal:
		return ns.IsEmpty() // matches only empty namespace
	case types.NSCList:
		// check if namespace is in the list
		return slices.Contains(namespaceList, ns)
	default:
		return false
	}
}