package validator

import (
	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/grammar/contentmodel"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// checkContentModel validates element children against the content model.
// Returns match results for each child and any violations.
func (r *validationRun) checkContentModel(elem xml.NodeID, cm *grammar.CompiledContentModel) ([]contentmodel.MatchResult, []errors.Validation) {
	if cm.RejectAll {
		children := getElementChildren(r.doc, elem)
		if len(children) == 0 {
			if cm.MinOccurs == 0 {
				return nil, nil
			}
			return nil, []errors.Validation{errors.NewValidation(errors.ErrRequiredElementMissing,
				"content does not satisfy empty choice", r.path.String())}
		}
		return nil, []errors.Validation{errors.NewValidation(errors.ErrUnexpectedElement,
			"element not allowed by empty choice", r.path.String())}
	}

	// for all groups, use the simple array-based validator
	if cm.AllElements != nil {
		return r.checkAllGroupContent(elem, cm)
	}

	if cm.Automaton != nil {
		return r.checkAutomatonContent(elem, cm)
	}

	// fallback: content model not compiled
	return nil, []errors.Validation{errors.NewValidation(errors.ErrContentModelInvalid,
		"Content model not compiled (automaton missing)", r.path.String())}
}

// checkAutomatonContent validates content using the DFA automaton.
func (r *validationRun) checkAutomatonContent(elem xml.NodeID, cm *grammar.CompiledContentModel) ([]contentmodel.MatchResult, []errors.Validation) {
	children := getElementChildren(r.doc, elem)
	matcher := r.matcher()
	wildcards := cm.Wildcards()

	matches, err := cm.Automaton.ValidateWithMatches(r.doc, children, matcher, wildcards)
	if err != nil {
		if ve, ok := err.(*contentmodel.ValidationError); ok {
			return matches, []errors.Validation{errors.NewValidation(errors.ErrorCode(ve.FullCode()), ve.Message, r.path.String())}
		}
	}

	return matches, nil
}

// checkAllGroupContent validates content using the all group validator.
func (r *validationRun) checkAllGroupContent(elem xml.NodeID, cm *grammar.CompiledContentModel) ([]contentmodel.MatchResult, []errors.Validation) {
	children := getElementChildren(r.doc, elem)
	if len(children) == 0 && cm.MinOccurs == 0 {
		return nil, nil
	}

	allElements := make([]contentmodel.AllGroupElementInfo, len(cm.AllElements))
	for i := range cm.AllElements {
		allElements[i] = cm.AllElements[i]
	}

	matcher := r.matcher()
	allValidator := contentmodel.NewAllGroupValidator(allElements, cm.Mixed)
	err := allValidator.Validate(r.doc, children, matcher)

	var violations []errors.Validation
	if err != nil {
		if ve, ok := err.(*contentmodel.ValidationError); ok {
			violations = append(violations, errors.NewValidation(errors.ErrorCode(ve.FullCode()), ve.Message, r.path.String()))
		}
	}

	var matches []contentmodel.MatchResult
	if err == nil {
		elementMap := make(map[types.QName]types.QName)
		for _, entry := range cm.AllElements {
			if entry != nil && entry.Element != nil {
				effectiveQName := r.effectiveElementQName(entry.Element)
				elementMap[effectiveQName] = effectiveQName
			}
		}
		for range children {
			matches = append(matches, contentmodel.MatchResult{
				IsWildcard:      false,
				ProcessContents: types.Strict,
			})
		}
		for i, child := range children {
			childQName := r.resolveElementQName(child)
			if matchedQName := elementMap[childQName]; !matchedQName.IsZero() {
				matches[i].MatchedQName = matchedQName
			}
		}
	}

	return matches, violations
}
