package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestIsUnionIntegerLexical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lexical string
		want    bool
	}{
		{name: "plain", lexical: "12", want: true},
		{name: "negative", lexical: "-12", want: true},
		{name: "positive", lexical: "+12", want: true},
		{name: "negative zero", lexical: "-0", want: true},
		{name: "positive zero", lexical: "+0", want: true},
		{name: "zero", lexical: "000", want: true},
		{name: "empty", lexical: "", want: false},
		{name: "sign only", lexical: "-", want: false},
		{name: "decimal", lexical: "12.5", want: false},
		{name: "exp", lexical: "12e3", want: false},
		{name: "space", lexical: " 12 ", want: false},
		{name: "word", lexical: "abc", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isUnionIntegerLexical([]byte(tc.lexical)); got != tc.want {
				t.Fatalf("isUnionIntegerLexical(%q) = %v, want %v", tc.lexical, got, tc.want)
			}
		})
	}
}

func TestMembersOverflowReturnsFalse(t *testing.T) {
	t.Parallel()

	_, _, _, ok := unionMembers(
		runtime.ValidatorMeta{Index: 0},
		runtime.ValidatorsBundle{
			Union: []runtime.UnionValidator{
				{MemberOff: ^uint32(0), MemberLen: 2},
			},
			UnionMembers:      []runtime.ValidatorID{1},
			UnionMemberTypes:  []runtime.TypeID{1},
			UnionMemberSameWS: []uint8{1},
		},
	)
	if ok {
		t.Fatal("unionMembers() ok = true, want false")
	}
}

func TestMatchRejectsImpossibleIntegerWithoutValidation(t *testing.T) {
	t.Parallel()

	calls := 0
	out := matchUnion(
		nil,
		nil,
		[]byte("abc"),
		nil,
		nil,
		runtime.ValidatorsBundle{
			Union: []runtime.UnionValidator{
				{MemberOff: 0, MemberLen: 1},
			},
			UnionMembers:      []runtime.ValidatorID{1},
			UnionMemberTypes:  []runtime.TypeID{7},
			UnionMemberSameWS: []uint8{1},
			Meta: []runtime.ValidatorMeta{
				{},
				{Kind: runtime.VInteger},
			},
		},
		runtime.ValidatorMeta{Index: 0},
		true,
		false,
		func(runtime.ValidatorID, []byte, bool, bool) ([]byte, runtime.ValueKind, []byte, bool, error) {
			calls++
			return nil, 0, nil, false, nil
		},
	)

	if calls != 0 {
		t.Fatalf("validator calls = %d, want 0", calls)
	}
	if out.Matched {
		t.Fatal("matchUnion() matched = true, want false")
	}
	if out.FirstErr == nil {
		t.Fatal("matchUnion() error = nil, want invalid integer")
	}
	if out.FirstErr.Error() != "invalid integer" {
		t.Fatalf("matchUnion() error = %q, want %q", out.FirstErr.Error(), "invalid integer")
	}
}
