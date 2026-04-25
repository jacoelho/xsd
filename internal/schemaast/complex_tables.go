package schemaast

const (
	attrSetAnyElement         = "any"
	attrSetAnyAttribute       = "anyAttribute"
	attrSetModelGroup         = "modelGroup"
	attrSetTopLevelGroup      = "topLevelGroup"
	attrSetIdentityConstraint = "identityConstraint"
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
)
