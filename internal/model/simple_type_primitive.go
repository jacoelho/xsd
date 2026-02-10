package model

// PrimitiveType returns the ultimate primitive base type for this simple type.
func (s *SimpleType) PrimitiveType() Type {
	// return cached value if available
	if s == nil {
		return nil
	}
	guard := s.guard()
	guard.mu.Lock()
	for s.primitiveType == nil && s.primitiveTypeComputing {
		guard.cond.Wait()
	}
	if s.primitiveType != nil {
		cached := s.primitiveType
		guard.mu.Unlock()
		return cached
	}
	s.primitiveTypeComputing = true
	guard.mu.Unlock()

	computed := s.computePrimitiveType(make(map[*SimpleType]bool))
	if computed == nil {
		guard.mu.Lock()
		s.primitiveTypeComputing = false
		guard.cond.Broadcast()
		guard.mu.Unlock()
		return nil
	}

	guard.mu.Lock()
	if s.primitiveType == nil {
		s.primitiveType = computed
	}
	s.primitiveTypeComputing = false
	cached := s.primitiveType
	guard.cond.Broadcast()
	guard.mu.Unlock()
	return cached
}

// isQNameOrNotationType reports whether this type derives from QName or NOTATION.
func (s *SimpleType) IsQNameOrNotationType() bool {
	if s == nil {
		return false
	}
	guard := s.guard()
	guard.mu.RLock()
	ready := s.qnameOrNotationReady
	result := s.qnameOrNotation
	guard.mu.RUnlock()
	if ready {
		return result
	}
	computed := s.computeQNameOrNotationType()
	guard.mu.Lock()
	if !s.qnameOrNotationReady {
		s.qnameOrNotation = computed
		s.qnameOrNotationReady = true
	}
	result = s.qnameOrNotation
	guard.mu.Unlock()
	return result
}

// SetQNameOrNotationType stores the precomputed QName/NOTATION derivation flag.
func (s *SimpleType) SetQNameOrNotationType(flag bool) {
	if s == nil {
		return
	}
	guard := s.guard()
	guard.mu.Lock()
	defer guard.mu.Unlock()
	s.qnameOrNotation = flag
	s.qnameOrNotationReady = true
}

func (s *SimpleType) computeQNameOrNotationType() bool {
	if s == nil || s.Variety() == ListVariety {
		return false
	}
	if IsQNameOrNotation(s.QName) {
		return true
	}
	if s.Restriction != nil && !s.Restriction.Base.IsZero() {
		base := s.Restriction.Base
		if (base.Namespace == XSDNamespace || base.Namespace == "") &&
			(base.Local == string(TypeNameQName) || base.Local == string(TypeNameNOTATION)) {
			return true
		}
	}
	switch base := s.ResolvedBase.(type) {
	case *SimpleType:
		if base.IsQNameOrNotationType() {
			return true
		}
	case *BuiltinType:
		if IsQNameOrNotation(base.Name()) {
			return true
		}
	}
	if primitive := s.PrimitiveType(); primitive != nil && IsQNameOrNotation(primitive.Name()) {
		return true
	}
	return false
}

// computePrimitiveType is the internal implementation with cycle detection.
func (s *SimpleType) computePrimitiveType(visited map[*SimpleType]bool) Type {
	// if already computed, return it
	guard := s.guard()
	guard.mu.RLock()
	cached := s.primitiveType
	guard.mu.RUnlock()
	if cached != nil {
		return cached
	}

	if visited[s] {
		// circular reference detected - return nil to break the cycle
		return nil
	}
	visited[s] = true
	defer delete(visited, s)

	if primitive := s.primitiveFromSelf(); primitive != nil {
		return primitive
	}

	if primitive := s.primitiveFromRestriction(visited); primitive != nil {
		return primitive
	}

	if primitive := s.primitiveFromList(visited); primitive != nil {
		return primitive
	}

	if primitive := s.primitiveFromUnion(visited); primitive != nil {
		return primitive
	}

	return nil
}

func (s *SimpleType) primitiveFromSelf() Type {
	if s.builtin && s.QName.Namespace == XSDNamespace && s.Variety() == AtomicVariety {
		if builtin := GetBuiltin(TypeName(s.QName.Local)); builtin != nil {
			return builtin.PrimitiveType()
		}
	}
	return nil
}

func (s *SimpleType) primitiveFromRestriction(visited map[*SimpleType]bool) Type {
	if s.Restriction == nil {
		return nil
	}
	if s.ResolvedBase != nil {
		return primitiveFromBaseType(s.ResolvedBase, visited)
	}
	if s.Restriction.Base.IsZero() {
		return nil
	}
	if s.Restriction.Base.Namespace != XSDNamespace {
		return nil
	}
	builtinType := GetBuiltin(TypeName(s.Restriction.Base.Local))
	if builtinType == nil {
		return nil
	}
	return builtinType.PrimitiveType()
}

func (s *SimpleType) primitiveFromList(visited map[*SimpleType]bool) Type {
	if s.List == nil {
		return nil
	}
	if s.ItemType != nil {
		return primitiveFromBaseType(s.ItemType, visited)
	}
	if !s.List.ItemType.IsZero() {
		if builtin := GetBuiltinNS(s.List.ItemType.Namespace, s.List.ItemType.Local); builtin != nil {
			return builtin.PrimitiveType()
		}
	}
	return nil
}

func (s *SimpleType) primitiveFromUnion(visited map[*SimpleType]bool) Type {
	if s.Union == nil {
		return nil
	}
	var commonPrimitive Type
	for _, memberType := range s.MemberTypes {
		memberPrimitive := primitiveFromBaseType(memberType, visited)
		if memberPrimitive == nil {
			continue
		}
		if commonPrimitive == nil {
			commonPrimitive = memberPrimitive
			continue
		}
		if commonPrimitive != memberPrimitive {
			return nil
		}
	}
	return commonPrimitive
}

func primitiveFromBaseType(base Type, visited map[*SimpleType]bool) Type {
	switch typed := base.(type) {
	case *SimpleType:
		return typed.computePrimitiveType(visited)
	case *BuiltinType:
		return typed.PrimitiveType()
	default:
		return nil
	}
}
