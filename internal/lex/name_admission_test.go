package lex

import "testing"

func TestNameAdmissionAcrossEncodingBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input  string
		xml    bool
		ncname bool
	}{
		{input: "é", xml: true, ncname: true},
		{input: "aé", xml: true, ncname: true},
		{input: "a\u0301", xml: true, ncname: true},
		{input: "\u0301a"},
		{input: ":a", xml: true},
		{input: "a:b", xml: true},
		{input: "\uFFFD", xml: true, ncname: true},
		{input: "a\uFFFE"},
		{input: "\xffa"},
		{input: "a\xc3"},
		{input: "a\xc0\xaf"},
		{input: "a\xed\xa0\x80"},
	} {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			if got := IsXMLName(test.input); got != test.xml {
				t.Errorf("IsXMLName(%q) = %v, want %v", test.input, got, test.xml)
			}
			if got := IsNCName(test.input); got != test.ncname {
				t.Errorf("IsNCName(%q) = %v, want %v", test.input, got, test.ncname)
			}
			if got := IsXMLNameBytes([]byte(test.input)); got != test.xml {
				t.Errorf("IsXMLNameBytes(%q) = %v, want %v", test.input, got, test.xml)
			}
			if got := IsNCNameBytes([]byte(test.input)); got != test.ncname {
				t.Errorf("IsNCNameBytes(%q) = %v, want %v", test.input, got, test.ncname)
			}
		})
	}
}
