package validator

import "github.com/jacoelho/xsd/internal/runtime"

func attrValidationSpecFromAttrUse(use runtime.AttrUse) attrValidationSpec {
	return attrValidationSpec{
		validator:   use.Validator,
		fixed:       use.Fixed,
		fixedKey:    use.FixedKey,
		fixedMember: use.FixedMember,
	}
}

func attrValidationSpecFromRuntimeAttribute(attr runtime.Attribute) attrValidationSpec {
	return attrValidationSpec{
		validator:   attr.Validator,
		fixed:       attr.Fixed,
		fixedKey:    attr.FixedKey,
		fixedMember: attr.FixedMember,
	}
}
