package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

// parseDerivationSetWithValidation parses and validates a derivation set.
// Returns an error if any token is not a valid derivation method.
// Per XSD spec, #all cannot be combined with other values.
func parseDerivationSetWithValidation(value string, allowed types.DerivationSet) (types.DerivationSet, error) {
	var set types.DerivationSet
	hasAll := false
	for token := range types.FieldsXMLWhitespaceSeq(value) {
		if hasAll {
			return set, fmt.Errorf("derivation set cannot combine '#all' with other values")
		}
		switch token {
		case "extension":
			if !allowed.Has(types.DerivationExtension) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationExtension)
		case "restriction":
			if !allowed.Has(types.DerivationRestriction) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationRestriction)
		case "list":
			if !allowed.Has(types.DerivationList) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationList)
		case "union":
			if !allowed.Has(types.DerivationUnion) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationUnion)
		case "substitution":
			if !allowed.Has(types.DerivationSubstitution) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationSubstitution)
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
