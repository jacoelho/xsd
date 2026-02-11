package validator

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestWildcardProcessParityElementAndAttribute(t *testing.T) {
	tests := []struct {
		name         string
		pc           runtime.ProcessContents
		withSymbol   bool
		wantElemSkip bool
		wantElemErr  xsderrors.ErrorCode
		wantAttrErr  xsderrors.ErrorCode
	}{
		{
			name:         "strict unresolved",
			pc:           runtime.PCStrict,
			withSymbol:   false,
			wantElemSkip: false,
			wantElemErr:  xsderrors.ErrValidateWildcardElemStrictUnresolved,
			wantAttrErr:  xsderrors.ErrValidateWildcardAttrStrictUnresolved,
		},
		{
			name:       "strict resolved",
			pc:         runtime.PCStrict,
			withSymbol: true,
		},
		{
			name:         "lax unresolved",
			pc:           runtime.PCLax,
			withSymbol:   false,
			wantElemSkip: true,
		},
		{
			name:       "lax resolved",
			pc:         runtime.PCLax,
			withSymbol: true,
		},
		{
			name:         "skip ignores symbol",
			pc:           runtime.PCSkip,
			withSymbol:   true,
			wantElemSkip: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			elemErrCode, elemSkip := runElementWildcardPolicyCase(t, tc.pc, tc.withSymbol)
			if elemErrCode != tc.wantElemErr {
				t.Fatalf("element error code = %v, want %v", elemErrCode, tc.wantElemErr)
			}
			if elemSkip != tc.wantElemSkip {
				t.Fatalf("element skip = %v, want %v", elemSkip, tc.wantElemSkip)
			}

			attrErrCode := runAttributeWildcardPolicyCase(t, tc.pc, tc.withSymbol)
			if attrErrCode != tc.wantAttrErr {
				t.Fatalf("attribute error code = %v, want %v", attrErrCode, tc.wantAttrErr)
			}
		})
	}
}

func runElementWildcardPolicyCase(t *testing.T, pc runtime.ProcessContents, withSymbol bool) (xsderrors.ErrorCode, bool) {
	t.Helper()
	schema, ids := buildRuntimeFixture(t)
	schema.Wildcards = []runtime.WildcardRule{
		{},
		{
			NS: runtime.NSConstraint{Kind: runtime.NSAny},
			PC: pc,
		},
	}
	sess := NewSession(schema)

	sym := runtime.SymbolID(0)
	if withSymbol {
		sym = ids.elemSym
	}
	result, err := sess.StartElement(
		StartMatch{Kind: MatchWildcard, Wildcard: 1},
		sym,
		ids.nsID,
		[]byte("urn:test"),
		nil,
		nil,
	)
	if err == nil {
		return "", result.Skip
	}
	code, _, ok := validationErrorInfo(err)
	if !ok {
		t.Fatalf("element wildcard error is not validation error: %v", err)
	}
	return code, false
}

func runAttributeWildcardPolicyCase(t *testing.T, pc runtime.ProcessContents, withSymbol bool) xsderrors.ErrorCode {
	t.Helper()
	schema, ids := buildAttrFixtureNoRequired(t)
	schema.ComplexTypes[1].AnyAttr = 1
	schema.Wildcards = []runtime.WildcardRule{
		{},
		{
			NS: runtime.NSConstraint{Kind: runtime.NSAny},
			PC: pc,
		},
	}
	sess := NewSession(schema)

	attrSym := runtime.SymbolID(0)
	local := []byte("unknown")
	if withSymbol {
		attrSym = ids.attrSymGlobal
		local = []byte("global")
	}
	attrs := []StartAttr{{
		Sym:     attrSym,
		NS:      ids.nsID,
		NSBytes: []byte("urn:test"),
		Local:   local,
	}}
	_, err := sess.ValidateAttributes(ids.typeBase, attrs, nil)
	if err == nil {
		return ""
	}
	code, _, ok := validationErrorInfo(err)
	if !ok {
		t.Fatalf("attribute wildcard error is not validation error: %v", err)
	}
	return code
}
