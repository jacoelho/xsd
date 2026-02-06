package ic

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/runtime"
)

type Row struct {
	Values []runtime.ValueKey
	Hash   uint64
}

type Constraint struct {
	Rows       []Row
	Keyrefs    []Row
	ID         runtime.ICID
	Referenced runtime.ICID
	Category   runtime.ICCategory
}

type IssueKind uint8

const (
	IssueDuplicate IssueKind = iota
	IssueKeyrefMissing
	IssueKeyrefUndefined
)

type Issue struct {
	Kind       IssueKind
	Category   runtime.ICCategory
	Constraint runtime.ICID
	Referenced runtime.ICID
	Row        int
}

type Table struct {
	hashes []uint64
	slots  []uint32
	rows   []Row
}

func Resolve(constraints []Constraint) []Issue {
	if len(constraints) == 0 {
		return nil
	}
	var maxID runtime.ICID
	for _, constraint := range constraints {
		if constraint.ID > maxID {
			maxID = constraint.ID
		}
	}
	if maxID == 0 {
		return nil
	}
	tables := make([]*Table, maxID+1)
	var issues []Issue

	for _, constraint := range constraints {
		if constraint.Category != runtime.ICKey && constraint.Category != runtime.ICUnique {
			continue
		}
		table, dupes := BuildTable(constraint.Rows)
		if table != nil {
			tables[constraint.ID] = table
		}
		for _, row := range dupes {
			issues = append(issues, Issue{
				Kind:       IssueDuplicate,
				Category:   constraint.Category,
				Constraint: constraint.ID,
				Row:        row,
			})
		}
	}

	for _, constraint := range constraints {
		if constraint.Category != runtime.ICKeyRef {
			continue
		}
		ref := constraint.Referenced
		if ref == 0 || int(ref) >= len(tables) || tables[ref] == nil {
			issues = append(issues, Issue{
				Kind:       IssueKeyrefUndefined,
				Category:   constraint.Category,
				Constraint: constraint.ID,
				Referenced: ref,
			})
			continue
		}
		table := tables[ref]
		for idx, row := range constraint.Keyrefs {
			if !table.Contains(row) {
				issues = append(issues, Issue{
					Kind:       IssueKeyrefMissing,
					Category:   constraint.Category,
					Constraint: constraint.ID,
					Referenced: ref,
					Row:        idx,
				})
			}
		}
	}

	return issues
}

func HashRow(values []runtime.ValueKey) uint64 {
	h := uint64(runtime.FNVOffset64)
	for _, value := range values {
		keyHash := value.Hash
		if keyHash == 0 {
			keyHash = runtime.HashKey(value.Kind, value.Bytes)
		}
		for i := 0; i < 8; i++ {
			h ^= uint64(byte(keyHash))
			h *= runtime.FNVPrime64
			keyHash >>= 8
		}
	}
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	if h == 0 {
		return 1
	}
	return h
}

func BuildTable(rows []Row) (*Table, []int) {
	if len(rows) == 0 {
		return nil, nil
	}
	size := runtime.NextPow2(len(rows) * 2)
	table := &Table{
		hashes: make([]uint64, size),
		slots:  make([]uint32, size),
		rows:   rows,
	}
	dupes := make([]int, 0, len(rows)/2)
	mask := uint64(size - 1)
	for i := range rows {
		hash := rows[i].Hash
		if hash == 0 {
			hash = HashRow(rows[i].Values)
		}
		h := hash
		slot := int(h & mask)
		for range size {
			entry := table.slots[slot]
			if entry == 0 {
				table.slots[slot] = uint32(i + 1)
				table.hashes[slot] = h
				break
			}
			if table.hashes[slot] == h {
				other := int(entry - 1)
				if rowsEqual(table.rows[other], rows[i]) {
					dupes = append(dupes, i)
					break
				}
			}
			slot = (slot + 1) & int(mask)
		}
	}
	return table, dupes
}

func (t *Table) Contains(row Row) bool {
	if t == nil || len(t.slots) == 0 {
		return false
	}
	if row.Hash == 0 {
		row.Hash = HashRow(row.Values)
	}
	mask := uint64(len(t.slots) - 1)
	slot := int(row.Hash & mask)
	for probes := 0; probes < len(t.slots); probes++ {
		entry := t.slots[slot]
		if entry == 0 {
			return false
		}
		if t.hashes[slot] == row.Hash {
			other := int(entry - 1)
			if rowsEqual(t.rows[other], row) {
				return true
			}
		}
		slot = (slot + 1) & int(mask)
	}
	return false
}

func rowsEqual(a, b Row) bool {
	if len(a.Values) != len(b.Values) {
		return false
	}
	for i := range a.Values {
		if a.Values[i].Kind != b.Values[i].Kind {
			return false
		}
		if !bytes.Equal(a.Values[i].Bytes, b.Values[i].Bytes) {
			return false
		}
	}
	return true
}
