package valruntime

// State captures cached parsed values and derived keys for one validation.
type State struct {
	Result  Result
	Measure Cache
}

// MeasureCache returns the cached parsed-value and facet-measure state.
func (s *State) MeasureCache() *Cache {
	if s == nil {
		return nil
	}
	return &s.Measure
}

// ResultState returns the derived-key and union-selection state.
func (s *State) ResultState() *Result {
	if s == nil {
		return nil
	}
	return &s.Result
}
