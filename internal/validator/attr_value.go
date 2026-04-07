package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

// ValueSpec describes the validator and fixed-value policy for one attribute value.
type ValueSpec struct {
	Validator   runtime.ValidatorID
	FixedMember runtime.ValidatorID
	Fixed       runtime.ValueRef
	FixedKey    runtime.ValueKeyRef
}

// ValueResult records the canonical bytes and semantic key derived for one attribute value.
type ValueResult struct {
	Canonical []byte
	KeyBytes  []byte
	KeyKind   runtime.ValueKind
	HasKey    bool
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

// ValidateValueCallbacks supplies session-bound value validation behavior.
type ValidateValueCallbacks struct {
	Validate        func(runtime.ValidatorID, []byte, bool) (ValueResult, error)
	IsIDValidator   func(runtime.ValidatorID) bool
	AppendCanonical func([]Start, Start, bool, []byte, runtime.ValueKind, []byte) []Start
	MatchFixed      func(ValueSpec, ValueResult) (bool, error)
}

// ValidateValue validates one attribute value, tracks duplicate IDs, stores the
// canonical result when requested, and enforces fixed-value policy.
func ValidateValue(
	validated []Start,
	attr Start,
	store bool,
	spec ValueSpec,
	seenID *bool,
	callbacks ValidateValueCallbacks,
) ([]Start, error) {
	result, err := callbacks.Validate(spec.Validator, attr.Value, store)
	if err != nil {
		return nil, err
	}
	if callbacks.IsIDValidator != nil && callbacks.IsIDValidator(spec.Validator) {
		if *seenID {
			return nil, xsderrors.New(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
		}
		*seenID = true
	}
	if callbacks.AppendCanonical != nil {
		validated = callbacks.AppendCanonical(validated, attr, store, result.Canonical, result.KeyKind, result.KeyBytes)
	}
	if spec.Fixed.Present && callbacks.MatchFixed != nil {
		match, err := callbacks.MatchFixed(spec, result)
		if err != nil {
			return nil, err
		}
		if !match {
			return nil, xsderrors.New(xsderrors.ErrAttributeFixedValue, "fixed attribute value mismatch")
		}
	}
	return validated, nil
}
