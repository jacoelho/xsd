package validator

import (
	"strconv"
	"testing"

	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func TestSessionReset(t *testing.T) {
	s := &Session{}
	s.elemStack = []elemFrame{{name: 1}, {name: 2}}
	s.Names.Scopes.Push(NamespaceScopeFrame{Off: 1, Len: 2})
	s.Names.Dense = []NameEntry{{LocalOff: 1, LocalLen: 2}}
	s.Names.Sparse = map[NameID]NameEntry{1: {LocalOff: 1, LocalLen: 2}}
	s.Names.Local = []byte("local")
	s.Names.NS = []byte("ns")
	s.buffers.textBuf = []byte("text")
	s.buffers.normBuf = []byte("norm")
	s.buffers.errBuf = []byte("err")
	s.validationErrors = []xsderrors.Validation{{Code: "x"}}
	s.identity.icState.Active = true
	s.identity.icState.Frames.Push(RuntimeFrame{ID: 1})
	s.identity.icState.Frames.Push(RuntimeFrame{ID: 2})
	s.identity.icState.Scopes.Push(Scope{RootID: 1})
	s.identity.icState.Uncommitted = []error{dummyError{}}
	s.identity.icState.Committed = []Violation{{Code: "x"}}
	s.Names.PrefixCache = []prefixCacheEntry{{Hash: 1}}
	s.attrs.attrState.Seen = []SeenEntry{{Hash: 1, Index: 1}}
	s.attrs.attrState.Classes = []Class{ClassOther}
	s.attrs.attrState.Present = []bool{true}
	s.attrs.attrState.Starts = []Start{{Local: []byte("raw")}}
	s.attrs.attrState.Validated = []Start{{Local: []byte("validated")}}
	s.identity.identityAttrs.Buckets = map[uint64][]AttrNameID{1: {1}}
	s.identity.identityAttrs.Names = []AttrName{{NS: []byte("urn"), Local: []byte("id")}}

	s.Reset()

	if len(s.elemStack) != 0 {
		t.Fatalf("elemStack len = %d, want 0", len(s.elemStack))
	}
	if s.Names.Scopes.Len() != 0 {
		t.Fatalf("namespace scope depth = %d, want 0", s.Names.Scopes.Len())
	}
	if len(s.Names.Dense) != 0 {
		t.Fatalf("dense name map len = %d, want 0", len(s.Names.Dense))
	}
	if len(s.Names.Sparse) != 0 {
		t.Fatalf("sparse name map len = %d, want 0", len(s.Names.Sparse))
	}
	if len(s.Names.Local) != 0 {
		t.Fatalf("name local buffer len = %d, want 0", len(s.Names.Local))
	}
	if len(s.Names.NS) != 0 {
		t.Fatalf("name namespace buffer len = %d, want 0", len(s.Names.NS))
	}
	if len(s.buffers.textBuf) != 0 || len(s.buffers.normBuf) != 0 || len(s.buffers.errBuf) != 0 {
		t.Fatalf("expected buffers to be cleared")
	}
	if len(s.validationErrors) != 0 {
		t.Fatalf("expected validation errors to be cleared")
	}
	if s.identity.icState.Active {
		t.Fatalf("identity state not reset")
	}
	if s.identity.icState.Frames.Len() != 0 || s.identity.icState.Scopes.Len() != 0 {
		t.Fatalf("identity stacks not reset")
	}
	if len(s.identity.icState.Uncommitted) != 0 || len(s.identity.icState.Committed) != 0 {
		t.Fatalf("identity state results not reset")
	}
	if len(s.Names.PrefixCache) != 0 || len(s.attrs.attrState.Seen) != 0 {
		t.Fatalf("session caches not reset")
	}
	if len(s.attrs.attrState.Classes) != 0 {
		t.Fatalf("attrState.Classes len = %d, want 0", len(s.attrs.attrState.Classes))
	}
	if len(s.attrs.attrState.Present) != 0 || len(s.attrs.attrState.Starts) != 0 || len(s.attrs.attrState.Validated) != 0 {
		t.Fatalf("attribute tracker buffers not reset")
	}
	if len(s.identity.identityAttrs.Buckets) != 0 || len(s.identity.identityAttrs.Names) != 0 {
		t.Fatalf("identity attr interner not reset")
	}
}

type dummyError struct{}

func (dummyError) Error() string { return "dummy" }

func TestSessionResetShrinksOversizedBuffers(t *testing.T) {
	s := &Session{}
	s.Names.Local = make([]byte, maxSessionBuffer+1)
	s.elemStack = make([]elemFrame, maxSessionEntries+1)
	s.attrs.attrState.Present = make([]bool, maxSessionEntries+1)
	s.attrs.attrState.Starts = make([]Start, maxSessionEntries+1)
	s.attrs.attrState.Validated = make([]Start, maxSessionEntries+1)
	s.attrs.attrState.Classes = make([]Class, maxSessionEntries+1)
	s.identity.idTable = make(map[string]struct{}, maxSessionIDTableEntries+1)
	s.identity.identityAttrs.Names = make([]AttrName, maxSessionEntries+1)
	s.identity.identityAttrs.Buckets = make(map[uint64][]AttrNameID, maxSessionEntries+1)
	for i := range maxSessionIDTableEntries + 1 {
		s.identity.idTable[strconv.Itoa(i)] = struct{}{}
	}
	for i := range maxSessionEntries + 1 {
		s.identity.identityAttrs.Buckets[uint64(i)] = []AttrNameID{AttrNameID(i + 1)}
	}

	s.Reset()

	if s.Names.Local != nil {
		t.Fatalf("expected name local buffer to be shrunk")
	}
	if s.elemStack != nil {
		t.Fatalf("expected elemStack to be shrunk")
	}
	if s.attrs.attrState.Present != nil {
		t.Fatalf("expected attrState.Present to be shrunk")
	}
	if s.attrs.attrState.Starts != nil {
		t.Fatalf("expected attrState.Starts to be shrunk")
	}
	if s.attrs.attrState.Validated != nil {
		t.Fatalf("expected attrState.Validated to be shrunk")
	}
	if s.attrs.attrState.Classes != nil {
		t.Fatalf("expected attrState.Classes to be shrunk")
	}
	if s.identity.idTable != nil {
		t.Fatalf("expected idTable to be dropped")
	}
	if s.identity.identityAttrs.Names != nil {
		t.Fatalf("expected identityAttrs.Names to be shrunk")
	}
	if s.identity.identityAttrs.Buckets != nil {
		t.Fatalf("expected identityAttrs.Buckets to be dropped")
	}
}

func TestSessionResetDropsOversizedStacks(t *testing.T) {
	s := &Session{}
	for range maxSessionEntries + 1 {
		s.Names.Scopes.Push(NamespaceScopeFrame{})
		s.identity.icState.Frames.Push(RuntimeFrame{})
		s.identity.icState.Scopes.Push(Scope{})
	}

	if s.Names.Scopes.Cap() <= maxSessionEntries {
		t.Fatalf("test setup failed: namespace scope cap = %d, want > %d", s.Names.Scopes.Cap(), maxSessionEntries)
	}
	if s.identity.icState.Frames.Cap() <= maxSessionEntries {
		t.Fatalf("test setup failed: frames cap = %d, want > %d", s.identity.icState.Frames.Cap(), maxSessionEntries)
	}
	if s.identity.icState.Scopes.Cap() <= maxSessionEntries {
		t.Fatalf("test setup failed: scopes cap = %d, want > %d", s.identity.icState.Scopes.Cap(), maxSessionEntries)
	}

	s.Reset()

	if s.Names.Scopes.Cap() != 0 {
		t.Fatalf("namespace scope cap = %d, want 0", s.Names.Scopes.Cap())
	}
	if s.identity.icState.Frames.Cap() != 0 {
		t.Fatalf("frames cap = %d, want 0", s.identity.icState.Frames.Cap())
	}
	if s.identity.icState.Scopes.Cap() != 0 {
		t.Fatalf("scopes cap = %d, want 0", s.identity.icState.Scopes.Cap())
	}
}

func TestShrinkNormStackMixedCapBehavior(t *testing.T) {
	small := make([]byte, 2, 8)
	large := make([]byte, 2, maxSessionBuffer+1)
	stack := [][]byte{small, large}

	got := shrinkNormStack(stack, maxSessionBuffer, maxSessionEntries)
	if len(got) != 2 {
		t.Fatalf("normStack len = %d, want 2", len(got))
	}
	if got[0] == nil {
		t.Fatalf("small entry unexpectedly dropped")
	}
	if len(got[0]) != 0 {
		t.Fatalf("small entry len = %d, want 0", len(got[0]))
	}
	if cap(got[0]) != cap(small) {
		t.Fatalf("small entry cap = %d, want %d", cap(got[0]), cap(small))
	}
	if got[1] != nil {
		t.Fatalf("large entry not dropped")
	}
}

func TestShrinkNormStackDropsOversizedEntries(t *testing.T) {
	stack := make([][]byte, maxSessionEntries+1)
	got := shrinkNormStack(stack, maxSessionBuffer, maxSessionEntries)
	if got != nil {
		t.Fatalf("expected oversized normStack to be dropped")
	}
}
