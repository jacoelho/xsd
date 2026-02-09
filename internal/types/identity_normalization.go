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
	guard := s.guard()
	guard.mu.Lock()
	for !s.identityNormalizationReady && s.identityNormalizationComputing {
		guard.cond.Wait()
	}
	if s.identityNormalizationReady {
		guard.mu.Unlock()
		return
	}
	s.identityNormalizationComputing = true
	guard.mu.Unlock()

	if s.Variety() == AtomicVariety {
		guard.mu.Lock()
		defer guard.mu.Unlock()
		s.identityNormalizable = true
		s.identityNormalizationReady = true
		s.identityNormalizationComputing = false
		guard.cond.Broadcast()
		return
	}
	state := make(map[*SimpleType]identityNormalizationState)
	precomputeIdentityNormalization(s, state)

	guard.mu.Lock()
	defer guard.mu.Unlock()
	s.identityNormalizationComputing = false
	guard.cond.Broadcast()
}

func precomputeIdentityNormalization(s *SimpleType, state map[*SimpleType]identityNormalizationState) bool {
	if s == nil {
		return false
	}
	guard := s.guard()
	guard.mu.RLock()
	ready := s.identityNormalizationReady
	normalizable := s.identityNormalizable
	guard.mu.RUnlock()
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
		itemType, ok := ListItemType(s)
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

	guard.mu.Lock()
	defer guard.mu.Unlock()
	s.identityNormalizable = calculatedNormalizable
	s.identityListItemType = listItem
	if s.Variety() == UnionVariety {
		s.identityMemberTypes = members
	} else {
		s.identityMemberTypes = nil
	}
	s.identityNormalizationReady = true
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
		guard := st.guard()
		guard.mu.RLock()
		ready := st.identityNormalizationReady
		normalizable := st.identityNormalizable
		guard.mu.RUnlock()
		if !ready {
			st.precomputeIdentityNormalization()
			guard.mu.RLock()
			normalizable = st.identityNormalizable
			guard.mu.RUnlock()
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
		guard := st.guard()
		guard.mu.RLock()
		ready := st.identityNormalizationReady
		listItem := st.identityListItemType
		guard.mu.RUnlock()
		if !ready {
			st.precomputeIdentityNormalization()
			guard.mu.RLock()
			listItem = st.identityListItemType
			guard.mu.RUnlock()
		}
		if listItem == nil {
			return nil, false
		}
		return listItem, true
	}
	if bt, ok := AsBuiltinType(typ); ok {
		if itemName, ok := BuiltinListItemTypeName(bt.Name().Local); ok {
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
	guard := st.guard()
	guard.mu.RLock()
	ready := st.identityNormalizationReady
	members := st.identityMemberTypes
	guard.mu.RUnlock()
	if !ready {
		st.precomputeIdentityNormalization()
		guard.mu.RLock()
		members = st.identityMemberTypes
		guard.mu.RUnlock()
	}
	return members
}
