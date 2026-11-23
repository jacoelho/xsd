package schema

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/xsderrors"
)

// upaViolation is the one Unique Particle Attribution diagnostic. It names the
// complex type and the two competing particles: a content-model diagnostic is
// located at the innermost compositor or component scope rather than at the
// type (see the rejected alternatives in ARCHITECTURE.md). Element particles
// come before wildcards, elements in expanded-name order and wildcards in text
// order, so the message does not depend on the order a check compared them in.
func upaViolation(rt *schemaBuild, typ complexTypeLabel, overlap particleOverlap) error {
	if err := checkUPAViolation(rt, typ, overlap); err != nil {
		return err
	}
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, fmt.Sprintf("UPA violation in %s: %s overlap%s",
		typ.label(&rt.Names), upaPairText(rt, overlap), upaOverlapSuffix(rt, overlap)))
}

// upaPairText renders each particle once and lists the pair in diagnostic
// order.
func upaPairText(rt *schemaBuild, overlap particleOverlap) string {
	left := upaParticleText(rt, overlap.left)
	right := upaParticleText(rt, overlap.right)
	if compareUPAParticles(rt, overlap.left, left, overlap.right, right) > 0 {
		return fmt.Sprintf("%s and %s", right, left)
	}
	return fmt.Sprintf("%s and %s", left, right)
}

// checkUPAViolation rejects a label or particle the message cannot name, so
// the renderers below read only resolvable facts.
func checkUPAViolation(rt *schemaBuild, typ complexTypeLabel, overlap particleOverlap) error {
	if !typ.valid() {
		return xsderrors.InternalInvariant("UPA violation names no complex type")
	}
	for _, particle := range [2]contentParticle{overlap.left, overlap.right} {
		switch particle.Kind {
		case particleElement:
			if _, ok := rt.ElementName(particle.Element); !ok {
				return xsderrors.InternalInvariant("UPA violation references missing element declaration")
			}
		case particleWildcard:
			if _, ok := wildcardByID(rt.Wildcards, particle.Wildcard); !ok {
				return xsderrors.InternalInvariant("UPA violation references missing wildcard")
			}
		case particleModel:
			return xsderrors.InternalInvariant("UPA violation names a model group particle")
		}
	}
	return nil
}

func compareUPAParticles(rt *schemaBuild, a contentParticle, aText string, b contentParticle, bText string) int {
	aWildcard := a.Kind == particleWildcard
	bWildcard := b.Kind == particleWildcard
	switch {
	case aWildcard && bWildcard:
		return strings.Compare(aText, bText)
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
// wildcard by the elements it admits.
func upaParticleText(rt *schemaBuild, particle contentParticle) string {
	if particle.Kind == particleWildcard {
		wildcard, _ := wildcardByID(rt.Wildcards, particle.Wildcard)
		return fmt.Sprintf("wildcard for %s", wildcardExpectation(&rt.Names, wildcard).WildcardDescription())
	}
	name, _ := rt.ElementName(particle.Element)
	return fmt.Sprintf("element %s", rt.Names.Format(name))
}

// upaOverlapSuffix names what the particles overlap on when their own
// rendering does not show it: the substitution-group member when they share
// no declaration name, or for two wildcards a namespace both admit, the first
// in text order of a finite namespace list. Two wildcards that both admit
// unboundedly many namespaces have no single namespace to name.
func upaOverlapSuffix(rt *schemaBuild, overlap particleOverlap) string {
	if overlap.left.Kind == particleWildcard && overlap.right.Kind == particleWildcard {
		left, _ := wildcardByID(rt.Wildcards, overlap.left.Wildcard)
		right, _ := wildcardByID(rt.Wildcards, overlap.right.Wildcard)
		return upaWildcardOverlapSuffix(&rt.Names, left, right)
	}
	if upaOverlapsOnMember(rt, overlap) {
		return " on substitution group member " + rt.Names.Format(overlap.name)
	}
	return ""
}

// upaOverlapsOnMember reports whether the common name is the declaration name
// of neither particle, so the message does not already show it: it came from
// a substitution group.
func upaOverlapsOnMember(rt *schemaBuild, overlap particleOverlap) bool {
	for _, particle := range [2]contentParticle{overlap.left, overlap.right} {
		if particle.Kind != particleElement {
			continue
		}
		if name, _ := rt.ElementName(particle.Element); name == overlap.name {
			return false
		}
	}
	return true
}

func upaWildcardOverlapSuffix(names *nameTable, a, b wildcardRuntime) string {
	finite, other := a, b
	if !wildcardFinite(finite) {
		finite, other = b, a
	}
	if !wildcardFinite(finite) {
		return ""
	}
	namespaces := finite.Namespaces
	if finite.Mode == WildcardLocal {
		namespaces = []namespaceID{emptyNamespaceID}
	}
	found := false
	var first string
	for _, ns := range namespaces {
		uri := names.Namespace(ns)
		if wildcardAllowsNamespace(other, ns) && (!found || compareNamespaceURI(uri, first) < 0) {
			first, found = uri, true
		}
	}
	if !found {
		return ""
	}
	return " on namespace " + diagnostictext.Quote(first)
}

func wildcardFinite(wildcard wildcardRuntime) bool {
	switch wildcard.Mode {
	case WildcardLocal, WildcardTargetNamespace, WildcardList:
		return true
	case WildcardAny, WildcardOther:
	}
	return false
}
