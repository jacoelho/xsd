package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/typechain"
)

// validateWildcardDerivation validates wildcard constraints in type derivation
func validateWildcardDerivation(schema *parser.Schema, ct *model.ComplexType) error {
	baseQName := ct.Content().BaseTypeQName()
	if baseQName.IsZero() {
		return nil
	}

	baseCT, ok := typechain.LookupComplexType(schema, baseQName)
	if !ok {
		return nil
	}

	baseWildcards := traversal.CollectFromContent(baseCT.Content(), func(p model.Particle) (*model.AnyElement, bool) {
		wildcard, ok := p.(*model.AnyElement)
		return wildcard, ok
	})
	derivedWildcards := traversal.CollectFromContent(ct.Content(), func(p model.Particle) (*model.AnyElement, bool) {
		wildcard, ok := p.(*model.AnyElement)
		return wildcard, ok
	})

	if ct.IsRestriction() {
		if len(baseWildcards) == 0 && len(derivedWildcards) > 0 {
			return fmt.Errorf("wildcard restriction: cannot add wildcard when base type has no wildcard")
		}
		for _, derivedWildcard := range derivedWildcards {
			foundSubset := false
			for _, baseWildcard := range baseWildcards {
				if model.ProcessContentsStrongerOrEqual(derivedWildcard.ProcessContents, baseWildcard.ProcessContents) &&
					model.NamespaceConstraintSubset(
						derivedWildcard.Namespace, derivedWildcard.NamespaceList, derivedWildcard.TargetNamespace,
						baseWildcard.Namespace, baseWildcard.NamespaceList, baseWildcard.TargetNamespace,
					) {
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
