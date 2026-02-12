package validator

import "github.com/jacoelho/xsd/internal/runtime"

func lookupAttrUse(rt *runtime.Schema, ref runtime.AttrIndexRef, sym runtime.SymbolID) (runtime.AttrUse, int, bool) {
	if rt == nil {
		return runtime.AttrUse{}, -1, false
	}
	uses := sliceAttrUses(rt.AttrIndex.Uses, ref)
	switch ref.Mode {
	case runtime.AttrIndexSortedBinary:
		return lookupAttrUseBinary(uses, sym)
	case runtime.AttrIndexHash:
		return lookupAttrUseHash(rt, ref, sym)
	default:
		return lookupAttrUseLinear(uses, sym)
	}
}
