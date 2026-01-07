package validator

import (
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

func (r *validationRun) isSubstitutableQName(actual, declared types.QName) bool {
	if actual == declared {
		return true
	}
	if subs := r.schema.SubstitutionGroup(declared); len(subs) > 0 {
		for _, sub := range subs {
			if sub.QName == actual {
				return true
			}
		}
	}
	return false
}

func (r *validationRun) substitutionDeclForQName(declared, actual types.QName, declaredElem *grammar.CompiledElement) *grammar.CompiledElement {
	if actual == declared {
		return declaredElem
	}
	if subs := r.schema.SubstitutionGroup(declared); len(subs) > 0 {
		for _, sub := range subs {
			if sub.QName == actual {
				return sub
			}
		}
	}
	return declaredElem
}

func (r *validationRun) resolveSubstitutionDecl(actualQName types.QName, declared *grammar.CompiledElement) *grammar.CompiledElement {
	if actualQName == declared.QName {
		return declared
	}
	actualDecl := r.findElementDeclaration(actualQName)
	if actualDecl == nil {
		return declared
	}
	matcher := r.matcher()
	if matcher.IsSubstitutable(actualQName, declared.QName) {
		return actualDecl
	}
	return declared
}

// substitutionMatcher implements contentmodel.SymbolMatcher for substitution groups.
type substitutionMatcher struct {
	view schemaView
}

func (r *validationRun) matcher() *substitutionMatcher {
	r.subMatcher.view = r.schema
	return &r.subMatcher
}

// IsSubstitutable reports whether actual can substitute for declared.
func (m *substitutionMatcher) IsSubstitutable(actual, declared types.QName) bool {
	if actual == declared {
		return true
	}

	// find the head element - declared might be a substitute itself, so check substitution groups
	head := m.view.Element(declared)
	if head == nil {
		head = m.view.SubstitutionGroupHead(declared)
		if head == nil {
			return false
		}
		declared = head.QName
	}

	if head.Block.Has(types.DerivationSubstitution) {
		return false
	}

	if subs := m.view.SubstitutionGroup(declared); len(subs) > 0 {
		for _, sub := range subs {
			if sub.QName == actual {
				if m.isDerivationBlocked(sub, head) {
					return false
				}
				return true
			}
		}
	}
	return false
}

// isDerivationBlocked checks if the substitute element's type derivation is blocked by the head.
// Per XSD spec, blocking can come from:
// 1. Element's block attribute
// 2. Type's block attribute (the head element's type)
func (m *substitutionMatcher) isDerivationBlocked(sub, head *grammar.CompiledElement) bool {
	if sub.Type == nil || head.Type == nil {
		return false
	}

	// combine blocking from element and type
	combinedBlock := head.Block.Add(types.DerivationMethod(head.Type.Block))

	// walk the derivation chain of the substitute's type
	// check if any derivation step from sub's type to head's type uses a blocked method
	for _, typeInChain := range sub.Type.DerivationChain {
		if typeInChain == head.Type {
			break
		}

		if typeInChain.DerivationMethod == types.DerivationExtension && combinedBlock.Has(types.DerivationExtension) {
			return true
		}
		if typeInChain.DerivationMethod == types.DerivationRestriction && combinedBlock.Has(types.DerivationRestriction) {
			return true
		}
	}

	return false
}
