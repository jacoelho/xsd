package parser

const (
	attrSetAnyElement         = "any"
	attrSetAnyAttribute       = "anyAttribute"
	attrSetModelGroup         = "modelGroup"
	attrSetTopLevelGroup      = "topLevelGroup"
	attrSetIdentityConstraint = "identityConstraint"

	childSetSimpleContentFacet  = "simpleContentFacet"
	childSetComplexContentChild = "complexContentChild"
)

var (
	validAttributeNames = map[string]map[string]bool{
		attrSetAnyElement: {
			"namespace":       true,
			"processContents": true,
			"minOccurs":       true,
			"maxOccurs":       true,
			"id":              true,
		},
		attrSetAnyAttribute: {
			"namespace":       true,
			"processContents": true,
			"id":              true,
		},
		attrSetModelGroup: {
			"id":        true,
			"minOccurs": true,
			"maxOccurs": true,
		},
		attrSetTopLevelGroup: {
			"id":   true,
			"name": true,
		},
		attrSetIdentityConstraint: {
			"xpath": true,
			"id":    true,
		},
	}
	validChildElementNames = map[string]map[string]bool{
		childSetSimpleContentFacet: {
			"length":         true,
			"minLength":      true,
			"maxLength":      true,
			"pattern":        true,
			"enumeration":    true,
			"whiteSpace":     true,
			"maxInclusive":   true,
			"maxExclusive":   true,
			"minInclusive":   true,
			"minExclusive":   true,
			"totalDigits":    true,
			"fractionDigits": true,
		},
		childSetComplexContentChild: {
			"annotation":     true,
			"sequence":       true,
			"choice":         true,
			"all":            true,
			"group":          true,
			"element":        true,
			"any":            true,
			"attribute":      true,
			"attributeGroup": true,
			"anyAttribute":   true,
		},
	}
	validNamespaceConstraintTokens = map[string]bool{
		"##targetNamespace": true,
		"##local":           true,
	}
)
