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
	ancestorIDs := rt.AncestorIDs(off, ln)
	ancestorMasks := rt.AncestorMasks(off, ln)
	if len(ancestorIDs) != int(ln) || len(ancestorMasks) != int(ln) {
		return fmt.Errorf("ancestor data out of range")
	}

	blocked := derivationBlockMask(block)
	if baseType, ok := typeByID(rt, base); ok {
		blocked |= baseType.Block
	}
	found := false
	for i, ancID := range ancestorIDs {
		if ancID == 0 {
			continue
		}
		mask := ancestorMasks[i]
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
	return rt.Type(id)
}
