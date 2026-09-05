package validate

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateDocumentCharacterData(t *testing.T) {
	t.Parallel()

	ctx := StartContext{Path: "/", Line: 2, Column: 3}
	tests := []struct {
		name    string
		input   DocumentCharacterData
		wantErr xsderrors.Code
	}{
		{name: "CDATA", input: DocumentCharacterData{Kind: stream.CharacterDataCDATA}, wantErr: xsderrors.CodeValidationXML},
		{name: "reference whitespace", input: DocumentCharacterData{Kind: stream.CharacterDataReference, Whitespace: true}, wantErr: xsderrors.CodeValidationXML},
		{name: "text", input: DocumentCharacterData{Kind: stream.CharacterDataText}, wantErr: xsderrors.CodeValidationText},
		{name: "whitespace", input: DocumentCharacterData{Kind: stream.CharacterDataText, Whitespace: true}},
		{name: "invalid kind", input: DocumentCharacterData{Kind: stream.CharacterDataInvalid}, wantErr: xsderrors.CodeInternalInvariant},
		{name: "unknown kind", input: DocumentCharacterData{Kind: stream.CharacterDataKind(99)}, wantErr: xsderrors.CodeInternalInvariant},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateDocumentCharacterData(tc.input, ctx)
			if tc.wantErr != "" {
				expectXSDCode(t, err, tc.wantErr)
				return
			}
			if err != nil {
				t.Fatalf("ValidateDocumentCharacterData() error = %v", err)
			}
		})
	}
}

func TestChildPolicies(t *testing.T) {
	t.Parallel()

	if got := childFramePolicy(&frame{Nilled: true}); got.issue.code != xsderrors.CodeValidationNil {
		t.Fatalf("childFramePolicy(nilled) = %+v", got)
	}

	name := runtime.RuntimeName{Local: "child"}
	tests := []struct {
		name          string
		typ           runtime.TypeID
		simpleContent runtime.SimpleTypeID
		code          xsderrors.Code
	}{
		{name: "simple type", typ: runtime.SimpleRef(0), simpleContent: runtime.NoSimpleType, code: xsderrors.CodeValidationContent},
		{name: "simple content", typ: runtime.ComplexRef(0), simpleContent: 0, code: xsderrors.CodeValidationContent},
		{name: "no model", typ: runtime.ComplexRef(0), simpleContent: runtime.NoSimpleType, code: xsderrors.CodeValidationElement},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := childContentPolicy(tc.typ, tc.simpleContent, runtime.ContentState{}, name); got.code != tc.code {
				t.Fatalf("childContentPolicy() = %+v, want code %q", got, tc.code)
			}
		})
	}
}

func TestContentCompletionRequiredPolicy(t *testing.T) {
	t.Parallel()

	if contentCompletionRequired(false, runtime.ComplexRef(1), runtime.ContentState{}) {
		t.Fatal("contentCompletionRequired(no model) = true")
	}
	if contentCompletionRequired(false, runtime.SimpleRef(1), runtime.ContentState{}) {
		t.Fatal("contentCompletionRequired(simple type) = true")
	}
}
