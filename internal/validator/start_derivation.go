package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

// CheckStartTypeDerivation enforces xsi:type derivation compatibility against the
// declared element type and blocking/final constraints.
func CheckStartTypeDerivation(rt *runtime.Schema, derived, base runtime.TypeID, block runtime.ElemBlock) error {
	if derived == 0 || base == 0 {
		return fmt.Errorf("missing type for derivation check")
	}
	if derived == base {
		return nil
	}

	typ, ok := typeByID(rt, derived)
	if !ok {
		return fmt.Errorf("type %d not found", derived)
	}
	off := typ.AncOff
	ln := typ.AncLen
	if ln == 0 {
		return fmt.Errorf("type %d not derived from %d", derived, base)
	}
	startIDs, endIDs, okIDs := checkedTypeSpan(off, ln, len(rt.Ancestors.IDs))
	startMasks, endMasks, okMasks := checkedTypeSpan(off, ln, len(rt.Ancestors.Masks))
	if !okIDs || !okMasks {
		return fmt.Errorf("ancestor data out of range")
	}

	blocked := derivationBlockMask(block)
	if baseType, ok := typeByID(rt, base); ok {
		blocked |= baseType.Block
	}
	found := false
	for i := startIDs; i < endIDs; i++ {
		ancID := rt.Ancestors.IDs[i]
		if ancID == 0 {
			continue
		}
		maskIndex := startMasks + (i - startIDs)
		if maskIndex < startMasks || maskIndex >= endMasks {
			return fmt.Errorf("ancestor data out of range")
		}
		mask := rt.Ancestors.Masks[maskIndex]
		ancType, ok := typeByID(rt, ancID)
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

func typeByID(rt *runtime.Schema, id runtime.TypeID) (runtime.Type, bool) {
	if id == 0 || rt == nil || int(id) >= len(rt.Types) {
		return runtime.Type{}, false
	}
	return rt.Types[id], true
}

func checkedTypeSpan(off, ln uint32, size int) (start, end int, ok bool) {
	start = int(off)
	if ln == 0 {
		return start, start, start <= size
	}
	if start < 0 || start > size {
		return 0, 0, false
	}
	if off > ^uint32(0)-ln {
		return 0, 0, false
	}
	end = start + int(ln)
	if end < start || end > size {
		return 0, 0, false
	}
	return start, end, true
}
