package xmlstream

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/jacoelho/xsd/internal/lex"
)

// The name byte table must classify every byte exactly as the XML name rules
// do; the parser trusts it for ASCII QName verdicts.
func TestNameByteClassesMatchNameRules(t *testing.T) {
	t.Parallel()

	isStart := func(b byte) bool { return nameByteClasses[b]&nameByteStart != 0 }
	isChar := func(b byte) bool { return nameByteClasses[b]&nameByteChar != 0 }
	isColon := func(b byte) bool { return nameByteClasses[b]&nameByteColon != 0 }
	isTerminator := func(b byte) bool { return nameByteClasses[b]&nameByteTerminator != 0 }
	for i := range 256 {
		b := byte(i)
		letter := 'A' <= b && b <= 'Z' || 'a' <= b && b <= 'z'
		wantStart := letter || b == '_'
		wantChar := wantStart || b == '-' || b == '.' || '0' <= b && b <= '9'
		wantColon := b == ':'
		wantTerminator := strings.IndexByte(" \t\n\r/>=", b) >= 0
		if b >= 0x80 {
			wantStart, wantChar, wantColon, wantTerminator = false, false, false, false
		}
		if got := isStart(b); got != wantStart {
			t.Errorf("byte %#02x start = %v, want %v", b, got, wantStart)
		}
		if got := isChar(b); got != wantChar {
			t.Errorf("byte %#02x char = %v, want %v", b, got, wantChar)
		}
		if got := isColon(b); got != wantColon {
			t.Errorf("byte %#02x colon = %v, want %v", b, got, wantColon)
		}
		if got := isTerminator(b); got != wantTerminator {
			t.Errorf("byte %#02x terminator = %v, want %v", b, got, wantTerminator)
		}
	}
}

var asciiQNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*(:[A-Za-z_][A-Za-z0-9_.-]*)?$`)

// specQName is the QName ::= NCName (':' NCName)? verdict for name, with the
// parts the parser must report for a valid name.
func specQName(name string) (xml.Name, bool) {
	if strings.Count(name, ":") > 1 {
		return xml.Name{}, false
	}
	prefix, local, prefixed := strings.Cut(name, ":")
	if !prefixed {
		return xml.Name{Local: name}, lex.IsNCNameBytes([]byte(name))
	}
	valid := lex.IsNCNameBytes([]byte(prefix)) && lex.IsNCNameBytes([]byte(local))
	return xml.Name{Space: prefix, Local: local}, valid
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// enumerateNames returns every string of 1..maxLen symbols from alphabet.
func enumerateNames(alphabet []string, maxLen int) []string {
	var names []string
	level := []string{""}
	for range maxLen {
		var next []string
		for _, prefix := range level {
			for _, symbol := range alphabet {
				next = append(next, prefix+symbol)
			}
		}
		names = append(names, next...)
		level = next
	}
	return names
}

// qnameSite places a name where the parser admits a QName and returns the
// name the parser reported there.
type qnameSite struct {
	name  string
	build func(name string) string
	// read drives the parser to the token carrying the name.
	read func(parser *parserTestHarness) (xml.Name, error)
	// nameStart is the offset of the name in build's document.
	nameStart int
}

var qnameSites = []qnameSite{
	{
		name:      "start",
		build:     func(name string) string { return "<r>pad<" + name + "/>" },
		nameStart: len("<r>pad<"),
		read: func(parser *parserTestHarness) (xml.Name, error) {
			for range 2 {
				if _, err := parser.next(); err != nil {
					return xml.Name{}, err
				}
			}
			token, err := parser.next()
			if err != nil {
				return xml.Name{}, err
			}
			return token.Start.Name, nil
		},
	},
	{
		name:      "attribute",
		build:     func(name string) string { return "<r " + name + `="v"/>` },
		nameStart: len("<r "),
		read: func(parser *parserTestHarness) (xml.Name, error) {
			token, err := parser.next()
			if err != nil {
				return xml.Name{}, err
			}
			if len(token.Start.Attr) != 1 || token.Start.Attr[0].Value != "v" {
				return xml.Name{}, errUnexpectedAttributes
			}
			return token.Start.Attr[0].Name, nil
		},
	},
	{
		name:      "end",
		build:     func(name string) string { return "<r></" + name + ">" },
		nameStart: len("<r></"),
		read: func(parser *parserTestHarness) (xml.Name, error) {
			if _, err := parser.next(); err != nil {
				return xml.Name{}, err
			}
			token, err := parser.next()
			if err != nil {
				return xml.Name{}, err
			}
			return token.End.Name, nil
		},
	},
}

var errUnexpectedAttributes = errors.New("unexpected attributes in QName site test")

// Every short name over an alphabet of name-start, name, colon, invalid, and
// non-ASCII bytes gets the QName verdict and parts the spec gives it, at every
// name site and however the source splits the name across reads.
func TestQNameAdmissionMatchesSpec(t *testing.T) {
	t.Parallel()

	names := enumerateNames([]string{"a", "1", "-", ":", "@", "é", "\xff"}, 4)
	names = append(names, enumerateNames([]string{"a", "1", ":", "@"}, 6)...)
	names = append(names, "row", "xs:int", ":bad", "bad:", "a:b:c", "bad@name", "é", "a-b.c1:d-e.f2")

	var parser parserTestHarness
	defer parser.detach()
	check := func(site qnameSite, name, reader string, input io.Reader) {
		t.Helper()
		want, valid := specQName(name)
		if err := parser.reset(input, Config{}); err != nil {
			t.Fatal(err)
		}
		got, err := site.read(&parser)
		switch {
		case valid && (err != nil || got != want):
			t.Fatalf("%s name %q (% x) %s = %+v, %v, want %+v", site.name, name, name, reader, got, err, want)
		case !valid && (err == nil || err.Error() != "invalid XML qualified name"):
			t.Fatalf("%s name %q (% x) %s = %+v, %v, want invalid XML qualified name", site.name, name, name, reader, got, err)
		}
	}
	for _, name := range names {
		if isASCII(name) {
			_, valid := specQName(name)
			if asciiQNamePattern.MatchString(name) != valid {
				t.Fatalf("oracles disagree on %q", name)
			}
		}
		for _, site := range qnameSites {
			document := site.build(name)
			check(site, name, "whole", strings.NewReader(document))
			check(site, name, "one_byte", iotest.OneByteReader(strings.NewReader(document)))
			if site.name != "start" {
				continue
			}
			for cut := site.nameStart; cut <= site.nameStart+len(name); cut++ {
				check(site, name, fmt.Sprintf("cut %d", cut), &scriptedReader{reads: []scriptedRead{
					{data: []byte(document[:cut])}, {data: []byte(document[cut:])},
				}})
			}
		}
	}
}
