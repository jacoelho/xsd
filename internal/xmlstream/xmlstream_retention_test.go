package xmlstream

import (
	"strconv"
	"strings"
	"testing"
)

type parserScratchCaps struct {
	name         int
	attrValue    int
	attrEnds     int
	attrRetained int
	attrs        int
}

func scratchCaps(p *parser) parserScratchCaps {
	return parserScratchCaps{
		name:         cap(p.nameBuf),
		attrValue:    cap(p.attrValueBuf),
		attrEnds:     cap(p.attrValueEnds),
		attrRetained: cap(p.attrValueRetained),
		attrs:        cap(p.attrs),
	}
}

func oversizedElement() string {
	name := strings.Repeat("n", maxRetainedBufferCap+1)
	uri := strings.Repeat("u", maxRetainedBufferCap+1)
	var input strings.Builder
	input.Grow(len(name) + len(uri) + (maxRetainedSliceCap+1)*12 + 32)
	input.WriteByte('<')
	input.WriteString(name)
	input.WriteString(` xmlns:p="`)
	input.WriteString(uri)
	input.WriteString(`"`)
	for i := range maxRetainedSliceCap + 1 {
		input.WriteString(` a`)
		input.WriteString(strconv.Itoa(i))
		input.WriteString(`="x"`)
	}
	input.WriteString(`/>`)
	return input.String()
}

func TestParserDetachDropsOversizedTokenScratch(t *testing.T) {
	input := oversizedElement()
	var reader Reader
	if err := reader.Reset(strings.NewReader(input), Config{Limits: Limits{
		MaxInputBytes: int64(len(input)) + 1,
		MaxTokenBytes: int64(len(input)) + 1,
	}, LazyAttrValues: true, SkipOrdinaryAttrValues: true}); err != nil {
		t.Fatal(err)
	}
	token, err := reader.Next()
	if err != nil {
		t.Fatal(err)
	}
	caps := scratchCaps(&reader.parser)
	if caps.name <= maxRetainedBufferCap || caps.attrValue <= maxRetainedBufferCap {
		t.Fatalf("oversized token scratch capacities = %+v, want byte buffers above %d", caps, maxRetainedBufferCap)
	}
	if caps.attrs <= maxRetainedSliceCap || caps.attrEnds <= maxRetainedSliceCap || caps.attrRetained <= maxRetainedSliceCap {
		t.Fatalf("oversized token slice capacities = %+v, want slices above %d", caps, maxRetainedSliceCap)
	}

	reader.Detach()
	if got := scratchCaps(&reader.parser); got != (parserScratchCaps{}) {
		t.Fatalf("scratch after Detach = %+v, want all oversized buffers dropped", got)
	}
	if token.Start.Name.Local != "" || token.Start.Attr != nil {
		t.Fatalf("borrowed token survived Detach: %+v", *token)
	}
}

func TestParserDetachRetainsSmallScratchForReset(t *testing.T) {
	var reader Reader
	const first = `<r a="value"/>`
	const second = `<s a="other"/>`
	if err := reader.Reset(strings.NewReader(first), Config{}); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Next(); err != nil {
		t.Fatal(err)
	}
	want := scratchCaps(&reader.parser)
	if want == (parserScratchCaps{}) {
		t.Fatal("small token did not allocate parser scratch")
	}
	reader.Detach()
	if got := scratchCaps(&reader.parser); got != want {
		t.Fatalf("scratch after small Detach = %+v, want reusable %+v", got, want)
	}

	if err := reader.Reset(strings.NewReader(second), Config{}); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Next(); err != nil {
		t.Fatal(err)
	}
	if got := scratchCaps(&reader.parser); got != want {
		t.Fatalf("scratch after next Reset = %+v, want reuse of %+v", got, want)
	}
	reader.Detach()
}
