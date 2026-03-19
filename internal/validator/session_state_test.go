package validator

import (
	"strconv"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/validator/identity"
	"github.com/jacoelho/xsd/internal/validator/names"
)

func TestSessionReset(t *testing.T) {
	s := &Session{}
	s.elemStack = []elemFrame{{name: 1}, {name: 2}}
	s.Names.Scopes.Push(names.ScopeFrame{Off: 1, Len: 2})
	s.Names.Dense = []names.Entry{{LocalOff: 1, LocalLen: 2}}
	s.Names.Sparse = map[names.ID]names.Entry{1: {LocalOff: 1, LocalLen: 2}}
	s.Names.Local = []byte("local")
	s.Names.NS = []byte("ns")
	s.textBuf = []byte("text")
	s.normBuf = []byte("norm")
	s.errBuf = []byte("err")
	s.validationErrors = []xsderrors.Validation{{Code: "x"}}
	s.icState.Active = true
	s.icState.Frames.Push(identity.RuntimeFrame{ID: 1})
	s.icState.Frames.Push(identity.RuntimeFrame{ID: 2})
	s.icState.Scopes.Push(identity.Scope{RootID: 1})
	s.icState.Uncommitted = []error{dummyError{}}
	s.icState.Committed = []identity.Violation{{Code: "x"}}
	s.Names.PrefixCache = []names.PrefixEntry{{Hash: 1}}
	s.attrState.Seen = []attrs.SeenEntry{{Hash: 1, Index: 1}}
	s.attrState.Classes = []attrs.Class{attrs.ClassOther}
	s.attrState.Present = []bool{true}
	s.attrState.Starts = []attrs.Start{{Local: []byte("raw")}}
	s.attrState.Validated = []attrs.Start{{Local: []byte("validated")}}
	s.identityAttrs.Buckets = map[uint64][]identity.AttrNameID{1: {1}}
	s.identityAttrs.Names = []identity.AttrName{{NS: []byte("urn"), Local: []byte("id")}}

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
	if len(s.textBuf) != 0 || len(s.normBuf) != 0 || len(s.errBuf) != 0 {
		t.Fatalf("expected buffers to be cleared")
	}
	if len(s.validationErrors) != 0 {
		t.Fatalf("expected validation errors to be cleared")
	}
	if s.icState.Active {
		t.Fatalf("identity state not reset")
	}
	if s.icState.Frames.Len() != 0 || s.icState.Scopes.Len() != 0 {
		t.Fatalf("identity stacks not reset")
	}
	if len(s.icState.Uncommitted) != 0 || len(s.icState.Committed) != 0 {
		t.Fatalf("identity state results not reset")
	}
	if len(s.Names.PrefixCache) != 0 || len(s.attrState.Seen) != 0 {
		t.Fatalf("session caches not reset")
	}
	if len(s.attrState.Classes) != 0 {
		t.Fatalf("attrState.Classes len = %d, want 0", len(s.attrState.Classes))
	}
	if len(s.attrState.Present) != 0 || len(s.attrState.Starts) != 0 || len(s.attrState.Validated) != 0 {
		t.Fatalf("attribute tracker buffers not reset")
	}
	if len(s.identityAttrs.Buckets) != 0 || len(s.identityAttrs.Names) != 0 {
		t.Fatalf("identity attr interner not reset")
	}
}

type dummyError struct{}

func (dummyError) Error() string { return "dummy" }

func TestSessionResetShrinksOversizedBuffers(t *testing.T) {
	s := &Session{}
	s.Names.Local = make([]byte, maxSessionBuffer+1)
	s.elemStack = make([]elemFrame, maxSessionEntries+1)
	s.attrState.Present = make([]bool, maxSessionEntries+1)
	s.attrState.Starts = make([]attrs.Start, maxSessionEntries+1)
	s.attrState.Validated = make([]attrs.Start, maxSessionEntries+1)
	s.attrState.Classes = make([]attrs.Class, maxSessionEntries+1)
	s.idTable = make(map[string]struct{}, maxSessionIDTableEntries+1)
	s.identityAttrs.Names = make([]identity.AttrName, maxSessionEntries+1)
	s.identityAttrs.Buckets = make(map[uint64][]identity.AttrNameID, maxSessionEntries+1)
	for i := range maxSessionIDTableEntries + 1 {
		s.idTable[strconv.Itoa(i)] = struct{}{}
	}
	for i := range maxSessionEntries + 1 {
		s.identityAttrs.Buckets[uint64(i)] = []identity.AttrNameID{identity.AttrNameID(i + 1)}
	}

	s.Reset()

	if s.Names.Local != nil {
		t.Fatalf("expected name local buffer to be shrunk")
	}
	if s.elemStack != nil {
		t.Fatalf("expected elemStack to be shrunk")
	}
	if s.attrState.Present != nil {
		t.Fatalf("expected attrState.Present to be shrunk")
	}
	if s.attrState.Starts != nil {
		t.Fatalf("expected attrState.Starts to be shrunk")
	}
	if s.attrState.Validated != nil {
		t.Fatalf("expected attrState.Validated to be shrunk")
	}
	if s.attrState.Classes != nil {
		t.Fatalf("expected attrState.Classes to be shrunk")
	}
	if s.idTable != nil {
		t.Fatalf("expected idTable to be dropped")
	}
	if s.identityAttrs.Names != nil {
		t.Fatalf("expected identityAttrs.Names to be shrunk")
	}
	if s.identityAttrs.Buckets != nil {
		t.Fatalf("expected identityAttrs.Buckets to be dropped")
	}
}

func TestSessionResetDropsOversizedStacks(t *testing.T) {
	s := &Session{}
	for range maxSessionEntries + 1 {
		s.Names.Scopes.Push(names.ScopeFrame{})
		s.icState.Frames.Push(identity.RuntimeFrame{})
		s.icState.Scopes.Push(identity.Scope{})
	}

	if s.Names.Scopes.Cap() <= maxSessionEntries {
		t.Fatalf("test setup failed: namespace scope cap = %d, want > %d", s.Names.Scopes.Cap(), maxSessionEntries)
	}
	if s.icState.Frames.Cap() <= maxSessionEntries {
		t.Fatalf("test setup failed: frames cap = %d, want > %d", s.icState.Frames.Cap(), maxSessionEntries)
	}
	if s.icState.Scopes.Cap() <= maxSessionEntries {
		t.Fatalf("test setup failed: scopes cap = %d, want > %d", s.icState.Scopes.Cap(), maxSessionEntries)
	}

	s.Reset()

	if s.Names.Scopes.Cap() != 0 {
		t.Fatalf("namespace scope cap = %d, want 0", s.Names.Scopes.Cap())
	}
	if s.icState.Frames.Cap() != 0 {
		t.Fatalf("frames cap = %d, want 0", s.icState.Frames.Cap())
	}
	if s.icState.Scopes.Cap() != 0 {
		t.Fatalf("scopes cap = %d, want 0", s.icState.Scopes.Cap())
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
