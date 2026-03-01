package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

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
	startIDs, endIDs, okIDs := checkedSpan(off, ln, len(s.rt.Ancestors.IDs))
	startMasks, endMasks, okMasks := checkedSpan(off, ln, len(s.rt.Ancestors.Masks))
	if !okIDs || !okMasks {
		return fmt.Errorf("ancestor data out of range")
	}

	blocked := derivationBlockMask(block)
	if baseType, ok := s.typeByID(base); ok {
		blocked |= baseType.Block
	}
	found := false
	for i := startIDs; i < endIDs; i++ {
		ancID := s.rt.Ancestors.IDs[i]
		if ancID == 0 {
			continue
		}
		maskIndex := startMasks + (i - startIDs)
		if maskIndex < startMasks || maskIndex >= endMasks {
			return fmt.Errorf("ancestor data out of range")
		}
		mask := s.rt.Ancestors.Masks[maskIndex]
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
