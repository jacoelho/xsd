package schema

import (
	"encoding/xml"
	"slices"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/vocab"
)

const (
	annotationChild  = vocab.XSDElemAnnotation
	allChild         = vocab.XSDElemAll
	anyChild         = vocab.XSDElemAny
	anyAttribute     = vocab.XSDElemAnyAttribute
	attributeChild   = vocab.XSDElemAttribute
	attributeGroup   = vocab.XSDElemAttributeGroup
	choiceChild      = vocab.XSDElemChoice
	complexContent   = vocab.XSDElemComplexContent
	complexTypeChild = vocab.XSDElemComplexType
	elementChild     = vocab.XSDElemElement
	extensionChild   = vocab.XSDElemExtension
	groupChild       = vocab.XSDElemGroup
	importChild      = vocab.XSDElemImport
	includeChild     = vocab.XSDElemInclude
	keyChild         = vocab.XSDElemKey
	keyrefChild      = vocab.XSDElemKeyref
	restrictionChild = vocab.XSDElemRestriction
	listChild        = vocab.XSDElemList
	notationChild    = vocab.XSDElemNotation
	sequenceChild    = vocab.XSDElemSequence
	simpleContent    = vocab.XSDElemSimpleContent
	simpleTypeChild  = vocab.XSDElemSimpleType
	unionChild       = vocab.XSDElemUnion
	uniqueChild      = vocab.XSDElemUnique
)

const (
	attributeOutOfOrderSuffix                     = " attribute is out of order"
	modelGroupOutOfOrderSuffix                    = " model group is out of order"
	oneAnyAttributeSuffix                         = " can contain at most one anyAttribute"
	complexTypeModelGroupOutOfOrder               = "complexType" + modelGroupOutOfOrderSuffix
	simpleContentSimpleTypeOutOfOrder             = "simpleContent simpleType is out of order"
	simpleContentFacetOutOfOrder                  = "simpleContent facet is out of order"
	simpleContentExtensionCannotContainSimpleType = "simpleContent extension cannot contain simpleType"
	topLevelGroupModelGroupCount                  = "top-level group must contain exactly one model group"
	identityConstraintOneSelector                 = "identity constraint can contain at most one selector"
)

// childRule classifies one child element kind inside a schema component.
// Children of one rule may repeat unless MaxOne is set; a child whose rule
// has a lower Level than an earlier child's is out of order. PrecedingMsg
// reports a child that no child of a lower level precedes.
type childRule struct {
	Match        func(local string) bool
	ForbiddenMsg string
	OrderMsg     string
	DupMsg       string
	PrecedingMsg string
	Level        int
	MaxOne       bool
	Terminal     bool
}

// childOrder is the XSD 1.0 content model of one schema element in one
// placement, after its leading annotation, whose placement
// childContentViolation owns: ordered sections of rules. childOrderFor selects
// it when the element starts, and childOrderState admits each child as it
// starts, so a misplaced, repeated or unknown child is the violation at its
// own start tag. Which children are required is checked when the component
// compiles.
type childOrder struct {
	// Label names the parent in the diagnostic of a child no rule matches.
	Label string
	Rules []childRule
}

// contentDerivationKind identifies the xs:extension/xs:restriction choice in a
// simpleContent or complexContent container.
type contentDerivationKind uint8

const (
	// contentDerivationNone represents a missing or invalid derivation child.
	contentDerivationNone contentDerivationKind = iota
	// contentDerivationExtension represents an xs:extension derivation child.
	contentDerivationExtension
	// contentDerivationRestriction represents an xs:restriction derivation child.
	contentDerivationRestriction
)

var simpleTypeChildOrder = childOrder{
	Rules: []childRule{
		{
			Match:  matchChildLocal(restrictionChild, listChild, unionChild),
			MaxOne: true,
			DupMsg: "simpleType can contain one restriction, list, or union",
		},
	},
	Label: "simpleType",
}

type simpleDerivationKind uint8

const (
	simpleDerivationInvalid simpleDerivationKind = iota
	simpleDerivationRestriction
	simpleDerivationList
	simpleDerivationUnion
)

// Union derivations may hold several member simpleType children; restriction
// and list derivations hold at most one. Only restriction admits facet
// children: list content is (annotation?, simpleType?) and union content is
// (annotation?, simpleType*).
var simpleRestrictionChildOrder = simpleDerivationOrder(simpleDerivationRestriction)
var simpleListChildOrder = simpleDerivationOrder(simpleDerivationList)
var simpleUnionChildOrder = simpleDerivationOrder(simpleDerivationUnion)

var complexTypeChildOrder = childOrder{
	Rules: []childRule{
		{
			Match:    matchChildLocal(simpleContent, complexContent),
			Level:    0,
			Terminal: true,
			OrderMsg: "complexType content model is out of order",
		},
		{
			Match:    matchChildLocal(sequenceChild, choiceChild, allChild, groupChild),
			Level:    1,
			MaxOne:   true,
			OrderMsg: complexTypeModelGroupOutOfOrder,
			DupMsg:   complexTypeModelGroupOutOfOrder,
		},
		{
			Match:    matchChildLocal(attributeChild, attributeGroup),
			Level:    2,
			OrderMsg: "complexType" + attributeOutOfOrderSuffix,
		},
		{
			Match:  matchChildLocal(anyAttribute),
			Level:  3,
			MaxOne: true,
			DupMsg: "complexType" + oneAnyAttributeSuffix,
		},
	},
	Label: "complexType",
}

var complexContentChildOrder = derivationContainerOrder(complexContent)
var simpleContentChildOrder = derivationContainerOrder(simpleContent)
var simpleContentRestrictionChildOrder = simpleContentDerivationOrder(restrictionChild)
var simpleContentExtensionChildOrder = simpleContentDerivationOrder(extensionChild)
var complexContentRestrictionChildOrder = complexContentDerivationOrder(restrictionChild)
var complexContentExtensionChildOrder = complexContentDerivationOrder(extensionChild)
var attributeGroupDeclarationChildOrder = childOrder{
	Rules: []childRule{
		{
			Match:    matchChildLocal(attributeChild, attributeGroup),
			Level:    0,
			OrderMsg: "attributeGroup attribute is out of order",
		},
		{
			Match:  matchChildLocal(anyAttribute),
			Level:  1,
			MaxOne: true,
			DupMsg: "attributeGroup can contain at most one anyAttribute",
		},
	},
	Label: attributeUseLabel,
}
var attributeDeclarationChildOrder = childOrder{
	Rules: []childRule{
		{
			Match:  matchChildLocal(simpleTypeChild),
			MaxOne: true,
			DupMsg: "attribute can contain at most one simpleType",
		},
	},
	Label: "attribute",
}
var elementDeclarationChildOrder = childOrder{
	Rules: []childRule{
		{
			Match:    matchChildLocal(simpleTypeChild, complexTypeChild),
			Level:    0,
			MaxOne:   true,
			OrderMsg: "element anonymous type must precede identity constraints",
			DupMsg:   "element can contain at most one anonymous type",
		},
		{
			Match: matchChildLocal(uniqueChild, keyChild, keyrefChild),
			Level: 1,
		},
	},
	Label: "element",
}

// nestedModelGroupChildOrder is the content of xs:sequence and xs:choice,
// whose particles are nestedParticle in the schema for schemas: no xs:all.
// allModelGroupChildOrder is the content of xs:all, whose particles are
// element declarations only. A model group reference that resolves to xs:all
// is checked when the content model compiles.
var (
	nestedModelGroupChildOrder = childOrder{
		Rules: []childRule{
			{Match: matchChildLocal(elementChild, sequenceChild, choiceChild, groupChild, anyChild)},
			{Match: matchChildLocal(allChild), ForbiddenMsg: "xs:all cannot be nested in model groups"},
		},
		Label: schemaComponentModelGroupLabel,
	}
	allModelGroupChildOrder = childOrder{
		Rules: []childRule{
			{Match: matchChildLocal(elementChild)},
			{Match: matchChildLocal(sequenceChild, choiceChild, allChild, groupChild, anyChild), ForbiddenMsg: "xs:all can contain only element particles"},
		},
		Label: schemaComponentModelGroupLabel,
	}
)

var topLevelGroupChildOrder = childOrder{
	Rules: []childRule{
		{
			Match:  matchChildLocal(sequenceChild, choiceChild, allChild),
			MaxOne: true,
			DupMsg: topLevelGroupModelGroupCount,
		},
		{
			Match:        matchChildLocal(groupChild),
			ForbiddenMsg: "top-level group cannot contain group ref",
		},
	},
	Label: "top-level group",
}

var identityConstraintChildOrder = childOrder{
	Rules: []childRule{
		{
			Match:    matchChildLocal(selectorChild),
			Level:    0,
			MaxOne:   true,
			OrderMsg: identityConstraintOneSelector,
			DupMsg:   identityConstraintOneSelector,
		},
		{
			Match:        matchChildLocal(fieldChild),
			Level:        1,
			PrecedingMsg: "identity constraint field requires selector",
		},
	},
	Label: "identity constraint",
}

// schemaChildOrder is the content of xs:schema, whose xs:annotation children
// may appear anywhere.
var schemaChildOrder = childOrder{
	Rules: []childRule{
		{
			Match:    matchChildLocal(includeChild),
			Level:    0,
			OrderMsg: `xs:"include" must precede global declarations`,
		},
		{
			Match:    matchChildLocal(importChild),
			Level:    0,
			OrderMsg: `xs:"import" must precede global declarations`,
		},
		{
			Match: matchChildLocal(simpleTypeChild, complexTypeChild, groupChild, attributeGroup, elementChild, attributeChild, notationChild),
			Level: 1,
		},
		{
			Match:        matchChildLocal(uniqueChild, keyChild, keyrefChild, selectorChild, fieldChild),
			ForbiddenMsg: "identity constraint must be inside element",
		},
	},
	Label: "top-level schema",
}

// childOrderFor returns the content model of the XSD element named local
// whose parent is named parent, or nil when the element admits only
// xs:annotation or is never admitted as a parent. An element whose parent's
// content model does not admit it never starts, so parent here is one that
// admits local.
func childOrderFor(local string, parent xml.Name) *childOrder {
	switch local {
	case vocab.XSDElemSchema:
		return &schemaChildOrder
	case simpleTypeChild:
		return &simpleTypeChildOrder
	case restrictionChild:
		switch parent.Local {
		case simpleContent:
			return &simpleContentRestrictionChildOrder
		case complexContent:
			return &complexContentRestrictionChildOrder
		}
		return &simpleRestrictionChildOrder
	case extensionChild:
		if parent.Local == simpleContent {
			return &simpleContentExtensionChildOrder
		}
		return &complexContentExtensionChildOrder
	case listChild:
		return &simpleListChildOrder
	case unionChild:
		return &simpleUnionChildOrder
	case complexTypeChild:
		return &complexTypeChildOrder
	case simpleContent:
		return &simpleContentChildOrder
	case complexContent:
		return &complexContentChildOrder
	case sequenceChild, choiceChild:
		return &nestedModelGroupChildOrder
	case allChild:
		return &allModelGroupChildOrder
	case groupChild:
		return &topLevelGroupChildOrder
	case attributeGroup:
		return &attributeGroupDeclarationChildOrder
	case attributeChild:
		return &attributeDeclarationChildOrder
	case elementChild:
		return &elementDeclarationChildOrder
	case uniqueChild, keyChild, keyrefChild:
		return &identityConstraintChildOrder
	}
	return nil
}

// childOrderState is the progress of one parent through its childOrder. Its
// zero value has seen no child.
type childOrderState struct {
	seen         uint64
	levelSeen    int
	terminalSeen bool
}

// accept admits child, the local name of the next XSD-namespace child, and
// returns why order forbids it there, or "" when it is admitted. The
// diagnostic is built only on rejection. Annotation placement is
// childContentViolation's.
func (s *childOrderState) accept(child string, order *childOrder) string {
	if s.terminalSeen {
		return "invalid " + order.Label + " child " + diagnostictext.Quote(child)
	}
	if child == annotationChild {
		return ""
	}
	ruleIndex := childRuleIndex(child, order.Rules)
	if ruleIndex < 0 {
		return "invalid " + order.Label + " child " + diagnostictext.Quote(child)
	}
	rule := &order.Rules[ruleIndex]
	switch {
	case rule.ForbiddenMsg != "":
		return rule.ForbiddenMsg
	case s.levelSeen > rule.Level:
		return rule.OrderMsg
	case rule.PrecedingMsg != "" && !s.seenBelow(rule.Level, order.Rules):
		return rule.PrecedingMsg
	}
	bit := uint64(1) << ruleIndex
	if s.seen&bit != 0 && rule.MaxOne {
		return rule.DupMsg
	}
	s.seen |= bit
	s.levelSeen = max(s.levelSeen, rule.Level)
	s.terminalSeen = rule.Terminal
	return ""
}

func (s *childOrderState) seenBelow(level int, rules []childRule) bool {
	for i := range rules {
		if rules[i].Level < level && s.seen&(uint64(1)<<i) != 0 {
			return true
		}
	}
	return false
}

func childRuleIndex(child string, rules []childRule) int {
	for i, rule := range rules {
		if rule.Match(child) {
			return i
		}
	}
	return -1
}

func simpleDerivationOrder(kind simpleDerivationKind) childOrder {
	var derivation string
	var singleChild bool
	var allowFacets bool
	switch kind {
	case simpleDerivationRestriction:
		derivation = restrictionChild
		singleChild = true
		allowFacets = true
	case simpleDerivationList:
		derivation = listChild
		singleChild = true
	case simpleDerivationUnion:
		derivation = unionChild
	case simpleDerivationInvalid:
		panic("invalid simple derivation kind")
	default:
		panic("unknown simple derivation kind")
	}
	rules := []childRule{
		{
			Match:    matchChildLocal(simpleTypeChild),
			Level:    0,
			MaxOne:   singleChild,
			OrderMsg: derivation + " simpleType must precede facets",
			DupMsg:   derivation + " can contain one simpleType",
		},
	}
	if allowFacets {
		rules = append(rules, childRule{
			Match: isFacetLocal,
			Level: 1,
		})
	}
	return childOrder{
		Rules: rules,
		Label: derivation,
	}
}

func derivationContainerOrder(label string) childOrder {
	return childOrder{
		Rules: []childRule{
			{
				Match:  matchChildLocal(extensionChild, restrictionChild),
				MaxOne: true,
				DupMsg: label + " can contain one derivation",
			},
		},
		Label: label,
	}
}

func simpleContentDerivationOrder(derivation string) childOrder {
	rules := []childRule{
		{
			Match:    matchChildLocal(simpleTypeChild),
			Level:    0,
			MaxOne:   true,
			OrderMsg: simpleContentSimpleTypeOutOfOrder,
			DupMsg:   simpleContentSimpleTypeOutOfOrder,
		},
		{
			Match:    matchChildLocal(attributeChild, attributeGroup),
			Level:    2,
			OrderMsg: "simpleContent" + attributeOutOfOrderSuffix,
		},
		{
			Match:  matchChildLocal(anyAttribute),
			Level:  3,
			MaxOne: true,
			DupMsg: "simpleContent" + oneAnyAttributeSuffix,
		},
	}
	if derivation == restrictionChild {
		rules = append(rules, childRule{
			Match:    isFacetLocal,
			Level:    1,
			OrderMsg: simpleContentFacetOutOfOrder,
		})
	} else {
		rules[0].ForbiddenMsg = simpleContentExtensionCannotContainSimpleType
	}
	return childOrder{
		Rules: rules,
		Label: "simpleContent " + derivation,
	}
}

func complexContentDerivationOrder(derivation string) childOrder {
	return childOrder{
		Rules: []childRule{
			{
				Match:    matchChildLocal(sequenceChild, choiceChild, allChild, groupChild),
				Level:    0,
				MaxOne:   true,
				OrderMsg: derivation + modelGroupOutOfOrderSuffix,
				DupMsg:   derivation + modelGroupOutOfOrderSuffix,
			},
			{
				Match:    matchChildLocal(attributeChild, attributeGroup),
				Level:    1,
				OrderMsg: derivation + attributeOutOfOrderSuffix,
			},
			{
				Match:  matchChildLocal(anyAttribute),
				Level:  2,
				MaxOne: true,
				DupMsg: derivation + oneAnyAttributeSuffix,
			},
		},
		Label: complexContent,
	}
}

func matchChildLocal(locals ...string) func(string) bool {
	return func(local string) bool {
		return slices.Contains(locals, local)
	}
}
