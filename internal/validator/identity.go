package validator

import (
	"strings"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// checkIdentityConstraints validates all identity constraints in the document.
// Uses precomputed ElementsWithConstraints from compilation for O(1) lookup.
func (r *validationRun) checkIdentityConstraints(root xml.NodeID) []errors.Validation {
	var violations []errors.Validation

	// use precomputed list of elements with constraints (computed during compilation)
	for _, decl := range r.schema.ElementsWithConstraints() {
		matchingElements := r.findAllMatchingElements(root, decl)
		for _, elem := range matchingElements {
			violations = append(violations, r.checkIdentityConstraintsOnElement(elem, decl)...)
		}
	}

	return violations
}

// checkIdentityConstraintsOnElement validates all identity constraints on a single element.
func (r *validationRun) checkIdentityConstraintsOnElement(elem xml.NodeID, decl *grammar.CompiledElement) []errors.Validation {
	var violations []errors.Validation

	// first pass: collect key and unique values
	localKeyTables := make(map[string]map[string]bool)

	for _, constraint := range decl.Constraints {
		switch constraint.Original.Type {
		case types.KeyConstraint:
			keyTable, keyViolations := r.checkKey(elem, constraint)
			violations = append(violations, keyViolations...)
			localKeyTables[constraint.Original.Name] = keyTable
		case types.UniqueConstraint:
			uniqueTable, uniqueViolations := r.checkUnique(elem, constraint)
			violations = append(violations, uniqueViolations...)
			localKeyTables[constraint.Original.Name] = uniqueTable
		}
	}

	// second pass: validate keyref constraints
	for _, constraint := range decl.Constraints {
		if constraint.Original.Type == types.KeyRefConstraint {
			violations = append(violations, r.checkKeyRef(elem, constraint, localKeyTables)...)
		}
	}

	return violations
}

// checkKey validates a key constraint and returns the key table and any violations.
func (r *validationRun) checkKey(elem xml.NodeID, constraint *grammar.CompiledConstraint) (map[string]bool, []errors.Validation) {
	var violations []errors.Validation
	ic := constraint.Original
	keyTable := make(map[string]bool)

	selectedElements := r.evaluateSelectorWithNS(elem, ic.Selector.XPath, ic.NamespaceContext)
	seenValues := make(map[string]xml.NodeID)

	for _, selectedElem := range selectedElements {
		keyResult := r.extractKeyValueWithNS(selectedElem, ic.Fields, ic.NamespaceContext)
		elemPath := r.elementPath(selectedElem)

		// key constraint requires all key values to be present (not absent)
		// note: Empty string "" is a valid value; KeyAbsent means no node selected
		if keyResult.State == KeyInvalid {
			violations = append(violations, errors.NewValidationf(errors.ErrIdentityAbsent, elemPath,
				"key '%s': field selects non-simple content for element at %s", ic.Name, elemPath))
			continue
		}
		if keyResult.State == KeyMultiple {
			violations = append(violations, errors.NewValidationf(errors.ErrIdentityAbsent, elemPath,
				"key '%s': field selects multiple nodes for element at %s", ic.Name, elemPath))
			continue
		}
		if keyResult.State == KeyAbsent {
			violations = append(violations, errors.NewValidationf(errors.ErrIdentityAbsent, elemPath,
				"key '%s': field value is absent for element at %s", ic.Name, elemPath))
			continue
		}

		if existingElem, exists := seenValues[keyResult.Value]; exists {
			violations = append(violations, errors.NewValidationf(errors.ErrIdentityDuplicate, elemPath,
				"key '%s': duplicate value '%s' at %s (first occurrence at %s)",
				ic.Name, r.getDisplayValueWithNS(selectedElem, ic.Fields, ic.NamespaceContext), elemPath, r.elementPath(existingElem)))
		} else {
			seenValues[keyResult.Value] = selectedElem
			keyTable[keyResult.Value] = true
		}
	}

	return keyTable, violations
}

// checkUnique validates a unique constraint.
func (r *validationRun) checkUnique(elem xml.NodeID, constraint *grammar.CompiledConstraint) (map[string]bool, []errors.Validation) {
	var violations []errors.Validation
	ic := constraint.Original
	uniqueTable := make(map[string]bool)

	selectedElements := r.evaluateSelectorWithNS(elem, ic.Selector.XPath, ic.NamespaceContext)
	seenValues := make(map[string]xml.NodeID)

	for _, selectedElem := range selectedElements {
		keyResult := r.extractKeyValueWithNS(selectedElem, ic.Fields, ic.NamespaceContext)

		if keyResult.State == KeyInvalid {
			elemPath := r.elementPath(selectedElem)
			violations = append(violations, errors.NewValidationf(errors.ErrIdentityDuplicate, elemPath,
				"unique '%s': field selects non-simple content for element at %s", ic.Name, elemPath))
			continue
		}
		if keyResult.State == KeyMultiple {
			elemPath := r.elementPath(selectedElem)
			violations = append(violations, errors.NewValidationf(errors.ErrIdentityDuplicate, elemPath,
				"unique '%s': field selects multiple nodes for element at %s", ic.Name, elemPath))
			continue
		}
		// unique constraint allows absent values (when no node is selected)
		if keyResult.State == KeyAbsent {
			continue
		}

		elemPath := r.elementPath(selectedElem)
		if existingElem, exists := seenValues[keyResult.Value]; exists {
			violations = append(violations, errors.NewValidationf(errors.ErrIdentityDuplicate, elemPath,
				"unique '%s': duplicate value '%s' at %s (first occurrence at %s)",
				ic.Name, r.getDisplayValueWithNS(selectedElem, ic.Fields, ic.NamespaceContext), elemPath, r.elementPath(existingElem)))
		} else {
			seenValues[keyResult.Value] = selectedElem
			uniqueTable[keyResult.Value] = true
		}
	}

	return uniqueTable, violations
}

// checkKeyRef validates a keyref constraint.
func (r *validationRun) checkKeyRef(elem xml.NodeID, constraint *grammar.CompiledConstraint, localKeyTables map[string]map[string]bool) []errors.Validation {
	var violations []errors.Validation
	ic := constraint.Original

	referName := ic.ReferQName.String()
	refLocal := ic.ReferQName.Local

	keyTable := localKeyTables[refLocal]
	if keyTable == nil {
		keyTable = localKeyTables[referName]
	}

	if keyTable == nil {
		elemPath := r.elementPath(elem)
		violations = append(violations, errors.NewValidationf(errors.ErrIdentityKeyRefFailed, elemPath,
			"keyref '%s' refers to undefined key '%s'", ic.Name, referName))
		return violations
	}

	selectedElements := r.evaluateSelectorWithNS(elem, ic.Selector.XPath, ic.NamespaceContext)

	for _, selectedElem := range selectedElements {
		keyResult := r.extractKeyValueWithNS(selectedElem, ic.Fields, ic.NamespaceContext)

		if keyResult.State == KeyInvalid {
			elemPath := r.elementPath(selectedElem)
			violations = append(violations, errors.NewValidationf(errors.ErrIdentityKeyRefFailed, elemPath,
				"keyref '%s': field selects non-simple content for element at %s", ic.Name, elemPath))
			continue
		}
		if keyResult.State == KeyMultiple {
			elemPath := r.elementPath(selectedElem)
			violations = append(violations, errors.NewValidationf(errors.ErrIdentityKeyRefFailed, elemPath,
				"keyref '%s': field selects multiple nodes for element at %s", ic.Name, elemPath))
			continue
		}
		// keyref with absent value is allowed (per XSD spec) - just skip
		if keyResult.State == KeyAbsent {
			continue
		}

		elemPath := r.elementPath(selectedElem)

		// check if value exists in referenced key table
		if !keyTable[keyResult.Value] {
			violations = append(violations, errors.NewValidationf(errors.ErrIdentityKeyRefFailed, elemPath,
				"keyref '%s': value '%s' not found in referenced key '%s' at %s",
				ic.Name, r.getDisplayValueWithNS(selectedElem, ic.Fields, ic.NamespaceContext), referName, elemPath))
		}
	}

	return violations
}

// findAllMatchingElements finds all elements that match the given declaration.
func (r *validationRun) findAllMatchingElements(root xml.NodeID, decl *grammar.CompiledElement) []xml.NodeID {
	return r.collectMatchingElements(root, decl, nil)
}

// collectMatchingElements recursively collects elements matching the declaration.
func (r *validationRun) collectMatchingElements(elem xml.NodeID, decl *grammar.CompiledElement, results []xml.NodeID) []xml.NodeID {
	elemQName := types.QName{
		Namespace: types.NamespaceURI(r.doc.NamespaceURI(elem)),
		Local:     r.doc.LocalName(elem),
	}

	// direct match
	if decl.QName == elemQName {
		results = append(results, elem)
	} else {
		for _, sub := range r.schema.SubstitutionGroup(decl.QName) {
			if sub.QName == elemQName {
				results = append(results, elem)
				break
			}
		}
	}

	// recursively check children
	for _, child := range r.doc.Children(elem) {
		results = r.collectMatchingElements(child, decl, results)
	}
	return results
}

// elementPath builds a path string for an element.
func (r *validationRun) elementPath(elem xml.NodeID) string {
	name := r.doc.LocalName(elem)
	if ns := r.doc.NamespaceURI(elem); ns != "" {
		return "/" + ns + ":" + name
	}
	return "/" + name
}

// getDisplayValueWithNS extracts the original value for display in error messages,
// using the provided namespace context for XPath prefix resolution.
func (r *validationRun) getDisplayValueWithNS(elem xml.NodeID, fields []types.Field, nsContext map[string]string) string {
	if len(fields) == 0 {
		return ""
	}

	values := make([]string, 0, len(fields))
	for _, field := range fields {
		rawValue := r.evaluateFieldWithNS(elem, field.XPath, nsContext)
		values = append(values, strings.TrimSpace(rawValue))
	}

	return strings.Join(values, ", ")
}
