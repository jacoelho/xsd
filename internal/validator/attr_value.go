package validator

import "github.com/jacoelho/xsd/internal/runtime"

// ValueSpec describes the validator and fixed-value policy for one attribute value.
type ValueSpec struct {
	Validator   runtime.ValidatorID
	FixedMember runtime.ValidatorID
	Fixed       runtime.ValueRef
	FixedKey    runtime.ValueKeyRef
}

// SpecFromUse converts one runtime attribute use into a value-validation spec.
func SpecFromUse(use runtime.AttrUse) ValueSpec {
	return ValueSpec{
		Validator:   use.Validator,
		Fixed:       use.Fixed,
		FixedKey:    use.FixedKey,
		FixedMember: use.FixedMember,
	}
}

// SpecFromAttribute converts one runtime global attribute into a value-validation spec.
func SpecFromAttribute(attr runtime.Attribute) ValueSpec {
	return ValueSpec{
		Validator:   attr.Validator,
		Fixed:       attr.Fixed,
		FixedKey:    attr.FixedKey,
		FixedMember: attr.FixedMember,
	}
}
