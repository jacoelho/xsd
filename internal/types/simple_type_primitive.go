package types

// PrimitiveType returns the ultimate primitive base type for this simple type.
func (s *SimpleType) PrimitiveType() Type {
	// return cached value if available
	if s == nil {
		return nil
	}
	typeCacheMu.Lock()
	for s.primitiveType == nil && s.primitiveTypeComputing {
		typeCacheCond.Wait()
	}
	if s.primitiveType != nil {
		cached := s.primitiveType
		typeCacheMu.Unlock()
		return cached
	}
	s.primitiveTypeComputing = true
	typeCacheMu.Unlock()

	computed := s.computePrimitiveType(make(map[*SimpleType]bool))
	if computed == nil {
		typeCacheMu.Lock()
		s.primitiveTypeComputing = false
		typeCacheCond.Broadcast()
		typeCacheMu.Unlock()
		return nil
	}

	typeCacheMu.Lock()
	if s.primitiveType == nil {
		s.primitiveType = computed
	}
	s.primitiveTypeComputing = false
	cached := s.primitiveType
	typeCacheCond.Broadcast()
	typeCacheMu.Unlock()
	return cached
}

func (s *SimpleType) precomputeCaches() {
	if s == nil {
		return
	}
	_ = s.PrimitiveType()
	_ = s.FundamentalFacets()
	s.precomputeIdentityNormalization()
}

// IsQNameOrNotationType reports whether this type derives from QName or NOTATION.
func (s *SimpleType) IsQNameOrNotationType() bool {
	if s == nil {
		return false
	}
	typeCacheMu.RLock()
	ready := s.qnameOrNotationReady
	result := s.qnameOrNotation
	typeCacheMu.RUnlock()
	if ready {
		return result
	}
	computed := s.computeQNameOrNotationType()
	typeCacheMu.Lock()
	if !s.qnameOrNotationReady {
		s.qnameOrNotation = computed
		s.qnameOrNotationReady = true
	}
	result = s.qnameOrNotation
	typeCacheMu.Unlock()
	return result
}

// SetQNameOrNotationType stores the precomputed QName/NOTATION derivation flag.
func (s *SimpleType) SetQNameOrNotationType(flag bool) {
	if s == nil {
		return
	}
	typeCacheMu.Lock()
	defer typeCacheMu.Unlock()
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
		if (base.Namespace == XSDNamespace || base.Namespace.IsEmpty()) &&
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
		if base.IsQNameOrNotationType() {
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
	typeCacheMu.RLock()
	cached := s.primitiveType
	typeCacheMu.RUnlock()
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
