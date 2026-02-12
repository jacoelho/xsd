package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) unionMemberInfo(meta runtime.ValidatorMeta) ([]runtime.ValidatorID, []runtime.TypeID, []uint8, bool) {
	if int(meta.Index) >= len(s.rt.Validators.Union) {
		return nil, nil, nil, false
	}
	union := s.rt.Validators.Union[meta.Index]
	end := union.MemberOff + union.MemberLen
	if int(end) > len(s.rt.Validators.UnionMembers) || int(end) > len(s.rt.Validators.UnionMemberTypes) || int(end) > len(s.rt.Validators.UnionMemberSameWS) {
		return nil, nil, nil, false
	}
	return s.rt.Validators.UnionMembers[union.MemberOff:end],
		s.rt.Validators.UnionMemberTypes[union.MemberOff:end],
		s.rt.Validators.UnionMemberSameWS[union.MemberOff:end],
		true
}
