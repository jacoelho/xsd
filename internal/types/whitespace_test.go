package types

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestWhiteSpace_Inheritance(t *testing.T) {
	// test that derived types inherit whitespace from base type
	baseType := mustBuiltinSimpleType(t, TypeNameString)
	baseType.SetWhiteSpace(WhiteSpacePreserve)

	derivedType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "MyString",
		},
		Restriction: &Restriction{
			Base: baseType.QName,
		},
	}
	derivedType.ResolvedBase = baseType
	derivedType.SetWhiteSpace(baseType.WhiteSpace()) // inherit from base

	if derivedType.WhiteSpace() != WhiteSpacePreserve {
		t.Errorf("WhiteSpace = %v, want %v", derivedType.WhiteSpace(), WhiteSpacePreserve)
	}
}

func TestWhiteSpace_Override(t *testing.T) {
	// test that whitespace can be overridden in restrictions
	baseType := mustBuiltinSimpleType(t, TypeNameString)
	baseType.SetWhiteSpace(WhiteSpacePreserve)

	derivedType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "MyString",
		},
		Restriction: &Restriction{
			Base: baseType.QName,
		},
	}
	derivedType.ResolvedBase = baseType
	derivedType.SetWhiteSpace(WhiteSpaceCollapse) // override to collapse

	if derivedType.WhiteSpace() != WhiteSpaceCollapse {
		t.Errorf("WhiteSpace = %v, want %v", derivedType.WhiteSpace(), WhiteSpaceCollapse)
	}
}

func TestWhiteSpace_StricterOnly(t *testing.T) {
	// test that whitespace can only be made stricter (preserve -> replace -> collapse)
	// this is validated during schema validation, not here, but we can test the values
	tests := []struct {
		name      string
		base      WhiteSpace
		derived   WhiteSpace
		shouldErr bool
	}{
		{"preserve to replace", WhiteSpacePreserve, WhiteSpaceReplace, false},
		{"preserve to collapse", WhiteSpacePreserve, WhiteSpaceCollapse, false},
		{"replace to collapse", WhiteSpaceReplace, WhiteSpaceCollapse, false},
		{"preserve to preserve", WhiteSpacePreserve, WhiteSpacePreserve, false},
		{"replace to replace", WhiteSpaceReplace, WhiteSpaceReplace, false},
		{"collapse to collapse", WhiteSpaceCollapse, WhiteSpaceCollapse, false},
		// these should fail validation (but we're just testing the values here)
		{"replace to preserve", WhiteSpaceReplace, WhiteSpacePreserve, true},
		{"collapse to preserve", WhiteSpaceCollapse, WhiteSpacePreserve, true},
		{"collapse to replace", WhiteSpaceCollapse, WhiteSpaceReplace, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseType := &SimpleType{
				QName: QName{
					Namespace: "http://example.com",
					Local:     "Base",
				},
			}
			baseType.SetWhiteSpace(tt.base)

			derivedType := &SimpleType{
				QName: QName{
					Namespace: "http://example.com",
					Local:     "Derived",
				},
			}
			derivedType.ResolvedBase = baseType
			derivedType.SetWhiteSpace(tt.derived)

			// check if the values are set correctly
			if derivedType.WhiteSpace() != tt.derived {
				t.Errorf("WhiteSpace = %v, want %v", derivedType.WhiteSpace(), tt.derived)
			}
			// the actual validation that it's stricter will be in schema validator
		})
	}
}

func TestNormalizeValue_WhiteSpace(t *testing.T) {
	typ := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "NormalizedString",
		},
	}
	typ.SetWhiteSpace(WhiteSpaceCollapse)

	normalized, err := NormalizeValue(" \talpha \n  beta\r\n", typ)
	if err != nil {
		t.Fatalf("NormalizeValue() error = %v", err)
	}
	if normalized != "alpha beta" {
		t.Errorf("NormalizeValue() = %q, want %q", normalized, "alpha beta")
	}
}

func TestNormalizeValue_XMLWhitespaceOnly(t *testing.T) {
	typ := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "NormalizedString",
		},
	}
	typ.SetWhiteSpace(WhiteSpaceCollapse)

	input := "alpha\u00a0beta"
	normalized, err := NormalizeValue(input, typ)
	if err != nil {
		t.Fatalf("NormalizeValue() error = %v", err)
	}
	if normalized != input {
		t.Errorf("NormalizeValue() = %q, want %q", normalized, input)
	}
}

func TestApplyWhiteSpaceMatchesValueNormalize(t *testing.T) {
	cases := []struct {
		name  string
		ws    WhiteSpace
		input string
	}{
		{name: "preserve", ws: WhiteSpacePreserve, input: " a\tb\n"},
		{name: "replace", ws: WhiteSpaceReplace, input: " a\tb\n"},
		{name: "collapse", ws: WhiteSpaceCollapse, input: "  a\tb \n c  "},
		{name: "collapse no change", ws: WhiteSpaceCollapse, input: "a b"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ApplyWhiteSpace(tc.input, tc.ws)
			mode := value.WhitespacePreserve
			switch tc.ws {
			case WhiteSpaceReplace:
				mode = value.WhitespaceReplace
			case WhiteSpaceCollapse:
				mode = value.WhitespaceCollapse
			}
			want := string(value.NormalizeWhitespace(mode, []byte(tc.input), nil))
			if got != want {
				t.Fatalf("ApplyWhiteSpace(%q) = %q, want %q", tc.input, got, want)
			}
		})
	}
}

func TestSplitXMLWhitespaceFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty", input: "", want: []string{}},
		{name: "xml whitespace only", input: " \t\r\n", want: []string{}},
		{name: "single token", input: "a", want: []string{"a"}},
		{name: "space separated", input: "a b", want: []string{"a", "b"}},
		{name: "tab separated", input: "a\tb", want: []string{"a", "b"}},
		{name: "lf separated", input: "a\nb", want: []string{"a", "b"}},
		{name: "cr separated", input: "a\rb", want: []string{"a", "b"}},
		{name: "crlf separated", input: "a\r\nb", want: []string{"a", "b"}},
		{name: "mixed separators", input: " \ta\r\nb\nc\t ", want: []string{"a", "b", "c"}},
		{name: "non-xml nbsp", input: "a\u00A0b", want: []string{"a\u00A0b"}},
		{name: "non-xml nel", input: "a\u0085b", want: []string{"a\u0085b"}},
		{name: "non-xml ls", input: "a\u2028b", want: []string{"a\u2028b"}},
		{name: "non-xml ps", input: "a\u2029b", want: []string{"a\u2029b"}},
		{name: "non-xml thin space", input: "a\u2009b", want: []string{"a\u2009b"}},
		{name: "non-xml vt", input: "a\u000bb", want: []string{"a\u000bb"}},
		{name: "non-xml ff", input: "a\u000cb", want: []string{"a\u000cb"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitXMLWhitespaceFields(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitXMLWhitespaceFields length = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("splitXMLWhitespaceFields[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCountXMLFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "empty", input: "", want: 0},
		{name: "xml whitespace only", input: " \t\r\n", want: 0},
		{name: "single token", input: "a", want: 1},
		{name: "space separated", input: "a b", want: 2},
		{name: "tab separated", input: "a\tb", want: 2},
		{name: "lf separated", input: "a\nb", want: 2},
		{name: "cr separated", input: "a\rb", want: 2},
		{name: "crlf separated", input: "a\r\nb", want: 2},
		{name: "mixed separators", input: " \ta\r\nb\nc\t ", want: 3},
		{name: "double spaces", input: "a  b", want: 2},
		{name: "non-xml nbsp", input: "a\u00A0b", want: 1},
		{name: "non-xml nel", input: "a\u0085b", want: 1},
		{name: "non-xml ls", input: "a\u2028b", want: 1},
		{name: "non-xml ps", input: "a\u2029b", want: 1},
		{name: "non-xml thin space", input: "a\u2009b", want: 1},
		{name: "non-xml vt", input: "a\u000bb", want: 1},
		{name: "non-xml ff", input: "a\u000cb", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countXMLFields(tt.input); got != tt.want {
				t.Fatalf("countXMLFields(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
