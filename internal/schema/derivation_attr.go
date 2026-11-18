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
	schemaBlockDefaultLabel = "schema blockDefault"
	schemaFinalDefaultLabel = "schema finalDefault"

	complexTypeBlockLabel = "complexType block"
	complexTypeFinalLabel = "complexType final"
	simpleTypeFinalLabel  = "simpleType final"
	elementBlockLabel     = "element block"
	elementFinalLabel     = "element final"
)

func schemaBlockDefaultDerivation() derivationAttrRule {
	return derivationAttrRule{Name: vocab.XSDAttrBlockDefault, Label: schemaBlockDefaultLabel, Allowed: derivationBlockDefaultMask}
}

func schemaFinalDefaultDerivation() derivationAttrRule {
	return derivationAttrRule{Name: vocab.XSDAttrFinalDefault, Label: schemaFinalDefaultLabel, Allowed: derivationFinalDefaultMask}
}

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

// derivationAttrRuleOf returns the rule of attr, a derivation-set attribute of
// the XSD element named element, and reports whether attr is one.
func derivationAttrRuleOf(element, attr string) (derivationAttrRule, bool) {
	switch element {
	case vocab.XSDElemSchema:
		switch attr {
		case vocab.XSDAttrBlockDefault:
			return schemaBlockDefaultDerivation(), true
		case vocab.XSDAttrFinalDefault:
			return schemaFinalDefaultDerivation(), true
		}
	case vocab.XSDElemElement:
		switch attr {
		case vocab.XSDAttrBlock:
			return elementBlockDerivation(), true
		case vocab.XSDAttrFinal:
			return elementFinalDerivation(), true
		}
	case vocab.XSDElemComplexType:
		switch attr {
		case vocab.XSDAttrBlock:
			return complexTypeBlockDerivation(), true
		case vocab.XSDAttrFinal:
			return complexTypeFinalDerivation(), true
		}
	case vocab.XSDElemSimpleType:
		if attr == vocab.XSDAttrFinal {
			return simpleTypeFinalDerivation(), true
		}
	}
	return derivationAttrRule{}, false
}

// parseDerivationAttrWithDefault parses an admitted derivation-set attribute
// or applies the schema default restricted to the rule's allowed derivation
// class.
func parseDerivationAttrWithDefault(attr lexicalAttribute, def DerivationMask, rule derivationAttrRule) (DerivationMask, error) {
	if !attr.Present {
		return def & rule.Allowed, nil
	}
	mask, err := parseDerivationSet(attr.Value, rule.Label, rule.Allowed)
	return mask, admittedValueInvariant(err)
}
