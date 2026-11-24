package schema

import (
	"encoding/xml"
	"slices"
	"strconv"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	valuepkg "github.com/jacoelho/xsd/internal/value"
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
// reports a child that no child of a lower level precedes. A rule with a
// MissingMsg requires a child: a parent that ends without one is reported
// with MissingCode, unless Supplied reports that the parent's attributes
// supply what the child would. A rule with a ConflictMsg forbids its child,
// reported with ConflictCode, when Supplied holds: the parent's attributes and
// the child would both supply it.
type childRule struct {
	Match        func(local string) bool
	Supplied     func(parent *schemaNode) (bool, error)
	ForbiddenMsg string
	OrderMsg     string
	DupMsg       string
	PrecedingMsg string
	MissingMsg   string
	ConflictMsg  string
	MissingCode  xsderrors.Code
	ConflictCode xsderrors.Code
	Level        int
	MaxOne       bool
	Terminal     bool
}

// childOrder is the XSD 1.0 content model of one schema element in one
// placement, after its leading annotation, whose placement
// childContentViolation owns: ordered sections of rules. childContentFor
// selects it when the element starts, and childOrderState admits each child as it
// starts, so a misplaced, repeated or unknown child is the violation at its
// own start tag. A missing required child is the violation at the parent's
// end tag, the earliest point it is known; see childOrderState.missingChild.
type childOrder struct {
	// Label names the parent in the diagnostic of a child no rule matches.
	Label string
	Rules []childRule
}

// maxChildRules bounds the rules of one childOrder: childOrderState records
// the rules it has seen as bits of one uint64.
const maxChildRules = 64

// checkedChildOrder returns order, and panics when it holds more rules than
// childOrderState can record. Every childOrder is built at package
// initialisation, so an oversized order fails every test run and never reaches
// a schema.
func checkedChildOrder(order childOrder) childOrder {
	if len(order.Rules) > maxChildRules {
		panic("schema child order " + order.Label + " has more than " + strconv.Itoa(maxChildRules) + " rules")
	}
	return order
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

var simpleTypeChildOrder = checkedChildOrder(childOrder{
	Rules: []childRule{
		{
			Match:       matchChildLocal(restrictionChild, listChild, unionChild),
			MaxOne:      true,
			DupMsg:      "simpleType can contain one restriction, list, or union",
			MissingMsg:  "simpleType must contain one restriction, list, or union",
			MissingCode: xsderrors.CodeSchemaContentModel,
		},
	},
	Label: "simpleType",
})

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

var complexTypeChildOrder = checkedChildOrder(childOrder{
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
})

var complexContentChildOrder = derivationContainerOrder(complexContent)
var simpleContentChildOrder = derivationContainerOrder(simpleContent)
var simpleContentRestrictionChildOrder = simpleContentDerivationOrder(restrictionChild)
var simpleContentExtensionChildOrder = simpleContentDerivationOrder(extensionChild)
var complexContentRestrictionChildOrder = complexContentDerivationOrder(restrictionChild)
var complexContentExtensionChildOrder = complexContentDerivationOrder(extensionChild)
var attributeGroupDeclarationChildOrder = checkedChildOrder(childOrder{
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
})
var attributeDeclarationChildOrder = checkedChildOrder(childOrder{
	Rules: []childRule{
		{
			Match:        matchChildLocal(simpleTypeChild),
			MaxOne:       true,
			DupMsg:       "attribute can contain at most one simpleType",
			Supplied:     attributeTypeSupplied,
			ConflictMsg:  "attribute cannot have both type and simpleType",
			ConflictCode: xsderrors.CodeSchemaInvalidAttribute,
		},
	},
	Label: "attribute",
})
var elementDeclarationChildOrder = checkedChildOrder(childOrder{
	Rules: []childRule{
		{
			Match:        matchChildLocal(simpleTypeChild, complexTypeChild),
			Level:        0,
			MaxOne:       true,
			OrderMsg:     "element anonymous type must precede identity constraints",
			DupMsg:       "element can contain at most one anonymous type",
			Supplied:     elementTypeSupplied,
			ConflictMsg:  "element cannot have both type and anonymous type",
			ConflictCode: xsderrors.CodeSchemaInvalidAttribute,
		},
		{
			Match: matchChildLocal(uniqueChild, keyChild, keyrefChild),
			Level: 1,
		},
	},
	Label: "element",
})

// nestedModelGroupChildOrder is the content of xs:sequence and xs:choice,
// whose particles are nestedParticle in the schema for schemas: no xs:all.
// allModelGroupChildOrder is the content of xs:all, whose particles are
// element declarations only. A model group reference that resolves to xs:all
// is checked when the content model compiles.
var (
	nestedModelGroupChildOrder = checkedChildOrder(childOrder{
		Rules: []childRule{
			{Match: matchChildLocal(elementChild, sequenceChild, choiceChild, groupChild, anyChild)},
			{Match: matchChildLocal(allChild), ForbiddenMsg: "xs:all cannot be nested in model groups"},
		},
		Label: schemaComponentModelGroupLabel,
	})
	allModelGroupChildOrder = checkedChildOrder(childOrder{
		Rules: []childRule{
			{Match: matchChildLocal(elementChild)},
			{Match: matchChildLocal(sequenceChild, choiceChild, allChild, groupChild, anyChild), ForbiddenMsg: "xs:all can contain only element particles"},
		},
		Label: schemaComponentModelGroupLabel,
	})
)

var topLevelGroupChildOrder = checkedChildOrder(childOrder{
	Rules: []childRule{
		{
			Match:       matchChildLocal(sequenceChild, choiceChild, allChild),
			MaxOne:      true,
			DupMsg:      topLevelGroupModelGroupCount,
			MissingMsg:  topLevelGroupModelGroupCount,
			MissingCode: xsderrors.CodeSchemaContentModel,
		},
		{
			Match:        matchChildLocal(groupChild),
			ForbiddenMsg: "top-level group cannot contain group ref",
		},
	},
	Label: "top-level group",
})

var identityConstraintChildOrder = checkedChildOrder(childOrder{
	Rules: []childRule{
		{
			Match:       matchChildLocal(selectorChild),
			Level:       0,
			MaxOne:      true,
			OrderMsg:    identityConstraintOneSelector,
			DupMsg:      identityConstraintOneSelector,
			MissingMsg:  "identity constraint missing selector",
			MissingCode: xsderrors.CodeSchemaIdentity,
		},
		{
			Match:        matchChildLocal(fieldChild),
			Level:        1,
			PrecedingMsg: "identity constraint field requires selector",
			MissingMsg:   "identity constraint missing fields",
			MissingCode:  xsderrors.CodeSchemaIdentity,
		},
	},
	Label: "identity constraint",
})

// schemaChildOrder is the content of xs:schema, whose xs:annotation children
// may appear anywhere.
var schemaChildOrder = checkedChildOrder(childOrder{
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
})

// childOrderFor returns the content model of the XSD element named local,
// other than xs:schema, whose parent is named parent, or nil when the element
// admits only xs:annotation or is never admitted as a parent; childContentFor
// selects the content model of xs:schema. An element whose parent's
// content model does not admit it never starts, so parent here is one that
// admits local.
func childOrderFor(local string, parent xml.Name) *childOrder {
	switch local {
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
	seen      uint64
	levelSeen int
	// facets records the facets a restriction has specified, so a second one
	// of a kind is rejected at its own start tag (src-single-facet-value).
	facets       valuepkg.FacetMask
	terminalSeen bool
}

// accept admits n, the next XSD-namespace child of parent, into order, or
// returns why order forbids it there. The diagnostic is built only on
// rejection. Annotation placement is childContentViolation's.
func (s *childOrderState) accept(n *schemaSyntaxNode, order *childOrder, parent *schemaNode) error {
	child := n.Name.Local
	if s.terminalSeen {
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "invalid "+order.Label+" child "+diagnostictext.Quote(child))
	}
	if child == annotationChild {
		return nil
	}
	ruleIndex := childRuleIndex(child, order.Rules)
	if ruleIndex < 0 {
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "invalid "+order.Label+" child "+diagnostictext.Quote(child))
	}
	rule := &order.Rules[ruleIndex]
	switch {
	case rule.ForbiddenMsg != "":
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, rule.ForbiddenMsg)
	case s.levelSeen > rule.Level:
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, rule.OrderMsg)
	case rule.PrecedingMsg != "" && !s.seenBelow(rule.Level, order.Rules):
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, rule.PrecedingMsg)
	}
	bit := uint64(1) << ruleIndex
	if s.seen&bit != 0 && rule.MaxOne {
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, rule.DupMsg)
	}
	if err := rule.rejectConflict(n, parent); err != nil {
		return err
	}
	if err := s.acceptFacet(n); err != nil {
		return err
	}
	s.seen |= bit
	s.levelSeen = max(s.levelSeen, rule.Level)
	s.terminalSeen = rule.Terminal
	return nil
}

// acceptFacet records n when it is a facet and rejects a facet the parent
// already specifies. A restriction step specifies each facet at most once,
// except pattern and enumeration, whose repetitions form one facet value
// (src-single-facet-value).
func (s *childOrderState) acceptFacet(n *schemaSyntaxNode) error {
	mask, ok := facetMaskForLocal(n.Name.Local)
	if !ok || mask == valuepkg.FacetPattern || mask == valuepkg.FacetEnumeration {
		return nil
	}
	if s.facets&mask != 0 {
		return schemaCompileAt(n, xsderrors.CodeSchemaFacet, "duplicate "+facetLabel(n.Name.Local)+" facet")
	}
	s.facets |= mask
	return nil
}

// rejectMissingChild reports the first missing required child in rule order, so
// the diagnostic does not depend on which children parent has.
func (s *childOrderState) rejectMissingChild(order *childOrder, parent *schemaNode) error {
	for i := range order.Rules {
		rule := &order.Rules[i]
		if rule.MissingMsg == "" || s.seen&(uint64(1)<<i) != 0 {
			continue
		}
		supplied, err := rule.supplied(parent)
		if err != nil {
			return err
		}
		if !supplied {
			return schemaCompileAt(parent, rule.MissingCode, rule.MissingMsg)
		}
	}
	return nil
}

func (r *childRule) supplied(parent *schemaNode) (bool, error) {
	if r.Supplied == nil {
		return false, nil
	}
	return r.Supplied(parent)
}

func (r *childRule) rejectConflict(n *schemaSyntaxNode, parent *schemaNode) error {
	if r.ConflictMsg == "" {
		return nil
	}
	supplied, err := r.supplied(parent)
	if err != nil || !supplied {
		return err
	}
	return schemaCompileAt(n, r.ConflictCode, r.ConflictMsg)
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
	var supplied func(*schemaNode) (bool, error)
	var missing, conflict string
	switch kind {
	case simpleDerivationRestriction:
		derivation = restrictionChild
		singleChild = true
		allowFacets = true
		supplied = restrictionBaseSupplied
		missing = "simple restriction missing base"
		conflict = "restriction cannot have both base and simpleType"
	case simpleDerivationList:
		derivation = listChild
		singleChild = true
		supplied = listItemTypeSupplied
		missing = "list missing item type"
		conflict = "list cannot have both itemType and simpleType"
	case simpleDerivationUnion:
		derivation = unionChild
		supplied = unionMemberTypesSupplied
		missing = "union missing member types"
	case simpleDerivationInvalid:
		panic("invalid simple derivation kind")
	default:
		panic("unknown simple derivation kind")
	}
	rules := []childRule{
		{
			Match:        matchChildLocal(simpleTypeChild),
			Level:        0,
			MaxOne:       singleChild,
			OrderMsg:     derivation + " simpleType must precede facets",
			DupMsg:       derivation + " can contain one simpleType",
			Supplied:     supplied,
			MissingMsg:   missing,
			MissingCode:  xsderrors.CodeSchemaReference,
			ConflictMsg:  conflict,
			ConflictCode: xsderrors.CodeSchemaContentModel,
		},
	}
	if allowFacets {
		rules = append(rules, childRule{
			Match: isFacetLocal,
			Level: 1,
		})
	}
	return checkedChildOrder(childOrder{
		Rules: rules,
		Label: derivation,
	})
}

// elementTypeSupplied reports whether n, an element declaration, names its
// type in type (src-element.3).
func elementTypeSupplied(n *schemaNode) (bool, error) {
	source, err := requiredSemanticSource[*schemaElementSource](n)
	return err == nil && source.Type.Lexical.Present, err
}

// attributeTypeSupplied reports whether n, an attribute declaration, names
// its type in type (src-attribute.4).
func attributeTypeSupplied(n *schemaNode) (bool, error) {
	source, err := requiredSemanticSource[*schemaAttributeSource](n)
	return err == nil && source.Type.Lexical.Present, err
}

// restrictionBaseSupplied reports whether n, a simple xs:restriction, names
// its base type in base (src-simple-type.2).
func restrictionBaseSupplied(n *schemaNode) (bool, error) {
	source, err := requiredSemanticSource[*schemaBaseDerivationSource](n)
	return err == nil && source.Base.Lexical.Present, err
}

// listItemTypeSupplied reports whether n, an xs:list, names its item type in
// itemType (src-simple-type.3).
func listItemTypeSupplied(n *schemaNode) (bool, error) {
	source, err := requiredSemanticSource[*schemaListDerivationSource](n)
	return err == nil && source.ItemType.Lexical.Present, err
}

// unionMemberTypesSupplied reports whether n, an xs:union, names at least one
// member type in memberTypes (src-simple-type.4).
func unionMemberTypesSupplied(n *schemaNode) (bool, error) {
	source, err := requiredSemanticSource[*schemaUnionDerivationSource](n)
	return err == nil && len(source.MemberTypes.Names) != 0, err
}

func derivationContainerOrder(label string) childOrder {
	return checkedChildOrder(childOrder{
		Rules: []childRule{
			{
				Match:       matchChildLocal(extensionChild, restrictionChild),
				MaxOne:      true,
				DupMsg:      label + " can contain one derivation",
				MissingMsg:  label + " missing extension or restriction",
				MissingCode: xsderrors.CodeSchemaContentModel,
			},
		},
		Label: label,
	})
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
	return checkedChildOrder(childOrder{
		Rules: rules,
		Label: "simpleContent " + derivation,
	})
}

func complexContentDerivationOrder(derivation string) childOrder {
	return checkedChildOrder(childOrder{
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
	})
}

func matchChildLocal(locals ...string) func(string) bool {
	return func(local string) bool {
		return slices.Contains(locals, local)
	}
}
