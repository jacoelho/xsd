package xmlstream

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// FuzzReader exercises the public Reader lifecycle against arbitrary XML. A
// start token is admitted and its handle retained until the matching end
// token, which keeps the fuzz target on the same transaction path as users.
func FuzzReader(f *testing.F) {
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
		var reader Reader
		if err := reader.Reset(strings.NewReader(input), Config{}); err != nil {
			return
		}
		frames := make([]Handle, 0, 16)
		for tokens := 0; ; tokens++ {
			if tokens > 4096 {
				t.Fatalf("Reader.Next emitted more than 4096 tokens for %d input bytes", len(input))
			}
			tok, err := reader.Next()
			if errors.Is(err, io.EOF) {
				completeErr := reader.Complete()
				if completeErr != nil {
					return
				}
				return
			}
			if err != nil {
				return
			}
			switch tok.Kind {
			case KindStart:
				frame, _, err := reader.Start()
				if err != nil {
					return
				}
				frames = append(frames, frame)
			case KindEnd:
				if len(frames) == 0 {
					return
				}
				frame := frames[len(frames)-1]
				if err := reader.End(frame); err != nil {
					return
				}
				frames = frames[:len(frames)-1]
			case KindCharData, KindDirective, KindComment, KindPI:
				// These tokens do not change the retained element-handle stack.
			}
		}
	})
}
