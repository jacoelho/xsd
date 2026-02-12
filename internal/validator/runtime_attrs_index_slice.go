package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) attrUses(ref runtime.AttrIndexRef) []runtime.AttrUse {
	return sliceAttrUses(s.rt.AttrIndex.Uses, ref)
}

func sliceAttrUses(uses []runtime.AttrUse, ref runtime.AttrIndexRef) []runtime.AttrUse {
	if ref.Len == 0 {
		return nil
	}
	off := ref.Off
	end := off + ref.Len
	if int(off) >= len(uses) || int(end) > len(uses) {
		return nil
	}
	return uses[off:end]
}
