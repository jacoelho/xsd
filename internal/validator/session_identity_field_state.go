package validator

func (s *rtFieldState) addNode(key rtFieldNodeKey) bool {
	if s.nodes == nil {
		s.nodes = make(map[rtFieldNodeKey]struct{})
	}
	if _, ok := s.nodes[key]; ok {
		return false
	}
	s.nodes[key] = struct{}{}
	s.count++
	if s.count > 1 {
		s.multiple = true
	}
	return true
}
