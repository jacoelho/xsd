package xmlstream

import (
	"fmt"
	"strings"
	"testing"
)

// A name read from one window and the same name read one byte per call reach
// the same outcome, including which of a token limit and a syntax error wins.
func TestParserNameOutcomeIndependentOfDelivery(t *testing.T) {
	names := []string{"e", "ab", "p:l", "π:名", "é", "·a", "1bc", "a:b:c", ":a", "a:", "a@b", "1", "-", ":"}
	for _, name := range names {
		for _, position := range []struct {
			kind     string
			document string
		}{
			{"element", "<r>pad<" + name + "/>"},
			{"attribute", `<r>pad<e ` + name + `="v"/>`},
			{"end", "<" + name + ">pad</" + name + ">"},
		} {
			for _, limit := range []int64{0, int64(len(name)) - 1, int64(len(name)), int64(len(name)) + 4} {
				config := Config{Limits: Limits{MaxTokenBytes: limit}}
				want := parserOutcome(t, oneByteReads(position.document), config)
				for _, reads := range deliveries(position.document) {
					if got := parserOutcome(t, reads, config); got != want {
						t.Fatalf("%s %q limit %d reads %q:\n got %s\nwant %s", position.kind, name, limit, reads, got, want)
					}
				}
			}
		}
	}
}

// At the element position the name is the token's only charged bytes, so a
// nonzero allowance one byte short reports the token limit and an exact allowance
// reaches the syntax verdict, whether the terminator is in the first window,
// the next one, or is the first byte of a refill.
func TestParserWindowNameLimitPrecedence(t *testing.T) {
	for _, test := range []struct {
		name  string
		valid bool
	}{
		{"e", true}, {"ab", true}, {"p:l", true}, {"π:名", true},
		{"é", true}, {"·a", false}, {"1bc", false}, {"a:b:c", false}, {"a:", false}, {"1", false}, {"-", false},
	} {
		document := "<r><" + test.name + "/>"
		for _, reads := range deliveries(document) {
			for _, limit := range []int64{int64(len(test.name)) - 1, int64(len(test.name))} {
				if limit == 0 {
					continue // Zero disables the limit.
				}
				got := parserOutcome(t, reads, Config{Limits: Limits{MaxTokenBytes: limit}})
				want := "start r|error " + errXMLTokenLimit.Error()
				if limit == int64(len(test.name)) {
					want = "start r|error invalid XML qualified name"
					if test.valid {
						want = "start r|start " + test.name + "|end " + test.name
					}
				}
				if !strings.HasPrefix(got, want) {
					t.Fatalf("name %q limit %d reads %q = %s, want %s", test.name, limit, reads, got, want)
				}
			}
		}
	}
}

// deliveries returns the document as one read and as two reads cut at every
// byte, so a name's terminator falls in the first window, at the start of a
// refill, and past it.
func deliveries(document string) [][]string {
	out := [][]string{{document}}
	for cut := 1; cut < len(document); cut++ {
		out = append(out, []string{document[:cut], document[cut:]})
	}
	return out
}

func oneByteReads(document string) []string {
	reads := make([]string, len(document))
	for i := range len(document) {
		reads[i] = document[i : i+1]
	}
	return reads
}

// parserOutcome renders every token name until the document ends or fails.
// The text of character data is omitted because only names are under test.
func parserOutcome(t *testing.T, reads []string, config Config) string {
	t.Helper()
	script := make([]scriptedRead, len(reads))
	for i, read := range reads {
		script[i] = scriptedRead{data: []byte(read)}
	}
	var parser parserTestHarness
	if err := parser.reset(&scriptedReader{reads: script}, config); err != nil {
		t.Fatal(err)
	}
	defer parser.detach()
	var out []string
	for {
		token, err := parser.next()
		if IsOnlyEOF(err) {
			return strings.Join(append(out, "end-of-document"), "|")
		}
		if err != nil {
			return strings.Join(append(out, "error "+err.Error()), "|")
		}
		switch token.Kind {
		case KindStart:
			entry := "start " + qnameText(token.Start.Name.Space, token.Start.Name.Local)
			for i := range token.Start.Attr {
				attr := &token.Start.Attr[i]
				entry += fmt.Sprintf(" %s=%q", qnameText(attr.Name.Space, attr.Name.Local), parser.attrValue(attr))
			}
			out = append(out, entry)
		case KindEnd:
			out = append(out, "end "+qnameText(token.End.Name.Space, token.End.Name.Local))
		case KindCharData, KindDirective, KindComment, KindPI:
			// Name admission is observable only through element and attribute names.
		}
	}
}

func qnameText(prefix, local string) string {
	if prefix == "" {
		return local
	}
	return prefix + ":" + local
}
