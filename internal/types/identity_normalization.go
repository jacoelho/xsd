package types

type identityNormalizationState uint8

const (
	identityStateUnknown identityNormalizationState = iota
	identityStateVisiting
	identityStateDone
)

func (s *SimpleType) precomputeIdentityNormalization() {
	if s == nil {
		return
	}
	typeCacheMu.Lock()
	for !s.identityNormalizationReady && s.identityNormalizationComputing {
		typeCacheCond.Wait()
	}
	if s.identityNormalizationReady {
		typeCacheMu.Unlock()
		return
	}
	s.identityNormalizationComputing = true
	typeCacheMu.Unlock()

	if s.Variety() == AtomicVariety {
		typeCacheMu.Lock()
		s.identityNormalizable = true
		s.identityNormalizationReady = true
		s.identityNormalizationComputing = false
		typeCacheMu.Unlock()
		typeCacheCond.Broadcast()
		return
	}
	state := make(map[*SimpleType]identityNormalizationState)
	precomputeIdentityNormalization(s, state)

	typeCacheMu.Lock()
	s.identityNormalizationComputing = false
	typeCacheMu.Unlock()
	typeCacheCond.Broadcast()
}

func precomputeIdentityNormalization(s *SimpleType, state map[*SimpleType]identityNormalizationState) bool {
	if s == nil {
		return false
	}
	typeCacheMu.RLock()
	ready := s.identityNormalizationReady
	normalizable := s.identityNormalizable
	typeCacheMu.RUnlock()
	if ready {
		return normalizable
	}
	switch state[s] {
	case identityStateDone:
		return normalizable
	case identityStateVisiting:
		return false
	}

	state[s] = identityStateVisiting

	var (
		calculatedNormalizable bool
		listItem               Type
		members                []Type
	)

	switch s.Variety() {
	case ListVariety:
		itemType, ok := listItemType(s, make(map[Type]bool))
		if ok && itemType != nil {
			listItem = itemType
			calculatedNormalizable = identityNormalizableType(itemType, state)
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
			calculatedNormalizable = len(members) > 0
		}
	default:
		calculatedNormalizable = true
	}

	typeCacheMu.Lock()
	s.identityNormalizable = calculatedNormalizable
	s.identityListItemType = listItem
	if s.Variety() == UnionVariety {
		s.identityMemberTypes = members
	} else {
		s.identityMemberTypes = nil
	}
	s.identityNormalizationReady = true
	typeCacheMu.Unlock()
	state[s] = identityStateDone
	return calculatedNormalizable
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
		typeCacheMu.RLock()
		ready := st.identityNormalizationReady
		normalizable := st.identityNormalizable
		typeCacheMu.RUnlock()
		if !ready {
			st.precomputeIdentityNormalization()
			typeCacheMu.RLock()
			normalizable = st.identityNormalizable
			typeCacheMu.RUnlock()
		}
		return normalizable
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
		typeCacheMu.RLock()
		ready := st.identityNormalizationReady
		listItem := st.identityListItemType
		typeCacheMu.RUnlock()
		if !ready {
			st.precomputeIdentityNormalization()
			typeCacheMu.RLock()
			listItem = st.identityListItemType
			typeCacheMu.RUnlock()
		}
		if listItem == nil {
			return nil, false
		}
		return listItem, true
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
	typeCacheMu.RLock()
	ready := st.identityNormalizationReady
	members := st.identityMemberTypes
	typeCacheMu.RUnlock()
	if !ready {
		st.precomputeIdentityNormalization()
		typeCacheMu.RLock()
		members = st.identityMemberTypes
		typeCacheMu.RUnlock()
	}
	return members
}
