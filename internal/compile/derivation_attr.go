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

var (
	// ComplexTypeBlockDerivation parses xs:complexType block.
	ComplexTypeBlockDerivation = DerivationAttrRule{Name: vocab.XSDAttrBlock, Label: complexTypeBlockLabel, Allowed: runtime.DerivationComplexMask}
	// ComplexTypeFinalDerivation parses xs:complexType final.
	ComplexTypeFinalDerivation = DerivationAttrRule{Name: vocab.XSDAttrFinal, Label: complexTypeFinalLabel, Allowed: runtime.DerivationComplexMask}
	// SimpleTypeFinalDerivation parses xs:simpleType final.
	SimpleTypeFinalDerivation = DerivationAttrRule{Name: vocab.XSDAttrFinal, Label: simpleTypeFinalLabel, Allowed: runtime.DerivationSimpleFinalMask}
	// ElementBlockDerivation parses xs:element block.
	ElementBlockDerivation = DerivationAttrRule{Name: vocab.XSDAttrBlock, Label: elementBlockLabel, Allowed: runtime.DerivationBlockDefaultMask}
	// ElementFinalDerivation parses xs:element final.
	ElementFinalDerivation = DerivationAttrRule{Name: vocab.XSDAttrFinal, Label: elementFinalLabel, Allowed: runtime.DerivationComplexMask}
)

// ParseDerivationAttrWithDefault parses a derivation-set attribute or applies
// the schema default restricted to the rule's allowed derivation class.
func ParseDerivationAttrWithDefault(value string, hasValue bool, def runtime.DerivationMask, rule DerivationAttrRule) (runtime.DerivationMask, error) {
	if hasValue {
		return ParseDerivationSet(value, rule.Label, rule.Allowed)
	}
	return def & rule.Allowed, nil
}
