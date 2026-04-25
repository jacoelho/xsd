package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) applySessionPlan() {
	if s == nil {
		return
	}
	plan := s.plan
	maxAttrs := sessionHint(plan.MaxAttrs, maxSessionEntries)
	if maxAttrs > 0 {
		s.attrs.attrState.Classes = make([]Class, 0, maxAttrs)
		s.attrs.attrState.Starts = make([]Start, 0, maxAttrs)
		s.attrs.attrState.Validated = make([]Start, 0, maxAttrs)
		s.attrs.attrAppliedBuf = make([]Applied, 0, maxAttrs)
		s.identity.attrScratch = make([]Attr, 0, maxAttrs)
		s.identity.identityAttrs.Buckets = make(map[uint64][]AttrNameID, maxAttrs)
		s.identity.identityAttrs.NS = make([]byte, 0, maxAttrs*16)
		s.identity.identityAttrs.Local = make([]byte, 0, maxAttrs*12)
	}
	maxAttrUses := sessionHint(plan.MaxAttrUses, maxSessionEntries)
	if maxAttrUses > 0 {
		s.attrs.attrState.Present = make([]bool, 0, maxAttrUses)
	}
	if maxAttrs > SmallDuplicateThreshold {
		seen := runtime.NextPow2(maxAttrs * 2)
		if seen <= maxSessionEntries {
			s.attrs.attrState.Seen = make([]SeenEntry, 0, seen)
		}
	}
	maxIdentityFields := sessionHint(plan.MaxIdentityFields, maxSessionEntries)
	if maxIdentityFields > 0 {
		s.identity.identityAttrs.Names = make([]AttrName, 0, maxIdentityFields)
	}
	nameHint := sessionHint(plan.NameHint, maxSessionEntries)
	if nameHint > 0 {
		s.Names.Dense = make([]NameEntry, 0, nameHint+1)
	}
	if nameBytes := sessionHint(plan.NameBytesHint, maxSessionBuffer); nameBytes > 0 {
		s.Names.Local = make([]byte, 0, nameBytes)
	}
	if namespaceBytes := sessionHint(plan.NamespaceBytesHint, maxSessionBuffer); namespaceBytes > 0 {
		s.Names.NS = make([]byte, 0, namespaceBytes)
	}
}

func sessionHint(value, limit int) int {
	if value <= 0 {
		return 0
	}
	return min(value, limit)
}
