package errors

import "testing"

type testIssue struct {
	code ErrorCode
	msg  string
}

func (i testIssue) ValidationCode() ErrorCode {
	return i.code
}

func (i testIssue) ValidationMessage() string {
	return i.msg
}

func TestAppendIssues(t *testing.T) {
	got := AppendIssues(nil, []testIssue{
		{code: ErrIdentityAbsent, msg: "missing"},
		{},
		{code: ErrIdentityDuplicate, msg: "duplicate"},
	})

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if code, ok := Info(got[0]); !ok || code != ErrIdentityAbsent {
		t.Fatalf("got[0] code = %v, ok=%v", code, ok)
	}
	if code, ok := Info(got[1]); !ok || code != ErrIdentityDuplicate {
		t.Fatalf("got[1] code = %v, ok=%v", code, ok)
	}
}
