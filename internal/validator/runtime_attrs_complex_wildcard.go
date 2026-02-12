package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateComplexWildcardAttr(
	ct *runtime.ComplexType,
	validated []StartAttr,
	attr StartAttr,
	resolver value.NSResolver,
	storeAttrs bool,
	seenID *bool,
) ([]StartAttr, error) {
	if ct.AnyAttr == 0 {
		return nil, newValidationError(xsderrors.ErrAttributeNotDeclared, "attribute not declared")
	}
	if !s.rt.WildcardAccepts(ct.AnyAttr, attr.NSBytes, attr.NS) {
		return nil, newValidationError(xsderrors.ErrAttributeNotDeclared, "attribute wildcard rejected namespace")
	}

	rule := s.rt.Wildcards[ct.AnyAttr]
	wildcardAttr, resolved, err := s.resolveWildcardAttrID(rule.PC, attr.Sym)
	if err != nil {
		return nil, err
	}
	if !resolved {
		return s.appendRawValidatedAttr(validated, attr, storeAttrs), nil
	}
	if int(wildcardAttr) >= len(s.rt.Attributes) {
		return nil, fmt.Errorf("attribute %d out of range", wildcardAttr)
	}

	globalAttr := s.rt.Attributes[wildcardAttr]
	return s.validateComplexAttrValue(validated, attr, resolver, storeAttrs, attrValidationSpecFromRuntimeAttribute(globalAttr), seenID)
}

func (s *Session) resolveWildcardAttrID(pc runtime.ProcessContents, sym runtime.SymbolID) (runtime.AttrID, bool, error) {
	var wildcardAttr runtime.AttrID
	resolved, err := resolveWildcardSymbol(pc, sym, func(symbol runtime.SymbolID) bool {
		id, ok := s.globalAttributeBySymbol(symbol)
		if !ok {
			return false
		}
		wildcardAttr = id
		return true
	}, func() error {
		return newValidationError(xsderrors.ErrValidateWildcardAttrStrictUnresolved, "attribute wildcard strict unresolved")
	})
	if err != nil {
		return 0, false, err
	}
	if !resolved {
		return 0, false, nil
	}
	return wildcardAttr, true, nil
}
