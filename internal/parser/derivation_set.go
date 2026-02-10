package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

// parseDerivationSetWithValidation parses and validates a derivation set.
// Returns an error if any token is not a valid derivation method.
// Per XSD spec, #all cannot be combined with other values.
func parseDerivationSetWithValidation(value string, allowed model.DerivationSet) (model.DerivationSet, error) {
	var set model.DerivationSet
	hasAll := false
	for token := range model.FieldsXMLWhitespaceSeq(value) {
		if hasAll {
			return set, fmt.Errorf("derivation set cannot combine '#all' with other values")
		}
		switch token {
		case "extension":
			if !allowed.Has(model.DerivationExtension) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(model.DerivationExtension)
		case "restriction":
			if !allowed.Has(model.DerivationRestriction) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(model.DerivationRestriction)
		case "list":
			if !allowed.Has(model.DerivationList) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(model.DerivationList)
		case "union":
			if !allowed.Has(model.DerivationUnion) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(model.DerivationUnion)
		case "substitution":
			if !allowed.Has(model.DerivationSubstitution) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(model.DerivationSubstitution)
		case "#all":
			if set != 0 {
				return set, fmt.Errorf("derivation set cannot combine '#all' with other values")
			}
			set = allowed
			hasAll = true
		default:
			return set, fmt.Errorf("invalid derivation method '%s'", token)
		}
	}
	return set, nil
}
