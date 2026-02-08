package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

// validateWildcardDerivation validates wildcard constraints in type derivation
func validateWildcardDerivation(schema *parser.Schema, ct *types.ComplexType) error {
	baseQName := ct.Content().BaseTypeQName()
	if baseQName.IsZero() {
		return nil
	}

	baseCT, ok := lookupComplexType(schema, baseQName)
	if !ok {
		return nil
	}

	baseWildcards := collectWildcardsFromContent(baseCT.Content())
	derivedWildcards := collectWildcardsFromContent(ct.Content())

	if ct.IsRestriction() {
		if len(baseWildcards) == 0 && len(derivedWildcards) > 0 {
			return fmt.Errorf("wildcard restriction: cannot add wildcard when base type has no wildcard")
		}
		for _, derivedWildcard := range derivedWildcards {
			foundSubset := false
			for _, baseWildcard := range baseWildcards {
				if wildcardIsSubset(derivedWildcard, baseWildcard) {
					foundSubset = true
					break
				}
			}
			if !foundSubset {
				return fmt.Errorf("wildcard restriction: derived wildcard is not a subset of any base wildcard")
			}
		}
	}

	return nil
}

// collectWildcardsFromContent collects all AnyElement wildcards from content model
func collectWildcardsFromContent(content types.Content) []*types.AnyElement {
	return traversal.CollectFromContent(content, func(p types.Particle) (*types.AnyElement, bool) {
		wildcard, ok := p.(*types.AnyElement)
		return wildcard, ok
	})
}

// wildcardIsSubset checks if wildcard1's namespace constraint is a subset of wildcard2's
// This is used for restriction validation
// w1 is a subset of w2 if every namespace that matches w1 also matches w2
func wildcardIsSubset(w1, w2 *types.AnyElement) bool {
	if !processContentsStrongerOrEqual(w1.ProcessContents, w2.ProcessContents) {
		return false
	}
	return wildcardNamespaceSubset(w1, w2)
}

func wildcardNamespaceSubset(w1, w2 *types.AnyElement) bool {
	return namespaceConstraintSubset(
		w1.Namespace, w1.NamespaceList, w1.TargetNamespace,
		w2.Namespace, w2.NamespaceList, w2.TargetNamespace,
	)
}
