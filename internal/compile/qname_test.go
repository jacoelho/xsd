package compile

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseQNameParts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lexical string
		want    QNameParts
		wantMsg string
	}{
		{
			name:    "unprefixed",
			lexical: " name ",
			want:    QNameParts{Local: "name"},
		},
		{
			name:    "prefixed",
			lexical: "p:name",
			want:    QNameParts{Prefix: "p", Local: "name", Prefixed: true},
		},
		{name: "empty", lexical: " ", wantMsg: invalidQNameMessagePrefix},
		{name: "bad unprefixed", lexical: "1bad", wantMsg: invalidQNameMessagePrefix + "1bad"},
		{name: "bad prefixed", lexical: "p:1bad", wantMsg: invalidQNameMessagePrefix + "p:1bad"},
		{name: "missing prefix", lexical: ":name", wantMsg: invalidQNameMessagePrefix + ":name"},
		{name: "missing local", lexical: "p:", wantMsg: invalidQNameMessagePrefix + "p:"},
		{name: "multiple colons", lexical: "p:a:b", wantMsg: invalidQNameMessagePrefix + "p:a:b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseQNameParts(tt.lexical)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ParseQNameParts() error = %v", err)
				}
				if got != tt.want {
					t.Fatalf("ParseQNameParts() = %#v, want %#v", got, tt.want)
				}
				return
			}
			diag, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("ParseQNameParts() error = %T %[1]v, want xsderrors.Error", err)
			}
			if diag.Category != xsderrors.CategorySchemaCompile || diag.Code != xsderrors.CodeSchemaReference || diag.Message != tt.wantMsg {
				t.Fatalf("diagnostic = (%s, %s, %q), want (%s, %s, %q)", diag.Category, diag.Code, diag.Message, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaReference, tt.wantMsg)
			}
		})
	}
}
