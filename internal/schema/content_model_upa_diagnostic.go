package schema

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/diagnostictext"
)

// upaViolationMessage names the complex type and the two competing particles.
// A content-model diagnostic is located at the innermost compositor or
// component scope rather than at the type, so the message names the type (see
// the rejected alternatives in ARCHITECTURE.md). Element particles come before
// wildcards, elements in expanded-name order and wildcards in text order, so
// the message does not depend on the order a check compared them in.
func upaViolationMessage(rt *schemaBuild, typ complexTypeLabel, overlap particleOverlap) string {
	first, second := overlap.left, overlap.right
	if compareUPAParticles(rt, first, second) > 0 {
		first, second = second, first
	}
	if upaOverlapsOnMember(rt, overlap) {
		return fmt.Sprintf("UPA violation in %s: %s and %s overlap on substitution group member %s",
			typ.label(&rt.Names), upaParticleText(rt, first), upaParticleText(rt, second), rt.Names.Format(overlap.name))
	}
	return fmt.Sprintf("UPA violation in %s: %s and %s overlap",
		typ.label(&rt.Names), upaParticleText(rt, first), upaParticleText(rt, second))
}

// upaOverlapsOnMember reports whether the common name is not the declaration
// name of an element particle in the pair, so it came from a substitution
// group.
func upaOverlapsOnMember(rt *schemaBuild, overlap particleOverlap) bool {
	for _, particle := range [2]contentParticle{overlap.left, overlap.right} {
		if particle.Kind != particleElement {
			continue
		}
		if name, _ := rt.ElementName(particle.Element); name != overlap.name {
			return true
		}
	}
	return false
}

func compareUPAParticles(rt *schemaBuild, a, b contentParticle) int {
	aWildcard := a.Kind == particleWildcard
	bWildcard := b.Kind == particleWildcard
	switch {
	case aWildcard && bWildcard:
		return strings.Compare(upaParticleText(rt, a), upaParticleText(rt, b))
	case aWildcard:
		return 1
	case bWildcard:
		return -1
	default:
		aName, _ := rt.ElementName(a.Element)
		bName, _ := rt.ElementName(b.Element)
		return compareQNameText(&rt.Names, aName, bName)
	}
}

// upaParticleText renders an element particle by its declaration's name and a
// wildcard by its namespace constraint.
func upaParticleText(rt *schemaBuild, particle contentParticle) string {
	if particle.Kind == particleWildcard {
		wildcard, _ := wildcardByID(rt.Wildcards, particle.Wildcard)
		return fmt.Sprintf("wildcard %s", upaWildcardNamespaces(&rt.Names, wildcard))
	}
	name, _ := rt.ElementName(particle.Element)
	return fmt.Sprintf("element %s", rt.Names.Format(name))
}

func upaWildcardNamespaces(names *nameTable, wildcard wildcardRuntime) string {
	switch wildcard.Mode {
	case WildcardAny:
		return "##any"
	case WildcardOther:
		return fmt.Sprintf("##other (%s)", upaTargetNamespace(names, wildcard.OtherThan))
	case WildcardLocal:
		return "##local"
	case WildcardTargetNamespace:
		if len(wildcard.Namespaces) != 1 {
			return "##targetNamespace"
		}
		return fmt.Sprintf("##targetNamespace (%s)", upaTargetNamespace(names, wildcard.Namespaces[0]))
	case WildcardList:
		return upaNamespaceList(names, wildcard.Namespaces)
	default:
		return "with an unknown namespace constraint"
	}
}

func upaTargetNamespace(names *nameTable, ns namespaceID) string {
	if ns == emptyNamespaceID {
		return "no target namespace"
	}
	return fmt.Sprintf("target namespace %s", diagnostictext.Quote(names.Namespace(ns)))
}

// upaNamespaceList lists the namespaces in text order.
func upaNamespaceList(names *nameTable, ids []namespaceID) string {
	uris := make([]string, len(ids))
	for i, id := range ids {
		uris[i] = names.Namespace(id)
	}
	sortNamespaceURIs(uris)
	return diagnostictext.QuoteList(uris)
}
