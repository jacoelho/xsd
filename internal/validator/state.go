package validator

import "github.com/jacoelho/xsd/internal/runtime"

// FieldNodeKind identifies the runtime node kind captured by one identity field.
type FieldNodeKind int

const (
	FieldNodeElement FieldNodeKind = iota
	FieldNodeAttribute
)

// FieldNodeKey identifies one selected runtime node within a selector match.
type FieldNodeKey struct {
	Kind       FieldNodeKind
	ElemID     uint64
	AttrSym    runtime.SymbolID
	AttrNameID AttrNameID
}

// FieldCapture records one deferred element-value capture for a selector match.
type FieldCapture struct {
	Match      *Match
	FieldIndex int
}

// FieldState stores the current runtime state for one selected field.
type FieldState struct {
	FirstNode FieldNodeKey
	KeyBytes  []byte
	Count     int
	KeyKind   runtime.ValueKind
	HasNode   bool
	Multiple  bool
	Missing   bool
	Invalid   bool
	HasValue  bool
}

// AddNode records one matched node and reports whether it was newly added.
func (s *FieldState) AddNode(key FieldNodeKey) bool {
	if !s.HasNode {
		s.FirstNode = key
		s.HasNode = true
		s.Count = 1
		return true
	}
	if s.FirstNode == key {
		return false
	}
	s.Count = 2
	s.Multiple = true
	return true
}

// Match stores the active state for one matched selector instance.
type Match struct {
	Constraint *ConstraintState
	Fields     []FieldState
	ID         uint64
	Depth      int
	Invalid    bool
	fields     [1]FieldState
}

// Row stores one finalized identity-constraint row.
type Row struct {
	Values []runtime.ValueKey
	Hash   uint64
}

// ConstraintState stores the active runtime state for one identity constraint.
type ConstraintState struct {
	Matches    map[uint64]*Match
	Name       string
	Selectors  []runtime.PathID
	Fields     [][]runtime.PathID
	Rows       []Row
	KeyrefRows []Row
	Violations []Violation
	rowValues  []runtime.ValueKey
	ID         runtime.ICID
	Referenced runtime.ICID
	Category   runtime.ICCategory
}

// Scope stores all constraint state rooted at one scoped element.
type Scope struct {
	Constraints []ConstraintState
	RootID      uint64
	RootDepth   int
	RootElem    runtime.ElemID
}
