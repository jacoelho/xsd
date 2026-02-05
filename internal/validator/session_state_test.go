package validator

import (
	"strconv"
	"testing"

	xsdErrors "github.com/jacoelho/xsd/errors"
)

func TestSessionReset(t *testing.T) {
	s := &Session{}
	s.elemStack = []elemFrame{{name: 1}, {name: 2}}
	s.nsStack = []nsFrame{{off: 1, len: 2}}
	s.nameMap = []nameEntry{{LocalOff: 1, LocalLen: 2}}
	s.nameMapSparse = map[NameID]nameEntry{1: {LocalOff: 1, LocalLen: 2}}
	s.nameLocal = []byte("local")
	s.nameNS = []byte("ns")
	s.textBuf = []byte("text")
	s.normBuf = []byte("norm")
	s.errBuf = []byte("err")
	s.validationErrors = []xsdErrors.Validation{{Code: "x"}}
	s.icState.active = true
	s.icState.frames = []rtIdentityFrame{{id: 1}, {id: 2}}
	s.icState.scopes = []rtIdentityScope{{rootID: 1}}
	s.icState.violations = []error{dummyError{}}
	s.icState.pending = []error{dummyError{}}
	s.prefixCache = []prefixEntry{{hash: 1}}
	s.attrSeenTable = []attrSeenEntry{{hash: 1, idx: 1}}

	s.Reset()

	if len(s.elemStack) != 0 {
		t.Fatalf("elemStack len = %d, want 0", len(s.elemStack))
	}
	if len(s.nsStack) != 0 {
		t.Fatalf("nsStack len = %d, want 0", len(s.nsStack))
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
	if len(s.icState.frames) != 0 || len(s.icState.scopes) != 0 {
		t.Fatalf("identity stacks not reset")
	}
	if len(s.icState.violations) != 0 || len(s.icState.pending) != 0 {
		t.Fatalf("identity state results not reset")
	}
	if len(s.prefixCache) != 0 || len(s.attrSeenTable) != 0 {
		t.Fatalf("session caches not reset")
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
	for i := range maxSessionIDTableEntries + 1 {
		s.idTable[strconv.Itoa(i)] = struct{}{}
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
}
