package validate

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateDocumentCharacterData(t *testing.T) {
	t.Parallel()

	ctx := StartContext{Path: "/", Line: 2, Column: 3}
	tests := []struct {
		name    string
		data    string
		cdata   bool
		wantErr xsderrors.Code
	}{
		{name: "CDATA", data: "x", cdata: true, wantErr: xsderrors.CodeValidationXML},
		{name: "text", data: "x", wantErr: xsderrors.CodeValidationText},
		{name: "whitespace", data: " \n\t"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateDocumentCharacterData([]byte(tc.data), tc.cdata, ctx)
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

	if got := childFramePolicy(true, true); !got.skip || got.issue.valid() {
		t.Fatalf("childFramePolicy(skip) = %+v", got)
	}
	if got := childFramePolicy(false, true); got.skip || got.issue.code != xsderrors.CodeValidationNil {
		t.Fatalf("childFramePolicy(nilled) = %+v", got)
	}

	name := runtime.RuntimeName{Local: "child"}
	tests := []struct {
		name  string
		child runtime.ChildContentInfo
		code  xsderrors.Code
	}{
		{name: "simple type", code: xsderrors.CodeValidationContent},
		{name: "simple content", child: runtime.ChildContentInfo{Complex: true, Simple: true}, code: xsderrors.CodeValidationContent},
		{name: "no model", child: runtime.ChildContentInfo{Complex: true}, code: xsderrors.CodeValidationElement},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := childContentPolicy(tc.child, runtime.ContentState{}, name); got.code != tc.code {
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

func TestValidateNilledContent(t *testing.T) {
	t.Parallel()

	ctx := StartContext{Path: "/root", Line: 2, Column: 3}
	for _, in := range []NilledContentInput{
		{Context: ctx, Nilled: true, HasText: true},
		{Context: ctx, Nilled: true, HasChild: true},
	} {
		expectXSDCode(t, ValidateNilledContent(in), xsderrors.CodeValidationNil)
	}
	for _, in := range []NilledContentInput{
		{Context: ctx, Nilled: true},
		{Context: ctx, HasText: true, HasChild: true},
	} {
		if err := ValidateNilledContent(in); err != nil {
			t.Fatalf("ValidateNilledContent() error = %v", err)
		}
	}
}
