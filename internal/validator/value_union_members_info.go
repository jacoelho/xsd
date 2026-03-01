package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) unionMemberInfo(meta runtime.ValidatorMeta) ([]runtime.ValidatorID, []runtime.TypeID, []uint8, bool) {
	if int(meta.Index) >= len(s.rt.Validators.Union) {
		return nil, nil, nil, false
	}
	union := s.rt.Validators.Union[meta.Index]
	startMembers, endMembers, okMembers := checkedSpan(union.MemberOff, union.MemberLen, len(s.rt.Validators.UnionMembers))
	startTypes, endTypes, okTypes := checkedSpan(union.MemberOff, union.MemberLen, len(s.rt.Validators.UnionMemberTypes))
	startWS, endWS, okWS := checkedSpan(union.MemberOff, union.MemberLen, len(s.rt.Validators.UnionMemberSameWS))
	if !okMembers || !okTypes || !okWS {
		return nil, nil, nil, false
	}
	return s.rt.Validators.UnionMembers[startMembers:endMembers],
		s.rt.Validators.UnionMemberTypes[startTypes:endTypes],
		s.rt.Validators.UnionMemberSameWS[startWS:endWS],
		true
}
