package validator

import (
	"strconv"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
)

func TestSessionReset(t *testing.T) {
	s := &Session{}
	s.elemStack = []elemFrame{{name: 1}, {name: 2}}
	s.nsStack.Push(nsFrame{off: 1, len: 2})
	s.nameMap = []nameEntry{{LocalOff: 1, LocalLen: 2}}
	s.nameMapSparse = map[NameID]nameEntry{1: {LocalOff: 1, LocalLen: 2}}
	s.nameLocal = []byte("local")
	s.nameNS = []byte("ns")
	s.textBuf = []byte("text")
	s.normBuf = []byte("norm")
	s.errBuf = []byte("err")
	s.validationErrors = []xsderrors.Validation{{Code: "x"}}
	s.icState.active = true
	s.icState.frames.Push(rtIdentityFrame{id: 1})
	s.icState.frames.Push(rtIdentityFrame{id: 2})
	s.icState.scopes.Push(rtIdentityScope{rootID: 1})
	s.icState.uncommittedViolations = []error{dummyError{}}
	s.icState.committedViolations = []error{dummyError{}}
	s.prefixCache = []prefixEntry{{hash: 1}}
	s.attrSeenTable = []attrSeenEntry{{hash: 1, idx: 1}}
	s.identityAttrBuckets = map[uint64][]identityAttrNameID{1: {1}}
	s.identityAttrNames = []identityAttrName{{ns: []byte("urn"), local: []byte("id")}}

	s.Reset()

	if len(s.elemStack) != 0 {
		t.Fatalf("elemStack len = %d, want 0", len(s.elemStack))
	}
	if s.nsStack.Len() != 0 {
		t.Fatalf("nsStack len = %d, want 0", s.nsStack.Len())
	}
	if len(s.nameMap) != 0 {
		t.Fatalf("nameMap len = %d, want 0", len(s.nameMap))
	}
	if len(s.nameMapSparse) != 0 {
		t.Fatalf("nameMapSparse len = %d, want 0", len(s.nameMapSparse))
	}
	if len(s.nameLocal) != 0 {
		t.Fatalf("nameLocal len = %d, want 0", len(s.nameLocal))
	}
	if len(s.nameNS) != 0 {
		t.Fatalf("nameNS len = %d, want 0", len(s.nameNS))
	}
	if len(s.textBuf) != 0 || len(s.normBuf) != 0 || len(s.errBuf) != 0 {
		t.Fatalf("expected buffers to be cleared")
	}
	if len(s.validationErrors) != 0 {
		t.Fatalf("expected validation errors to be cleared")
	}
	if s.icState.active {
		t.Fatalf("identity state not reset")
	}
	if s.icState.frames.Len() != 0 || s.icState.scopes.Len() != 0 {
		t.Fatalf("identity stacks not reset")
	}
	if len(s.icState.uncommittedViolations) != 0 || len(s.icState.committedViolations) != 0 {
		t.Fatalf("identity state results not reset")
	}
	if len(s.prefixCache) != 0 || len(s.attrSeenTable) != 0 {
		t.Fatalf("session caches not reset")
	}
	if len(s.identityAttrBuckets) != 0 || len(s.identityAttrNames) != 0 {
		t.Fatalf("identity attr interner not reset")
	}
}

type dummyError struct{}

func (dummyError) Error() string { return "dummy" }

func TestSessionResetShrinksOversizedBuffers(t *testing.T) {
	s := &Session{}
	s.nameLocal = make([]byte, maxSessionBuffer+1)
	s.elemStack = make([]elemFrame, maxSessionEntries+1)
	s.attrPresent = make([]bool, maxSessionEntries+1)
	s.idTable = make(map[string]struct{}, maxSessionIDTableEntries+1)
	s.identityAttrNames = make([]identityAttrName, maxSessionEntries+1)
	s.identityAttrBuckets = make(map[uint64][]identityAttrNameID, maxSessionEntries+1)
	for i := range maxSessionIDTableEntries + 1 {
		s.idTable[strconv.Itoa(i)] = struct{}{}
	}
	for i := range maxSessionEntries + 1 {
		s.identityAttrBuckets[uint64(i)] = []identityAttrNameID{identityAttrNameID(i + 1)}
	}

	s.Reset()

	if s.nameLocal != nil {
		t.Fatalf("expected nameLocal to be shrunk")
	}
	if s.elemStack != nil {
		t.Fatalf("expected elemStack to be shrunk")
	}
	if s.attrPresent != nil {
		t.Fatalf("expected attrPresent to be shrunk")
	}
	if s.idTable != nil {
		t.Fatalf("expected idTable to be dropped")
	}
	if s.identityAttrNames != nil {
		t.Fatalf("expected identityAttrNames to be shrunk")
	}
	if s.identityAttrBuckets != nil {
		t.Fatalf("expected identityAttrBuckets to be dropped")
	}
}
