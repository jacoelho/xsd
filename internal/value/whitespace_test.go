package value

import (
	"bytes"
	"reflect"
	"testing"
)

func TestNormalizeWhitespaceReplace(t *testing.T) {
	in := []byte("a\tb\nc\rd")
	got := NormalizeWhitespace(WhitespaceReplace, in, nil)
	want := []byte("a b c d")
	if !bytes.Equal(got, want) {
		t.Fatalf("NormalizeWhitespace(replace) = %q, want %q", string(got), string(want))
	}
}

func TestNormalizeWhitespaceCollapse(t *testing.T) {
	in := []byte("  a\t b \n c  ")
	got := NormalizeWhitespace(WhitespaceCollapse, in, nil)
	want := []byte("a b c")
	if !bytes.Equal(got, want) {
		t.Fatalf("NormalizeWhitespace(collapse) = %q, want %q", string(got), string(want))
	}
}

func TestTrimXMLWhitespace(t *testing.T) {
	in := []byte("\t abc \n")
	got := TrimXMLWhitespace(in)
	want := []byte("abc")
	if !bytes.Equal(got, want) {
		t.Fatalf("TrimXMLWhitespace() = %q, want %q", string(got), string(want))
	}
}

func TestTrimXMLWhitespaceString(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: " \t\n\r foo \t\n\r ", want: "foo"},
		{in: "foo", want: "foo"},
		{in: "  ", want: ""},
		{in: "", want: ""},
	}
	for _, tc := range cases {
		if got := TrimXMLWhitespaceString(tc.in); got != tc.want {
			t.Fatalf("TrimXMLWhitespaceString(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSplitXMLWhitespace(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{in: "a b c", want: []string{"a", "b", "c"}},
		{in: "  a  b  ", want: []string{"a", "b"}},
		{in: "a\tb\nc", want: []string{"a", "b", "c"}},
		{in: " \t\n\r ", want: nil},
		{in: "", want: nil},
	}
	for _, tc := range cases {
		gotBytes := SplitXMLWhitespace([]byte(tc.in))
		var got []string
		for _, item := range gotBytes {
			got = append(got, string(item))
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("SplitXMLWhitespace(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestForEachXMLWhitespaceField(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{in: "a b c", want: []string{"a", "b", "c"}},
		{in: "  a  b  ", want: []string{"a", "b"}},
		{in: "a\tb\nc", want: []string{"a", "b", "c"}},
		{in: " \t\n\r ", want: nil},
		{in: "", want: nil},
	}
	for _, tc := range cases {
		var got []string
		count, err := ForEachXMLWhitespaceField([]byte(tc.in), func(field []byte) error {
			got = append(got, string(field))
			return nil
		})
		if err != nil {
			t.Fatalf("ForEachXMLWhitespaceField(%q) error = %v", tc.in, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("ForEachXMLWhitespaceField(%q) = %v, want %v", tc.in, got, tc.want)
		}
		if count != len(tc.want) {
			t.Fatalf("ForEachXMLWhitespaceField(%q) count = %d, want %d", tc.in, count, len(tc.want))
		}
	}
}

func TestFieldsXMLWhitespaceSeq(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{in: "a b c", want: []string{"a", "b", "c"}},
		{in: "  a  b  ", want: []string{"a", "b"}},
		{in: "a\tb\nc", want: []string{"a", "b", "c"}},
		{in: " \t\n\r ", want: nil},
		{in: "", want: nil},
	}
	for _, tc := range cases {
		var got []string
		for field := range FieldsXMLWhitespaceSeq([]byte(tc.in)) {
			got = append(got, string(field))
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("FieldsXMLWhitespaceSeq(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestFieldsXMLWhitespaceStringSeq(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{in: "a b c", want: []string{"a", "b", "c"}},
		{in: "  a  b  ", want: []string{"a", "b"}},
		{in: "a\tb\nc", want: []string{"a", "b", "c"}},
		{in: " \t\n\r ", want: nil},
		{in: "", want: nil},
		{in: "a\u00A0b", want: []string{"a\u00A0b"}},
	}
	for _, tc := range cases {
		var got []string
		for field := range FieldsXMLWhitespaceStringSeq(tc.in) {
			got = append(got, field)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("FieldsXMLWhitespaceStringSeq(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestIsXMLWhitespaceByte(t *testing.T) {
	for _, b := range []byte{' ', '\t', '\n', '\r'} {
		if !IsXMLWhitespaceByte(b) {
			t.Fatalf("expected %q to be XML whitespace", b)
		}
	}
	for _, b := range []byte{'a', '0', 0} {
		if IsXMLWhitespaceByte(b) {
			t.Fatalf("expected %q to be non-whitespace", b)
		}
	}
}
