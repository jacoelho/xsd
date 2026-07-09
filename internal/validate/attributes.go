package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func attributeValidation(ctx StartContext, msg string) error {
	return validation(ctx, xsderrors.CodeValidationAttribute, msg)
}

func isXSIAttributeName(name xml.Name) bool {
	return name.Space == vocab.XSINamespaceURI &&
		(name.Local == vocab.XSIAttrType ||
			name.Local == vocab.XSIAttrNil ||
			name.Local == vocab.XSIAttrSchemaLocation ||
			name.Local == vocab.XSIAttrNoNamespaceSchemaLocation)
}

// AttributeSeen tracks declared attributes seen on one element.
type AttributeSeen struct {
	list []bool
	mask uint64
}

// NewAttributeSeen returns presence state sized for n declared attribute uses.
func NewAttributeSeen(n int) AttributeSeen {
	if n > 64 {
		return AttributeSeen{list: make([]bool, n)}
	}
	return AttributeSeen{}
}

func newAttributeSeenWithScratch(n int, scratch *[]bool) AttributeSeen {
	if n <= 64 {
		return AttributeSeen{}
	}
	if n > maxRetainedSliceCap {
		return AttributeSeen{list: make([]bool, n)}
	}
	if cap(*scratch) < n {
		*scratch = make([]bool, n)
	} else {
		*scratch = (*scratch)[:n]
		clear(*scratch)
	}
	return AttributeSeen{list: *scratch}
}

func (s *AttributeSeen) mark(slot int) bool {
	if s.list != nil {
		if s.list[slot] {
			return false
		}
		s.list[slot] = true
		return true
	}
	bit := uint64(1) << slot
	if s.mask&bit != 0 {
		return false
	}
	s.mask |= bit
	return true
}

func (s *AttributeSeen) has(slot int) bool {
	if s.list != nil {
		return s.list[slot]
	}
	return s.mask&(uint64(1)<<slot) != 0
}

// AttributeRuntime supplies runtime facts needed for attribute wildcard matching.
type AttributeRuntime interface {
	WildcardView(id runtime.WildcardID) (runtime.WildcardView, bool)
	GlobalAttribute(name runtime.QName) (runtime.AttributeID, bool, bool)
}

// AttributeWildcardMatch is the result of matching an attribute wildcard.
type AttributeWildcardMatch struct {
	Attribute    runtime.AttributeID
	Matched      bool
	Skip         bool
	LaxMissing   bool
	HasAttribute bool
}

// MatchAttributeWildcard matches an instance attribute against an attribute wildcard.
func MatchAttributeWildcard(rt AttributeRuntime, wildcard runtime.WildcardID, name runtime.RuntimeName) (AttributeWildcardMatch, bool) {
	if wildcard == runtime.NoWildcard {
		return AttributeWildcardMatch{}, true
	}
	w, ok := rt.WildcardView(wildcard)
	if !ok {
		return AttributeWildcardMatch{}, false
	}
	if !w.AllowsURI(name.NS) {
		return AttributeWildcardMatch{}, true
	}
	if w.Process() == runtime.ProcessSkip {
		return AttributeWildcardMatch{Matched: true, Skip: true}, true
	}
	if name.Known {
		id, found, ok := rt.GlobalAttribute(name.Name)
		if !ok {
			return AttributeWildcardMatch{}, false
		}
		if found {
			return AttributeWildcardMatch{
				Attribute:    id,
				Matched:      true,
				HasAttribute: true,
			}, true
		}
	}
	return AttributeWildcardMatch{
		Matched:    true,
		LaxMissing: w.Process() == runtime.ProcessLax,
	}, true
}
