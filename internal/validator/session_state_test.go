package validator

import "testing"

func TestSessionReset(t *testing.T) {
	s := &Session{}
	s.elemStack = []elemFrame{{name: 1}, {name: 2}}
	s.nsStack = []nsFrame{{off: 1, len: 2}}
	s.nameMap = []nameEntry{{LocalOff: 1, LocalLen: 2}}
	s.nameLocal = []byte("local")
	s.nameNS = []byte("ns")
	s.textBuf = []byte("text")
	s.normBuf = []byte("norm")
	s.errBuf = []byte("err")
	s.icState.active = true
	s.icState.frames = []rtIdentityFrame{{id: 1}, {id: 2}}
	s.icState.scopes = []rtIdentityScope{{rootID: 1}}
	s.icState.violations = []error{dummyError{}}
	s.icState.completed = []rtIdentityScopeResult{{rootElem: 1}}

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
	if len(s.nameLocal) != 0 {
		t.Fatalf("nameLocal len = %d, want 0", len(s.nameLocal))
	}
	if len(s.nameNS) != 0 {
		t.Fatalf("nameNS len = %d, want 0", len(s.nameNS))
	}
	if len(s.textBuf) != 0 || len(s.normBuf) != 0 || len(s.errBuf) != 0 {
		t.Fatalf("expected buffers to be cleared")
	}
	if s.icState.active {
		t.Fatalf("identity state not reset")
	}
	if len(s.icState.frames) != 0 || len(s.icState.scopes) != 0 {
		t.Fatalf("identity stacks not reset")
	}
	if len(s.icState.violations) != 0 || len(s.icState.completed) != 0 {
		t.Fatalf("identity state results not reset")
	}
}

type dummyError struct{}

func (dummyError) Error() string { return "dummy" }
