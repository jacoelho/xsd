package ic

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/runtime"
)

const (
	hashOffset64 = 14695981039346656037
	hashPrime64  = 1099511628211
)

type Row struct {
	Values []Key
	Hash   uint64
}

type Key struct {
	Bytes []byte
	Kind  runtime.ValueKind
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

func HashRow(values []Key) uint64 {
	h := uint64(hashOffset64)
	for _, value := range values {
		h ^= uint64(value.Kind)
		h *= hashPrime64
		length := uint32(len(value.Bytes))
		for range 4 {
			h ^= uint64(byte(length))
			h *= hashPrime64
			length >>= 8
		}
		for _, c := range value.Bytes {
			h ^= uint64(c)
			h *= hashPrime64
		}
	}
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
		if rows[i].Hash == 0 {
			rows[i].Hash = HashRow(rows[i].Values)
		}
		h := rows[i].Hash
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
