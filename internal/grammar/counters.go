package grammar

type groupCounterState struct {
	counts     []int
	remainders []int
	seen       []bool
	checked    []bool
}

func (g *groupCounterState) reset(groupCount int) {
	if groupCount <= 0 {
		g.counts = g.counts[:0]
		g.remainders = g.remainders[:0]
		g.seen = g.seen[:0]
		g.checked = g.checked[:0]
		return
	}
	g.counts = ensureIntSlice(g.counts, groupCount)
	g.remainders = ensureIntSlice(g.remainders, groupCount)
	g.seen = ensureBoolSlice(g.seen, groupCount)
	g.checked = ensureBoolSlice(g.checked, groupCount)
	clear(g.counts)
	clear(g.remainders)
	clear(g.seen)
	clear(g.checked)
}

func ensureIntSlice(buf []int, size int) []int {
	if cap(buf) < size {
		return make([]int, size)
	}
	return buf[:size]
}

func ensureBoolSlice(buf []bool, size int) []bool {
	if cap(buf) < size {
		return make([]bool, size)
	}
	return buf[:size]
}
