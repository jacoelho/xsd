package stream

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func FuzzXMLStreamParser(f *testing.F) {
	for _, seed := range []string{
		`<root/>`,
		`<?xml version="1.0"?><root a="&amp;">text</root>`,
		`<root><![CDATA[x<y]]><!--c--><?pi v?></root>`,
		`<root><a/><b attr="value">text</b></root>`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			t.Skip()
		}
		names := NewCache()
		values := NewCache()
		parser := new(Parser)
		if err := parser.Reset(strings.NewReader(input), &names, &values); err != nil {
			return
		}
		for tokens := 0; ; tokens++ {
			if tokens > 4096 {
				t.Skip()
			}
			_, err := parser.Next()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				return
			}
		}
	})
}
