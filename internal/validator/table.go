package validator

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/runtime"
)

type resolutionConstraint struct {
	Rows       []Row
	Keyrefs    []Row
	ID         runtime.ICID
	Referenced runtime.ICID
	Category   runtime.ICCategory
}

type issueKind uint8

const (
	issueDuplicate issueKind = iota
	issueKeyrefMissing
	issueKeyrefUndefined
)

type resolutionIssue struct {
	Kind       issueKind
	Category   runtime.ICCategory
	Constraint runtime.ICID
	Referenced runtime.ICID
	Row        int
}

type rowTable struct {
	hashes []uint64
	slots  []uint32
	rows   []Row
}

func resolveConstraintIssues(constraints []resolutionConstraint) []resolutionIssue {
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

	tables := make([]*rowTable, maxID+1)
	var issues []resolutionIssue

	for _, constraint := range constraints {
		if constraint.Category != runtime.ICKey && constraint.Category != runtime.ICUnique {
			continue
		}
		table, dupes := buildRowTable(constraint.Rows)
		if table != nil {
			tables[constraint.ID] = table
		}
		for _, row := range dupes {
			issues = append(issues, resolutionIssue{
				Kind:       issueDuplicate,
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
			issues = append(issues, resolutionIssue{
				Kind:       issueKeyrefUndefined,
				Category:   constraint.Category,
				Constraint: constraint.ID,
				Referenced: ref,
			})
			continue
		}
		table := tables[ref]
		for idx, row := range constraint.Keyrefs {
			if !table.contains(row) {
				issues = append(issues, resolutionIssue{
					Kind:       issueKeyrefMissing,
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

func hashRow(values []runtime.ValueKey) uint64 {
	h := uint64(runtime.FNVOffset64)
	for _, value := range values {
		keyHash := value.Hash
		if keyHash == 0 {
			keyHash = runtime.HashKey(value.Kind, value.Bytes)
		}
		for range 8 {
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

func buildRowTable(rows []Row) (*rowTable, []int) {
	if len(rows) == 0 {
		return nil, nil
	}
	size := runtime.NextPow2(len(rows) * 2)
	table := &rowTable{
		hashes: make([]uint64, size),
		slots:  make([]uint32, size),
		rows:   rows,
	}
	dupes := make([]int, 0, len(rows)/2)
	mask := uint64(size - 1)
	for i := range rows {
		hash := rows[i].Hash
		if hash == 0 {
			hash = hashRow(rows[i].Values)
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

func (t *rowTable) contains(row Row) bool {
	if t == nil || len(t.slots) == 0 {
		return false
	}
	if row.Hash == 0 {
		row.Hash = hashRow(row.Values)
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
