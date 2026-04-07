package validator

// ValueMetrics carries per-validation result metadata and cached measurements.
type ValueMetrics struct {
	State ValueState
	Cache ValueCache
}

func (state *ValueMetrics) cache() *ValueCache {
	if state == nil {
		return nil
	}
	return &state.Cache
}

func (state *ValueMetrics) result() *ValueState {
	if state == nil {
		return nil
	}
	return &state.State
}
