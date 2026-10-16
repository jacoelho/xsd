package schema

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
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
	redefineChild    = "redefine"
	assertChild      = "assert"
	sequenceChild    = vocab.XSDElemSequence
	simpleContent    = vocab.XSDElemSimpleContent
	simpleTypeChild  = vocab.XSDElemSimpleType
	unionChild       = vocab.XSDElemUnion
	uniqueChild      = vocab.XSDElemUnique
)

const (
	nilErrorString                                = "<nil>"
	annotationMustBeFirstSuffix                   = " annotation must be first"
	attributeOutOfOrderSuffix                     = " attribute is out of order"
	modelGroupOutOfOrderSuffix                    = " model group is out of order"
	oneAnyAttributeSuffix                         = " can contain at most one anyAttribute"
	complexTypeModelGroupOutOfOrder               = "complexType" + modelGroupOutOfOrderSuffix
	simpleContentSimpleTypeOutOfOrder             = "simpleContent simpleType is out of order"
	simpleContentFacetOutOfOrder                  = "simpleContent facet is out of order"
	simpleContentExtensionCannotContainSimpleType = "simpleContent extension cannot contain simpleType"
)

// ChildRule classifies one child element kind inside a schema component.
type ChildRule struct {
	Match        func(local string) bool
	ForbiddenMsg string
	OrderMsg     string
	DupMsg       string
	Level        int
	MaxOne       bool
	Terminal     bool
}

// ChildOrder describes the permitted children of one schema component:
// optional leading annotations followed by ordered sections of rules.
type ChildOrder struct {
	InvalidMsg         func(local string) string
	AnnotationFirstMsg string
	Rules              []ChildRule
	SingleAnnotation   bool
}

// ChildOrderError identifies the child index that violated an order rule.
type ChildOrderError struct {
	Message string
	Index   int
}

func (e *ChildOrderError) Error() string {
	if e == nil {
		return nilErrorString
	}
	return e.Message
}

// ContentDerivationKind identifies the xs:extension/xs:restriction choice in a
// simpleContent or complexContent container.
type ContentDerivationKind uint8

const (
	// ContentDerivationNone represents a missing or invalid derivation child.
	ContentDerivationNone ContentDerivationKind = iota
	// ContentDerivationExtension represents an xs:extension derivation child.
	ContentDerivationExtension
	// ContentDerivationRestriction represents an xs:restriction derivation child.
	ContentDerivationRestriction
)

func (k ContentDerivationKind) String() string {
	switch k {
	case ContentDerivationExtension:
		return extensionChild
	case ContentDerivationRestriction:
		return restrictionChild
	case ContentDerivationNone:
		return ""
	default:
	}
	return ""
}

var simpleTypeChildOrder = ChildOrder{
	AnnotationFirstMsg: "simpleType annotation must be first",
	SingleAnnotation:   true,
	Rules: []ChildRule{
		{
			Match:  matchChildLocal(restrictionChild, listChild, unionChild),
			MaxOne: true,
			DupMsg: "simpleType can contain one restriction, list, or union",
		},
	},
	InvalidMsg: func(local string) string { return "unsupported simpleType child " + local },
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

var complexTypeChildOrder = ChildOrder{
	AnnotationFirstMsg: "complexType" + annotationMustBeFirstSuffix,
	Rules: []ChildRule{
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
	InvalidMsg: func(local string) string { return "invalid complexType child " + local },
}

var complexContentChildOrder = derivationContainerOrder(complexContent)
var simpleContentChildOrder = derivationContainerOrder(simpleContent)
var simpleContentRestrictionChildOrder = simpleContentDerivationOrder(restrictionChild)
var simpleContentExtensionChildOrder = simpleContentDerivationOrder(extensionChild)
var complexContentRestrictionChildOrder = complexContentDerivationOrder(restrictionChild)
var complexContentExtensionChildOrder = complexContentDerivationOrder(extensionChild)
var anyParticleChildOrder = annotationOnlyChildOrder(anyChild)
var anyAttributeChildOrder = annotationOnlyChildOrder(anyAttribute)
var elementRefChildOrder = annotationOnlyChildOrder("element ref")
var attributeRefChildOrder = annotationOnlyChildOrder("attribute ref")
var attributeGroupUseChildOrder = annotationOnlyChildOrder("attributeGroup use")
var groupUseChildOrder = annotationOnlyChildOrder("group use")
var attributeGroupDeclarationChildOrder = ChildOrder{
	AnnotationFirstMsg: "attributeGroup annotation must be first",
	Rules: []ChildRule{
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
	InvalidMsg: func(local string) string { return "invalid attribute use child " + local },
}
var attributeDeclarationChildOrder = ChildOrder{
	AnnotationFirstMsg: "attribute annotation must precede simpleType",
	Rules: []ChildRule{
		{
			Match:  matchChildLocal(simpleTypeChild),
			MaxOne: true,
			DupMsg: "attribute can contain at most one simpleType",
		},
	},
	InvalidMsg: func(local string) string { return "invalid attribute child " + local },
}
var elementDeclarationChildOrder = ChildOrder{
	AnnotationFirstMsg: "element annotation must be first",
	Rules: []ChildRule{
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
	InvalidMsg: func(local string) string { return "invalid element child " + local },
}

// ContentDerivationBase owns the lexical and presence state of a required base
// attribute together with the grammar context used in its diagnostic.
type ContentDerivationBase struct {
	Container  string
	Derivation string
	Lexical    string
	Present    bool
}

// ValidateContentDerivationBase validates that a content derivation has its
// required base attribute.
func ValidateContentDerivationBase(base ContentDerivationBase) error {
	if base.Present {
		return nil
	}
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, base.Container+" "+base.Derivation+" missing base")
}

var (
	modelGroupChildRules = []ChildRule{
		{
			Match: matchChildLocal(elementChild, sequenceChild, choiceChild, allChild, groupChild, anyChild),
		},
	}
	sequenceModelGroupChildOrder = ChildOrder{
		AnnotationFirstMsg: sequenceChild + annotationMustBeFirstSuffix,
		Rules:              modelGroupChildRules,
		InvalidMsg:         invalidModelGroupChild,
	}
	choiceModelGroupChildOrder = ChildOrder{
		AnnotationFirstMsg: choiceChild + annotationMustBeFirstSuffix,
		Rules:              modelGroupChildRules,
		InvalidMsg:         invalidModelGroupChild,
	}
	allModelGroupChildOrder = ChildOrder{
		AnnotationFirstMsg: allChild + annotationMustBeFirstSuffix,
		Rules:              modelGroupChildRules,
		InvalidMsg:         invalidModelGroupChild,
	}
)

func modelGroupChildOrder(parent string) ChildOrder {
	switch parent {
	case sequenceChild:
		return sequenceModelGroupChildOrder
	case choiceChild:
		return choiceModelGroupChildOrder
	case allChild:
		return allModelGroupChildOrder
	default:
		return ChildOrder{
			AnnotationFirstMsg: parent + annotationMustBeFirstSuffix,
			Rules:              modelGroupChildRules,
			InvalidMsg:         invalidModelGroupChild,
		}
	}
}

func invalidModelGroupChild(local string) string { return "invalid model group child " + local }

type childOrderState struct {
	seen              uint64
	annotationSeen    bool
	nonAnnotationSeen bool
	terminalSeen      bool
	maxLevelSeen      int
}

func (s *childOrderState) accept(index int, child string, order ChildOrder) error {
	if s.terminalSeen {
		return childOrderError(index, order.InvalidMsg(child))
	}
	if child == annotationChild {
		return s.acceptAnnotation(index, order)
	}
	return s.acceptRule(index, child, order)
}

func (s *childOrderState) acceptAnnotation(index int, order ChildOrder) error {
	if s.nonAnnotationSeen || order.SingleAnnotation && s.annotationSeen {
		return childOrderError(index, order.AnnotationFirstMsg)
	}
	s.annotationSeen = true
	return nil
}

func (s *childOrderState) acceptRule(index int, child string, order ChildOrder) error {
	ruleIndex := childRuleIndex(child, order.Rules)
	if ruleIndex < 0 {
		return childOrderError(index, order.InvalidMsg(child))
	}
	rule := order.Rules[ruleIndex]
	if rule.ForbiddenMsg != "" {
		return childOrderError(index, rule.ForbiddenMsg)
	}
	s.nonAnnotationSeen = true
	if s.maxLevelSeen > rule.Level {
		return childOrderError(index, rule.OrderMsg)
	}
	bit, err := childRuleSeenBit(ruleIndex)
	if err != nil {
		return err
	}
	if s.seen&bit != 0 && rule.MaxOne {
		return childOrderError(index, rule.DupMsg)
	}
	s.seen |= bit
	s.maxLevelSeen = max(s.maxLevelSeen, rule.Level)
	s.terminalSeen = rule.Terminal
	return nil
}

func childRuleIndex(child string, rules []ChildRule) int {
	for i, rule := range rules {
		if rule.Match(child) {
			return i
		}
	}
	return -1
}

func childRuleSeenBit(idx int) (uint64, error) {
	if idx < 0 || idx >= 64 {
		return 0, xsderrors.InternalInvariant(fmt.Sprintf("child order rule index %d exceeds bitset capacity", idx))
	}
	return uint64(1) << idx, nil
}

func childOrderError(index int, msg string) error {
	return &ChildOrderError{Index: index, Message: msg}
}

func simpleDerivationOrder(kind simpleDerivationKind) ChildOrder {
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
	rules := []ChildRule{
		{
			Match:    matchChildLocal(simpleTypeChild),
			Level:    0,
			MaxOne:   singleChild,
			OrderMsg: derivation + " simpleType must precede facets",
			DupMsg:   derivation + " can contain one simpleType",
		},
	}
	if allowFacets {
		rules = append(rules, ChildRule{
			Match: isFacetChild,
			Level: 1,
		})
	}
	return ChildOrder{
		AnnotationFirstMsg: derivation + annotationMustBeFirstSuffix,
		SingleAnnotation:   true,
		Rules:              rules,
		InvalidMsg:         func(local string) string { return "invalid " + derivation + " child " + local },
	}
}

func derivationContainerOrder(label string) ChildOrder {
	return ChildOrder{
		AnnotationFirstMsg: label + annotationMustBeFirstSuffix,
		Rules: []ChildRule{
			{
				Match:  matchChildLocal(extensionChild, restrictionChild),
				MaxOne: true,
				DupMsg: label + " can contain one derivation",
			},
		},
		InvalidMsg: func(local string) string { return "invalid " + label + " child " + local },
	}
}

func simpleContentDerivationOrder(derivation string) ChildOrder {
	rules := []ChildRule{
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
		rules = append(rules, ChildRule{
			Match:    isFacetChild,
			Level:    1,
			OrderMsg: simpleContentFacetOutOfOrder,
		})
	} else {
		rules[0].ForbiddenMsg = simpleContentExtensionCannotContainSimpleType
	}
	return ChildOrder{
		AnnotationFirstMsg: derivation + annotationMustBeFirstSuffix,
		Rules:              rules,
		InvalidMsg: func(local string) string {
			return "invalid simpleContent " + derivation + " child " + local
		},
	}
}

func complexContentDerivationOrder(derivation string) ChildOrder {
	return ChildOrder{
		AnnotationFirstMsg: derivation + annotationMustBeFirstSuffix,
		Rules: []ChildRule{
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
		InvalidMsg: func(local string) string { return "invalid complexContent child " + local },
	}
}

func annotationOnlyChildOrder(label string) ChildOrder {
	return ChildOrder{
		AnnotationFirstMsg: label + " can contain at most one annotation",
		SingleAnnotation:   true,
		InvalidMsg:         func(string) string { return label + " can contain only annotation" },
	}
}

func isFacetChild(local string) bool {
	switch local {
	case vocab.XSDFacetLength, vocab.XSDFacetMinLength, vocab.XSDFacetMaxLength, vocab.XSDFacetTotalDigits, vocab.XSDFacetFractionDigits,
		vocab.XSDFacetMinInclusive, vocab.XSDFacetMaxInclusive, vocab.XSDFacetMinExclusive, vocab.XSDFacetMaxExclusive,
		vocab.XSDFacetEnumeration, vocab.XSDFacetPattern, vocab.XSDFacetWhiteSpace:
		return true
	default:
		return false
	}
}

func matchChildLocal(locals ...string) func(string) bool {
	return func(local string) bool {
		return slices.Contains(locals, local)
	}
}
