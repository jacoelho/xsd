package validator

import (
	"bytes"
	"fmt"

	xsdErrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

type StartMatchKind uint8

const (
	MatchNone StartMatchKind = iota
	MatchElem
	MatchWildcard
)

type StartMatch struct {
	Kind     StartMatchKind
	Elem     runtime.ElemID
	Wildcard runtime.WildcardID
}

type StartAttr struct {
	NSBytes    []byte
	Local      []byte
	Value      []byte
	KeyBytes   []byte
	Sym        runtime.SymbolID
	NS         runtime.NamespaceID
	NameCached bool
	KeyKind    runtime.ValueKind
}

type StartResult struct {
	Elem   runtime.ElemID
	Type   runtime.TypeID
	Nilled bool
	Skip   bool
}

func (s *Session) StartElement(match StartMatch, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte, attrs []StartAttr, resolver value.NSResolver) (StartResult, error) {
	if s == nil || s.rt == nil {
		return StartResult{}, newValidationError(xsdErrors.ErrSchemaNotLoaded, "schema not loaded")
	}

	decl, err := s.resolveMatch(match, sym, nsID, nsBytes)
	if err != nil {
		return StartResult{}, err
	}
	if decl == 0 {
		return StartResult{Skip: true}, nil
	}
	elem, ok := s.element(decl)
	if !ok {
		return StartResult{}, fmt.Errorf("element %d out of range", decl)
	}
	if elem.Flags&runtime.ElemAbstract != 0 {
		return StartResult{}, newValidationError(xsdErrors.ErrElementAbstract, "element is abstract")
	}

	xsiType, xsiNil, err := s.scanXsiAttributes(attrs)
	if err != nil {
		return StartResult{}, err
	}

	actualType := elem.Type
	if len(xsiType) > 0 {
		resolved, err := s.resolveXsiType(xsiType, resolver)
		if err != nil {
			return StartResult{}, newValidationError(xsdErrors.ErrValidateXsiTypeUnresolved, err.Error())
		}
		if err := s.checkTypeDerivation(resolved, actualType, elem.Block); err != nil {
			return StartResult{}, newValidationError(xsdErrors.ErrValidateXsiTypeDerivationBlocked, err.Error())
		}
		actualType = resolved
	}

	nilled := false
	if len(xsiNil) > 0 {
		flag, err := value.ParseBoolean(xsiNil)
		if err != nil {
			return StartResult{}, newValidationError(xsdErrors.ErrDatatypeInvalid, fmt.Sprintf("invalid xsi:nil: %v", err))
		}
		if flag {
			if elem.Flags&runtime.ElemNillable == 0 {
				return StartResult{}, newValidationError(xsdErrors.ErrValidateXsiNilNotNillable, "element is not nillable")
			}
			if elem.Fixed.Present {
				return StartResult{}, newValidationError(xsdErrors.ErrValidateNilledHasFixed, "element has fixed value and cannot be nilled")
			}
			nilled = true
		}
	}

	if typ, ok := s.typeByID(actualType); ok {
		if typ.Flags&runtime.TypeAbstract != 0 {
			return StartResult{}, newValidationError(xsdErrors.ErrElementTypeAbstract, "type is abstract")
		}
	}

	return StartResult{Elem: decl, Type: actualType, Nilled: nilled}, nil
}

func (s *Session) resolveMatch(match StartMatch, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (runtime.ElemID, error) {
	switch match.Kind {
	case MatchNone:
		return 0, newValidationError(xsdErrors.ErrUnexpectedElement, "no content model match")
	case MatchElem:
		if match.Elem == 0 {
			return 0, newValidationError(xsdErrors.ErrElementNotDeclared, "element not declared")
		}
		return match.Elem, nil
	case MatchWildcard:
		if match.Wildcard == 0 {
			return 0, newValidationError(xsdErrors.ErrWildcardNotDeclared, "wildcard match invalid")
		}
		if !s.rt.WildcardAccepts(match.Wildcard, nsBytes, nsID) {
			return 0, newValidationError(xsdErrors.ErrUnexpectedElement, "wildcard rejected namespace")
		}
		rule := s.rt.Wildcards[match.Wildcard]
		switch rule.PC {
		case runtime.PCSkip:
			return 0, nil
		case runtime.PCLax, runtime.PCStrict:
			if sym == 0 {
				if rule.PC == runtime.PCStrict {
					return 0, newValidationError(xsdErrors.ErrValidateWildcardElemStrictUnresolved, "wildcard strict unresolved element")
				}
				return 0, nil
			}
			elem, ok := s.globalElementBySymbol(sym)
			if !ok {
				if rule.PC == runtime.PCStrict {
					return 0, newValidationError(xsdErrors.ErrValidateWildcardElemStrictUnresolved, "wildcard strict unresolved element")
				}
				return 0, nil
			}
			return elem, nil
		default:
			return 0, fmt.Errorf("unknown wildcard processContents %d", rule.PC)
		}
	default:
		return 0, fmt.Errorf("unknown match kind %d", match.Kind)
	}
}

func (s *Session) scanXsiAttributes(attrs []StartAttr) ([]byte, []byte, error) {
	var xsiType []byte
	var xsiNil []byte
	predef := s.rt.Predef
	xsiNS := s.rt.PredefNS.Xsi
	xsiNSBytes := s.rt.Namespaces.Bytes(xsiNS)
	typeLocal := []byte("type")
	nilLocal := []byte("nil")
	for _, attr := range attrs {
		switch attr.Sym {
		case predef.XsiType:
			if len(xsiType) > 0 {
				return nil, nil, newValidationError(xsdErrors.ErrDatatypeInvalid, "duplicate xsi:type attribute")
			}
			xsiType = attr.Value
		case predef.XsiNil:
			if len(xsiNil) > 0 {
				return nil, nil, newValidationError(xsdErrors.ErrDatatypeInvalid, "duplicate xsi:nil attribute")
			}
			xsiNil = attr.Value
			continue
		}
		if attr.Sym != 0 {
			continue
		}
		if !isXsiNamespace(attr, xsiNS, xsiNSBytes) {
			continue
		}
		if bytes.Equal(attr.Local, typeLocal) {
			if len(xsiType) > 0 {
				return nil, nil, newValidationError(xsdErrors.ErrDatatypeInvalid, "duplicate xsi:type attribute")
			}
			xsiType = attr.Value
			continue
		}
		if bytes.Equal(attr.Local, nilLocal) {
			if len(xsiNil) > 0 {
				return nil, nil, newValidationError(xsdErrors.ErrDatatypeInvalid, "duplicate xsi:nil attribute")
			}
			xsiNil = attr.Value
		}
	}
	return xsiType, xsiNil, nil
}

func isXsiNamespace(attr StartAttr, xsiNS runtime.NamespaceID, xsiNSBytes []byte) bool {
	if attr.NS != 0 {
		return attr.NS == xsiNS
	}
	if len(attr.NSBytes) == 0 || len(xsiNSBytes) == 0 {
		return false
	}
	return bytes.Equal(attr.NSBytes, xsiNSBytes)
}

func (s *Session) resolveXsiType(valueBytes []byte, resolver value.NSResolver) (runtime.TypeID, error) {
	canonical, err := value.CanonicalQName(valueBytes, resolver, nil)
	if err != nil {
		return 0, fmt.Errorf("resolve xsi:type: %w", err)
	}
	ns, local, err := splitCanonicalQName(canonical)
	if err != nil {
		return 0, err
	}
	var nsID runtime.NamespaceID
	if len(ns) == 0 {
		nsID = s.rt.PredefNS.Empty
	} else {
		nsID = s.rt.Namespaces.Lookup(ns)
		if nsID == 0 {
			return 0, fmt.Errorf("xsi:type namespace not found")
		}
	}
	sym := s.rt.Symbols.Lookup(nsID, local)
	if sym == 0 {
		return 0, fmt.Errorf("xsi:type symbol not found")
	}
	id, ok := s.typeBySymbolID(sym)
	if !ok {
		return 0, fmt.Errorf("xsi:type %d not found", sym)
	}
	return id, nil
}

func splitCanonicalQName(valueBytes []byte) ([]byte, []byte, error) {
	for i, b := range valueBytes {
		if b == 0 {
			return valueBytes[:i], valueBytes[i+1:], nil
		}
	}
	return nil, nil, fmt.Errorf("invalid canonical QName")
}

func (s *Session) checkTypeDerivation(derived, base runtime.TypeID, block runtime.ElemBlock) error {
	if derived == 0 || base == 0 {
		return fmt.Errorf("missing type for derivation check")
	}
	if derived == base {
		return nil
	}

	typ, ok := s.typeByID(derived)
	if !ok {
		return fmt.Errorf("type %d not found", derived)
	}
	off := typ.AncOff
	ln := typ.AncLen
	if ln == 0 {
		return fmt.Errorf("type %d not derived from %d", derived, base)
	}
	end := off + ln
	if int(end) > len(s.rt.Ancestors.IDs) || int(end) > len(s.rt.Ancestors.Masks) {
		return fmt.Errorf("ancestor data out of range")
	}

	blocked := derivationBlockMask(block)
	if baseType, ok := s.typeByID(base); ok {
		blocked |= baseType.Block
	}
	found := false
	for i := off; i < end; i++ {
		ancID := s.rt.Ancestors.IDs[i]
		if ancID == 0 {
			continue
		}
		mask := s.rt.Ancestors.Masks[i]
		ancType, ok := s.typeByID(ancID)
		if !ok {
			return fmt.Errorf("type %d not found", ancID)
		}
		if ancType.Final&mask != 0 {
			return fmt.Errorf("derivation blocked by final")
		}
		if ancID == base {
			if blocked&mask != 0 {
				return fmt.Errorf("derivation blocked")
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("type %d not derived from %d", derived, base)
	}
	return nil
}

func derivationBlockMask(block runtime.ElemBlock) runtime.DerivationMethod {
	var mask runtime.DerivationMethod
	if block&runtime.ElemBlockExtension != 0 {
		mask |= runtime.DerExtension
	}
	if block&runtime.ElemBlockRestriction != 0 {
		mask |= runtime.DerRestriction
	}
	return mask
}

func (s *Session) globalElementBySymbol(sym runtime.SymbolID) (runtime.ElemID, bool) {
	if sym == 0 {
		return 0, false
	}
	if s.rt == nil || int(sym) >= len(s.rt.GlobalElements) {
		return 0, false
	}
	id := s.rt.GlobalElements[sym]
	return id, id != 0
}

func (s *Session) typeBySymbolID(sym runtime.SymbolID) (runtime.TypeID, bool) {
	if sym == 0 {
		return 0, false
	}
	if s.rt == nil || int(sym) >= len(s.rt.GlobalTypes) {
		return 0, false
	}
	id := s.rt.GlobalTypes[sym]
	return id, id != 0
}

func (s *Session) element(id runtime.ElemID) (runtime.Element, bool) {
	if id == 0 || int(id) >= len(s.rt.Elements) {
		return runtime.Element{}, false
	}
	return s.rt.Elements[id], true
}

func (s *Session) typeByID(id runtime.TypeID) (runtime.Type, bool) {
	if id == 0 || int(id) >= len(s.rt.Types) {
		return runtime.Type{}, false
	}
	return s.rt.Types[id], true
}
