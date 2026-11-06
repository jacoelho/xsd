package schema

import (
	"github.com/jacoelho/xsd/internal/vocab"
)

// derivationAttrRule identifies one XSD derivation-set attribute and the mask
// allowed for that attribute.
type derivationAttrRule struct {
	Name    string
	Label   string
	Allowed DerivationMask
}

const (
	complexTypeBlockLabel = "complexType block"
	complexTypeFinalLabel = "complexType final"
	simpleTypeFinalLabel  = "simpleType final"
	elementBlockLabel     = "element block"
	elementFinalLabel     = "element final"
)

func complexTypeBlockDerivation() derivationAttrRule {
	return derivationAttrRule{Name: vocab.XSDAttrBlock, Label: complexTypeBlockLabel, Allowed: derivationComplexMask}
}

func complexTypeFinalDerivation() derivationAttrRule {
	return derivationAttrRule{Name: vocab.XSDAttrFinal, Label: complexTypeFinalLabel, Allowed: derivationComplexMask}
}

func simpleTypeFinalDerivation() derivationAttrRule {
	return derivationAttrRule{Name: vocab.XSDAttrFinal, Label: simpleTypeFinalLabel, Allowed: derivationSimpleFinalMask}
}

func elementBlockDerivation() derivationAttrRule {
	return derivationAttrRule{Name: vocab.XSDAttrBlock, Label: elementBlockLabel, Allowed: derivationBlockDefaultMask}
}

func elementFinalDerivation() derivationAttrRule {
	return derivationAttrRule{Name: vocab.XSDAttrFinal, Label: elementFinalLabel, Allowed: derivationComplexMask}
}

// parseDerivationAttrWithDefault parses a derivation-set attribute or applies
// the schema default restricted to the rule's allowed derivation class.
func parseDerivationAttrWithDefault(attr lexicalAttribute, def DerivationMask, rule derivationAttrRule) (DerivationMask, error) {
	if attr.Present {
		return parseDerivationSet(attr.Value, rule.Label, rule.Allowed)
	}
	return def & rule.Allowed, nil
}
