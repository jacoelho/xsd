package ic

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestDuplicateDetection(t *testing.T) {
	rows := []Row{
		{Values: [][]byte{[]byte("dup")}},
		{Values: [][]byte{[]byte("dup")}},
	}
	table, dupes := BuildTable(rows)
	if table == nil {
		t.Fatalf("expected table")
	}
	if len(dupes) != 1 || dupes[0] != 1 {
		t.Fatalf("dupes = %v, want [1]", dupes)
	}
	if !table.Contains(rows[0]) {
		t.Fatalf("expected table to contain row 0")
	}
}

func TestKeyrefResolution(t *testing.T) {
	constraints := []Constraint{
		{
			ID:       1,
			Category: runtime.ICKey,
			Rows: []Row{
				{Values: [][]byte{[]byte("a")}},
			},
		},
		{
			ID:         2,
			Category:   runtime.ICKeyRef,
			Referenced: 1,
			Keyrefs: []Row{
				{Values: [][]byte{[]byte("a")}},
				{Values: [][]byte{[]byte("b")}},
			},
		},
	}
	issues := Resolve(constraints)
	if len(issues) != 1 {
		t.Fatalf("issues = %d, want 1", len(issues))
	}
	if issues[0].Kind != IssueKeyrefMissing {
		t.Fatalf("issue kind = %d, want %d", issues[0].Kind, IssueKeyrefMissing)
	}

	issues = Resolve([]Constraint{{
		ID:         3,
		Category:   runtime.ICKeyRef,
		Referenced: 9,
		Keyrefs: []Row{
			{Values: [][]byte{[]byte("x")}},
		},
	}})
	if len(issues) != 1 {
		t.Fatalf("issues = %d, want 1", len(issues))
	}
	if issues[0].Kind != IssueKeyrefUndefined {
		t.Fatalf("issue kind = %d, want %d", issues[0].Kind, IssueKeyrefUndefined)
	}
}
