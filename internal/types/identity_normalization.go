package types

type identityNormalizationState uint8

const (
	identityStateUnknown identityNormalizationState = iota
	identityStateVisiting
	identityStateDone
)

func (s *SimpleType) precomputeIdentityNormalization() {
	if s == nil || s.identityNormalizationReady {
		return
	}
	if s.Variety() == AtomicVariety {
		s.identityNormalizable = true
		s.identityNormalizationReady = true
		return
	}
	state := make(map[*SimpleType]identityNormalizationState)
	precomputeIdentityNormalization(s, state)
}

func precomputeIdentityNormalization(s *SimpleType, state map[*SimpleType]identityNormalizationState) bool {
	if s == nil {
		return false
	}
	switch state[s] {
	case identityStateDone:
		return s.identityNormalizable
	case identityStateVisiting:
		return false
	}

	state[s] = identityStateVisiting

	var (
		normalizable bool
		listItem     Type
		members      []Type
	)

	switch s.Variety() {
	case ListVariety:
		itemType, ok := listItemType(s, make(map[Type]bool))
		if ok && itemType != nil {
			listItem = itemType
			normalizable = identityNormalizableType(itemType, state)
		}
	case UnionVariety:
		unionMembers := unionMemberTypesForIdentity(s)
		if len(unionMembers) > 0 {
			members = make([]Type, 0, len(unionMembers))
			for _, member := range unionMembers {
				if identityNormalizableType(member, state) {
					members = append(members, member)
				}
			}
			normalizable = len(members) > 0
		}
	default:
		normalizable = true
	}

	s.identityNormalizable = normalizable
	s.identityListItemType = listItem
	if s.Variety() == UnionVariety {
		s.identityMemberTypes = members
	} else {
		s.identityMemberTypes = nil
	}
	s.identityNormalizationReady = true
	state[s] = identityStateDone
	return normalizable
}

func identityNormalizableType(typ Type, state map[*SimpleType]identityNormalizationState) bool {
	if typ == nil {
		return false
	}
	if st, ok := AsSimpleType(typ); ok {
		return precomputeIdentityNormalization(st, state)
	}
	return true
}

func unionMemberTypesForIdentity(st *SimpleType) []Type {
	if st == nil {
		return nil
	}
	if len(st.MemberTypes) > 0 {
		return st.MemberTypes
	}
	if st.ResolvedBase != nil {
		if baseST, ok := AsSimpleType(st.ResolvedBase); ok && baseST.Variety() == UnionVariety && len(baseST.MemberTypes) > 0 {
			return baseST.MemberTypes
		}
	}
	return nil
}

// IdentityNormalizable reports whether typ can be normalized for identity constraints
// without encountering cycles.
func IdentityNormalizable(typ Type) bool {
	if typ == nil {
		return false
	}
	if st, ok := AsSimpleType(typ); ok {
		if !st.identityNormalizationReady {
			st.precomputeIdentityNormalization()
		}
		return st.identityNormalizable
	}
	return true
}

// IdentityListItemType returns the resolved list item type for identity normalization.
// It returns false when typ is not a list type or has no resolvable item type.
func IdentityListItemType(typ Type) (Type, bool) {
	if typ == nil {
		return nil, false
	}
	if st, ok := AsSimpleType(typ); ok {
		if st.Variety() != ListVariety {
			return nil, false
		}
		if !st.identityNormalizationReady {
			st.precomputeIdentityNormalization()
		}
		if st.identityListItemType == nil {
			return nil, false
		}
		return st.identityListItemType, true
	}
	if bt, ok := AsBuiltinType(typ); ok {
		if itemName, ok := builtinListItemTypeName(bt.Name().Local); ok {
			if item := GetBuiltin(itemName); item != nil {
				return item, true
			}
		}
	}
	return nil, false
}

// IdentityMemberTypes returns the union member types eligible for identity normalization.
func IdentityMemberTypes(typ Type) []Type {
	st, ok := AsSimpleType(typ)
	if !ok || st.Variety() != UnionVariety {
		return nil
	}
	if !st.identityNormalizationReady {
		st.precomputeIdentityNormalization()
	}
	return st.identityMemberTypes
}
