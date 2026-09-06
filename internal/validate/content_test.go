package validate

import (
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/xmlstream"
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
		{name: "CDATA", input: DocumentCharacterData{Kind: xmlstream.CharacterDataCDATA}, wantErr: xsderrors.CodeValidationXML},
		{name: "reference whitespace", input: DocumentCharacterData{Kind: xmlstream.CharacterDataReference, Whitespace: true}, wantErr: xsderrors.CodeValidationXML},
		{name: "text", input: DocumentCharacterData{Kind: xmlstream.CharacterDataText}, wantErr: xsderrors.CodeValidationText},
		{name: "whitespace", input: DocumentCharacterData{Kind: xmlstream.CharacterDataText, Whitespace: true}},
		{name: "invalid kind", input: DocumentCharacterData{Kind: xmlstream.CharacterDataInvalid}, wantErr: xsderrors.CodeInternalInvariant},
		{name: "unknown kind", input: DocumentCharacterData{Kind: xmlstream.CharacterDataKind(99)}, wantErr: xsderrors.CodeInternalInvariant},
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

	name := xsdSchema.RuntimeName{Local: "child"}
	tests := []struct {
		name          string
		typ           xsdSchema.TypeID
		simpleContent xsdSchema.SimpleTypeID
		code          xsderrors.Code
	}{
		{name: "simple type", typ: xsdSchema.SimpleRef(0), simpleContent: xsdSchema.NoSimpleType, code: xsderrors.CodeValidationContent},
		{name: "simple content", typ: xsdSchema.ComplexRef(0), simpleContent: 0, code: xsderrors.CodeValidationContent},
		{name: "no model", typ: xsdSchema.ComplexRef(0), simpleContent: xsdSchema.NoSimpleType, code: xsderrors.CodeValidationElement},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := childContentPolicy(tc.typ, tc.simpleContent, xsdSchema.ContentState{}, name); got.code != tc.code {
				t.Fatalf("childContentPolicy() = %+v, want code %q", got, tc.code)
			}
		})
	}
}

func TestContentCompletionRequiredPolicy(t *testing.T) {
	t.Parallel()

	if contentCompletionRequired(false, xsdSchema.ComplexRef(1), xsdSchema.ContentState{}) {
		t.Fatal("contentCompletionRequired(no model) = true")
	}
	if contentCompletionRequired(false, xsdSchema.SimpleRef(1), xsdSchema.ContentState{}) {
		t.Fatal("contentCompletionRequired(simple type) = true")
	}
}
