package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
)

// DerivationAttrRule identifies one XSD derivation-set attribute and the mask
// allowed for that attribute.
type DerivationAttrRule struct {
	Name    string
	Label   string
	Allowed runtime.DerivationMask
}

const (
	complexTypeBlockLabel = "complexType block"
	complexTypeFinalLabel = "complexType final"
	simpleTypeFinalLabel  = "simpleType final"
	elementBlockLabel     = "element block"
	elementFinalLabel     = "element final"
)

func complexTypeBlockDerivation() DerivationAttrRule {
	return DerivationAttrRule{Name: vocab.XSDAttrBlock, Label: complexTypeBlockLabel, Allowed: runtime.DerivationComplexMask}
}

func complexTypeFinalDerivation() DerivationAttrRule {
	return DerivationAttrRule{Name: vocab.XSDAttrFinal, Label: complexTypeFinalLabel, Allowed: runtime.DerivationComplexMask}
}

func simpleTypeFinalDerivation() DerivationAttrRule {
	return DerivationAttrRule{Name: vocab.XSDAttrFinal, Label: simpleTypeFinalLabel, Allowed: runtime.DerivationSimpleFinalMask}
}

func elementBlockDerivation() DerivationAttrRule {
	return DerivationAttrRule{Name: vocab.XSDAttrBlock, Label: elementBlockLabel, Allowed: runtime.DerivationBlockDefaultMask}
}

func elementFinalDerivation() DerivationAttrRule {
	return DerivationAttrRule{Name: vocab.XSDAttrFinal, Label: elementFinalLabel, Allowed: runtime.DerivationComplexMask}
}

// ParseDerivationAttrWithDefault parses a derivation-set attribute or applies
// the schema default restricted to the rule's allowed derivation class.
func ParseDerivationAttrWithDefault(value string, hasValue bool, def runtime.DerivationMask, rule DerivationAttrRule) (runtime.DerivationMask, error) {
	if hasValue {
		return ParseDerivationSet(value, rule.Label, rule.Allowed)
	}
	return def & rule.Allowed, nil
}
