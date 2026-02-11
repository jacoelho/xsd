package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

type defaultFixedPolicy struct {
	value   runtime.ValueRef
	key     runtime.ValueKeyRef
	member  runtime.ValidatorID
	fixed   bool
	present bool
}

func selectAttrDefaultFixed(use *runtime.AttrUse) defaultFixedPolicy {
	if use == nil {
		return defaultFixedPolicy{}
	}
	if use.Fixed.Present {
		return defaultFixedPolicy{
			value:   use.Fixed,
			key:     use.FixedKey,
			member:  use.FixedMember,
			fixed:   true,
			present: true,
		}
	}
	if use.Default.Present {
		return defaultFixedPolicy{
			value:   use.Default,
			key:     use.DefaultKey,
			member:  use.DefaultMember,
			present: true,
		}
	}
	return defaultFixedPolicy{}
}

func (s *Session) materializePolicyKey(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID, stored runtime.ValueKeyRef) (runtime.ValueKind, []byte, error) {
	kind := stored.Kind
	key := valueBytes(s.rt.Values, stored.Ref)
	if stored.Ref.Present {
		return kind, key, nil
	}
	return s.keyForCanonicalValue(validator, canonical, nil, member)
}

func (s *Session) materializeObservedKey(
	validator runtime.ValidatorID,
	canonical []byte,
	resolver value.NSResolver,
	member runtime.ValidatorID,
	metrics valueMetrics,
) (runtime.ValueKind, []byte, error) {
	if metrics.keyKind != runtime.VKInvalid {
		return metrics.keyKind, metrics.keyBytes, nil
	}
	return s.keyForCanonicalValue(validator, canonical, resolver, member)
}

func selectTextDefaultFixed(hasContent bool, elem runtime.Element, elemOK bool, ct runtime.ComplexType, hasComplexText bool) defaultFixedPolicy {
	if hasContent {
		return defaultFixedPolicy{}
	}
	switch {
	case elemOK && elem.Fixed.Present:
		return defaultFixedPolicy{
			value:   elem.Fixed,
			key:     elem.FixedKey,
			member:  elem.FixedMember,
			fixed:   true,
			present: true,
		}
	case elemOK && elem.Default.Present:
		return defaultFixedPolicy{
			value:   elem.Default,
			key:     elem.DefaultKey,
			member:  elem.DefaultMember,
			present: true,
		}
	case hasComplexText && ct.TextFixed.Present:
		return defaultFixedPolicy{
			value:   ct.TextFixed,
			member:  ct.TextFixedMember,
			fixed:   true,
			present: true,
		}
	case hasComplexText && ct.TextDefault.Present:
		return defaultFixedPolicy{
			value:   ct.TextDefault,
			member:  ct.TextDefaultMember,
			present: true,
		}
	default:
		return defaultFixedPolicy{}
	}
}

func selectTextFixedConstraint(elem runtime.Element, elemOK bool, ct runtime.ComplexType, hasComplexText bool) defaultFixedPolicy {
	switch {
	case elemOK && elem.Fixed.Present:
		return defaultFixedPolicy{
			value:   elem.Fixed,
			key:     elem.FixedKey,
			member:  elem.FixedMember,
			fixed:   true,
			present: true,
		}
	case hasComplexText && ct.TextFixed.Present:
		return defaultFixedPolicy{
			value:   ct.TextFixed,
			member:  ct.TextFixedMember,
			fixed:   true,
			present: true,
		}
	default:
		return defaultFixedPolicy{}
	}
}
