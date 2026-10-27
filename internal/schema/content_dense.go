package schema

import "math"

const (
	denseDFAMinEdges     = 16
	denseNamespaceOffset = 0
	denseMinLocalOffset  = 1
	denseEdgeSlotsOffset = 2
)

// Optional conversion owns no published state or hard content-work allowance.
// Its byte cap counts requested array backing, including both metadata cells.
type denseIndexBudget struct {
	bytes uint64
	work  uint64
}

func defaultDenseIndexBudget() denseIndexBudget {
	return denseIndexBudget{bytes: 4 << 20, work: 16_000_000}
}

func (b *denseIndexBudget) spend(work uint64) bool {
	if work > b.work {
		return false
	}
	b.work -= work
	return true
}

func (b *denseIndexBudget) admit(bytes, work uint64) bool {
	if bytes > b.bytes || work > b.work {
		return false
	}
	b.bytes -= bytes
	b.work -= work
	return true
}

func mayUseDenseDFA(row *compiledModelRowRead) bool {
	return len(row.Edges) >= denseDFAMinEdges && row.indexKind == publishedDFAMap && len(row.index.data) == 0
}

// Conversion follows mandatory publication and visits models/rows in source
// order. Exhaustion stops the phase; all remaining cloned map reads are complete.
func addDenseDFAIndexes(models []compiledModelRead, budget *denseIndexBudget) {
	for i := range models {
		if !budget.spend(1) {
			return
		}
		model := &models[i]
		if model.Kind == CompiledModelDFA && addDenseDFARows(model.Rows, budget) {
			return
		}
	}
}

func addDenseDFARows(rows []compiledModelRowRead, budget *denseIndexBudget) (exhausted bool) {
	for j := range rows {
		if !budget.spend(1) {
			return true
		}
		row := &rows[j]
		if !mayUseDenseDFA(row) {
			continue
		}
		data, exhausted := newDenseDFAData(row, budget)
		if exhausted {
			return true
		}
		if data != nil {
			row.index = publishedDFAIndex{data: data}
			row.indexKind = publishedDFADense
		}
	}
	return false
}

// A nil array without exhaustion means the row is ineligible. No partial array
// or representation change escapes a failed admission.
func newDenseDFAData(row *compiledModelRowRead, budget *denseIndexBudget) (data []uint32, exhausted bool) {
	index := row.index.nameToEdge
	count := len(index)
	if count < 2 || count > 256 {
		return nil, false
	}
	if !budget.spend(uint64(count)) {
		return nil, true
	}
	bounds, valid := scanDenseIndexBounds(index, len(row.Edges))
	span := bounds.span(count)
	if !valid || span == 0 {
		return nil, false
	}
	// Both complete passes have fixed charges independent of map iteration.
	// The final four units zero and fill the two metadata cells.
	if !budget.admit(4*(span+denseEdgeSlotsOffset), 2*uint64(count)+span+4) {
		return nil, true
	}
	data = make([]uint32, span+denseEdgeSlotsOffset)
	data[denseNamespaceOffset] = uint32(bounds.namespace)
	data[denseMinLocalOffset] = uint32(bounds.minLocal)
	for name, pos := range index {
		data[uint64(name.Local)-uint64(bounds.minLocal)+denseEdgeSlotsOffset] = pos + 1
	}
	return data, false
}

func (idx publishedDFAIndex) denseEdge(name QName) int {
	if name.Namespace != NamespaceID(idx.data[denseNamespaceOffset]) || name.Local < LocalNameID(idx.data[denseMinLocalOffset]) {
		return -1
	}
	offset := uint64(name.Local) - uint64(idx.data[denseMinLocalOffset])
	slots := idx.data[denseEdgeSlotsOffset:]
	if offset >= uint64(len(slots)) || slots[offset] == 0 {
		return -1
	}
	return int(slots[offset] - 1)
}

type denseNameBounds struct {
	namespace NamespaceID
	minLocal  LocalNameID
	maxLocal  LocalNameID
	mixed     bool
}

func newDenseNameBounds(name QName) denseNameBounds {
	return denseNameBounds{namespace: name.Namespace, minLocal: name.Local, maxLocal: name.Local}
}

func (b *denseNameBounds) include(name QName) {
	b.mixed = b.mixed || name.Namespace != b.namespace
	b.minLocal = min(b.minLocal, name.Local)
	b.maxLocal = max(b.maxLocal, name.Local)
}

func (b *denseNameBounds) span(count int) uint64 {
	span := uint64(b.maxLocal) - uint64(b.minLocal) + 1
	if b.mixed || count < 2 || span > 256 || span > 2*uint64(count) {
		return 0
	}
	return span
}

func scanDenseIndexBounds(index map[QName]uint32, edgeCount int) (denseNameBounds, bool) {
	if edgeCount < 0 {
		return denseNameBounds{}, false
	}
	var bounds denseNameBounds
	first, valid := true, true
	for name, pos := range index {
		if first {
			bounds = newDenseNameBounds(name)
			first = false
		}
		bounds.include(name)
		valid = valid && pos != math.MaxUint32 && uint64(pos) < uint64(edgeCount)
	}
	return bounds, valid
}
