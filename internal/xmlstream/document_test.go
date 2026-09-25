package xmlstream

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
)

func TestReaderAdmitsNamespacesAndCompletesDocument(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r xmlns:p="urn:test"><p:child p:id="v"/></r>`), Config{Limits: Limits{MaxDepth: 8}}); err != nil {
		t.Fatal(err)
	}

	rootToken := mustNextToken(t, &reader)
	rootFrame, root, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if root.Name != (xml.Name{Local: "r"}) {
		t.Fatalf("root expanded name = %+v", root.Name)
	}
	if uri, ok := reader.Lookup("p"); !ok || uri != "urn:test" {
		t.Fatalf("Lookup(p) = %q, %v", uri, ok)
	}
	if got := rootToken.Start.Attr[0].Name; got != (xml.Name{Space: "xmlns", Local: "p"}) {
		t.Fatalf("namespace attribute = %+v", got)
	}
	context := reader.Context()

	childToken := mustNextToken(t, &reader)
	childFrame, child, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if child.Name != (xml.Name{Space: "urn:test", Local: "child"}) {
		t.Fatalf("child expanded name = %+v", child.Name)
	}
	if got := childToken.Start.Attr[0].Name; got != (xml.Name{Space: "urn:test", Local: "id"}) {
		t.Fatalf("expanded attribute = %+v", got)
	}
	if got, ok := context.Lookup("p"); !ok || got != "urn:test" {
		t.Fatalf("captured context Lookup(p) = %q, %v", got, ok)
	}
	_ = mustNextToken(t, &reader)
	if err := reader.MatchEnd(childFrame); err != nil {
		t.Fatal(err)
	}
	if err := reader.CommitEnd(childFrame); err != nil {
		t.Fatal(err)
	}
	if reader.Depth() != 1 {
		t.Fatalf("Depth after child = %d, want 1", reader.Depth())
	}
	_ = mustNextToken(t, &reader)
	if err := reader.MatchEnd(rootFrame); err != nil {
		t.Fatal(err)
	}
	if err := reader.CommitEnd(rootFrame); err != nil {
		t.Fatal(err)
	}
	if reader.Depth() != 0 {
		t.Fatalf("Depth after root = %d, want 0", reader.Depth())
	}
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("Next after document = %v, want EOF", err)
	}
	if err := reader.Complete(); err != nil {
		t.Fatal(err)
	}
}

func TestReaderDefaultNamespaceShadowRestoresAfterChild(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<root xmlns="urn:root"><child xmlns="urn:child"/><sibling/></root>`), Config{}); err != nil {
		t.Fatal(err)
	}

	rootToken := mustNextToken(t, &reader)
	if rootToken.Start.Name.Local != "root" {
		t.Fatalf("root lexical name = %+v", rootToken.Start.Name)
	}
	rootFrame, root, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if root.Name != (xml.Name{Space: "urn:root", Local: "root"}) {
		t.Fatalf("root expanded name = %+v", root.Name)
	}

	childToken := mustNextToken(t, &reader)
	if childToken.Start.Name.Local != "child" {
		t.Fatalf("child lexical name = %+v", childToken.Start.Name)
	}
	childFrame, child, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if child.Name != (xml.Name{Space: "urn:child", Local: "child"}) {
		t.Fatalf("child expanded name = %+v", child.Name)
	}
	_ = mustNextToken(t, &reader)
	err = reader.End(childFrame)
	if err != nil {
		t.Fatal(err)
	}

	siblingToken := mustNextToken(t, &reader)
	if siblingToken.Start.Name.Local != "sibling" {
		t.Fatalf("sibling lexical name = %+v", siblingToken.Start.Name)
	}
	siblingFrame, sibling, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if sibling.Name != (xml.Name{Space: "urn:root", Local: "sibling"}) {
		t.Fatalf("sibling expanded name = %+v", sibling.Name)
	}
	_ = mustNextToken(t, &reader)
	err = reader.End(siblingFrame)
	if err != nil {
		t.Fatal(err)
	}

	_ = mustNextToken(t, &reader)
	if err := reader.End(rootFrame); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("Next() after root = %v, want EOF", err)
	}
}

func TestReaderOwnsAndInvalidatesBorrowedToken(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r/>`), Config{}); err != nil {
		t.Fatal(err)
	}

	start := mustNextToken(t, &reader)
	frame, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	end := mustNextToken(t, &reader)
	if start != end {
		t.Fatal("Reader.Next returned separate token storage")
	}
	if end.Kind != KindEnd {
		t.Fatalf("current token kind = %v, want end", end.Kind)
	}
	if err := reader.End(frame); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("Next after document = %v, want EOF", err)
	}

	if err := reader.Reset(strings.NewReader(`<fresh/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	old := mustNextToken(t, &reader)
	if err := reader.Reset(strings.NewReader(`<new/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	if old.Start.Name != (xml.Name{}) || old.End.Name != (xml.Name{}) || old.Data != nil || old.Directive != nil || old.Line != 0 || old.Column != 0 {
		t.Fatalf("borrowed token survived Reset: %+v", *old)
	}
}

func TestReaderRejectsDocumentTopologyAndOutsideRootData(t *testing.T) {
	tests := []struct {
		want error
		name string
		xml  string
	}{
		{name: "text before root", xml: `text<r/>`, want: ErrTextOutsideRoot},
		{name: "reference before root", xml: `&#32;<r/>`, want: ErrReferenceOutsideRoot},
		{name: "CDATA before root", xml: `<![CDATA[]]><r/>`, want: ErrCDATOutsideRoot},
		{name: "DTD", xml: `<!DOCTYPE r><r/>`, want: ErrUnsupportedDTD},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reader Reader
			if err := reader.Reset(strings.NewReader(tt.xml), Config{}); err != nil {
				t.Fatal(err)
			}
			tok, err := reader.Next()
			if !errors.Is(err, tt.want) {
				t.Fatalf("Next() = %v, want cause %v", err, tt.want)
			}
			boundaryErr, ok := errors.AsType[*Error](err)
			if !ok || boundaryErr == nil {
				t.Fatalf("Next() error %T = %v, want *Error", err, err)
			}
			if tok != nil {
				t.Fatalf("Next() returned token %+v with error", tok)
			}
		})
	}

	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r/><s/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	frame, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	if err := reader.MatchEnd(frame); err != nil {
		t.Fatal(err)
	}
	if err := reader.CommitEnd(frame); err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	if _, _, err := reader.Start(); !errors.Is(err, ErrMultipleRoots) {
		t.Fatalf("second root Start() = %v, want %v", err, ErrMultipleRoots)
	}

	var empty Reader
	if err := empty.Reset(strings.NewReader(`</r>`), Config{}); err != nil {
		t.Fatal(err)
	}
	if _, err := empty.Next(); !errors.Is(err, ErrUnexpectedEnd) {
		t.Fatalf("unexpected end Next() = %v, want %v", err, ErrUnexpectedEnd)
	}
}

func TestReaderRejectsXMLDeclarationAfterComment(t *testing.T) {
	for _, config := range []Config{{}, {CommentMode: CommentModeEmit}} {
		var reader Reader
		if err := reader.Reset(strings.NewReader(`<!--comment--><?xml version="1.0"?><root/>`), config); err != nil {
			t.Fatal(err)
		}
		if config.CommentMode == CommentModeEmit {
			tok, err := reader.Next()
			if err != nil {
				t.Fatal(err)
			}
			if tok.Kind != KindComment {
				t.Fatalf("first token kind = %v, want comment", tok.Kind)
			}
		}
		if _, err := reader.Next(); err == nil {
			t.Fatal("accepted XML declaration after a comment")
		} else {
			boundary, ok := errors.AsType[*Error](err)
			if !ok || boundary == nil || boundary.Kind != ErrorSyntax {
				t.Fatalf("error = %T %v, want syntax boundary error", err, err)
			}
		}
	}
}

func TestReaderBoundedDiscardChargesCommentPayloadWithoutRetainingIt(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		limit int64
	}{
		{name: "plain", input: `<r><!--12345--></r>`, limit: 4},
		{name: "normalized line ending", input: "<r><!--\r\n123--></r>", limit: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			var reader Reader
			if err := reader.Reset(strings.NewReader(test.input), Config{
				CommentMode: CommentModeBoundedDiscard,
				Limits:      Limits{MaxTokenBytes: test.limit},
			}); err != nil {
				t.Fatal(err)
			}
			_ = mustNextToken(t, &reader)
			if _, _, err := reader.Start(); err != nil {
				t.Fatal(err)
			}
			if _, err := reader.Next(); err == nil || !IsTokenLimit(err) {
				t.Fatalf("Next() = %v, want comment token limit", err)
			}
			if cap(reader.parser.directive) != 0 {
				t.Fatalf("bounded discarded comment retained directive capacity %d", cap(reader.parser.directive))
			}
		})
	}
}

func TestReaderDiscardCommentModeDoesNotRetainOrChargePayload(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r><!--12345--></r>`), Config{Limits: Limits{MaxTokenBytes: 4}}); err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	frame, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	end := mustNextToken(t, &reader)
	if end.Kind != KindEnd {
		t.Fatalf("token kind = %v, want end after discarded comment", end.Kind)
	}
	if err := reader.End(frame); err != nil {
		t.Fatal(err)
	}
	if cap(reader.parser.directive) != 0 {
		t.Fatalf("discarded comment retained directive capacity %d", cap(reader.parser.directive))
	}
}

func TestReaderRejectsUnknownCommentMode(t *testing.T) {
	var reader Reader
	err := reader.Reset(strings.NewReader(`<r/>`), Config{CommentMode: CommentMode(99)})
	if !errors.Is(err, errXMLCommentMode) {
		t.Fatalf("Reset() error = %v, want invalid comment mode", err)
	}
}

func TestReaderNamespaceAdmissionRollsBackAndRejectsDuplicateExpandedAttributes(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r xmlns:a="urn" xmlns:b="urn" a:x="1" b:x="2"/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	if _, _, err := reader.Start(); err == nil {
		t.Fatal("Start() accepted duplicate expanded attributes")
	} else {
		var boundaryErr *Error
		if !errors.As(err, &boundaryErr) || boundaryErr.Kind != ErrorNamespace {
			t.Fatalf("Start() error = %T %v, want namespace boundary error", err, err)
		}
	}
	if reader.Depth() != 0 {
		t.Fatalf("Depth after failed start = %d, want 0", reader.Depth())
	}
	if _, ok := reader.Lookup("a"); ok {
		t.Fatal("failed start retained namespace binding")
	}
}

func TestReaderAbortStartAndResetInvalidateFrames(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r xmlns:p="urn"><p:c/></r>`), Config{}); err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	frame, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.AbortStart(frame); err != nil {
		t.Fatal(err)
	}
	if reader.Depth() != 0 {
		t.Fatalf("Depth after AbortStart = %d, want 0", reader.Depth())
	}
	if _, ok := reader.Lookup("p"); ok {
		t.Fatal("AbortStart retained namespace binding")
	}
	if err := reader.Reset(strings.NewReader(`<new/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	if err := reader.CommitEnd(frame); !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("stale CommitEnd() = %v, want %v", err, ErrInvalidFrame)
	}
}

func TestReaderDepthAndParserLimitsAreTyped(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r><c/></r>`), Config{Limits: Limits{MaxDepth: 1}}); err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	_, element, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if element.Name.Local != "r" {
		t.Fatalf("root expanded name = %+v", element.Name)
	}
	_ = mustNextToken(t, &reader)
	if _, _, err := reader.Start(); err == nil {
		t.Fatal("Start() accepted depth beyond limit")
	} else {
		boundaryErr, ok := errors.AsType[*Error](err)
		if !ok || boundaryErr.Kind != ErrorDepth {
			t.Fatalf("depth error = %T %v, want ErrorDepth", err, err)
		}
	}
	if reader.Depth() != 1 {
		t.Fatalf("Depth after rejected child = %d, want 1", reader.Depth())
	}
	if err := reader.Reset(strings.NewReader(`<fresh/>`), Config{}); err != nil {
		t.Fatal(err)
	}

	if err := reader.Reset(strings.NewReader(`<r a="123"/>`), Config{Limits: Limits{MaxTokenBytes: 2}}); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Next(); err == nil {
		t.Fatal("Next() accepted token beyond token limit")
	} else {
		var boundaryErr *Error
		if !errors.As(err, &boundaryErr) || boundaryErr.Kind != ErrorLimit {
			t.Fatalf("token limit error = %T %v, want ErrorLimit", err, err)
		}
	}
}

func TestReaderMaterializesBorrowedAttributeBeforeAdvance(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r a="value"/>`), Config{LazyAttrValues: true}); err != nil {
		t.Fatal(err)
	}
	tok := mustNextToken(t, &reader)
	if len(tok.Start.Attr) != 1 {
		t.Fatalf("attribute count = %d, want 1", len(tok.Start.Attr))
	}
	value, ok := reader.MaterializeValue(&tok.Start.Attr[0])
	if !ok || value != "value" {
		t.Fatalf("MaterializeValue() = %q, %v", value, ok)
	}
	if _, _, err := reader.Start(); err != nil {
		t.Fatal(err)
	}
}

func TestReaderSkipsOrdinaryAttributeValuesWhileRetainingNamespaceValues(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r a="value" xmlns:p="urn:test" xml:space="preserve"/>`), Config{
		LazyAttrValues:         true,
		SkipOrdinaryAttrValues: true,
	}); err != nil {
		t.Fatal(err)
	}
	tok := mustNextToken(t, &reader)
	if len(tok.Start.Attr) != 3 {
		t.Fatalf("attribute count = %d, want 3", len(tok.Start.Attr))
	}
	if raw, ok := tok.Start.Attr[0].RawValue(); ok || raw != nil {
		t.Fatalf("ordinary attribute RawValue() = %q, %v; want no retained value", raw, ok)
	}
	for _, index := range []int{1, 2} {
		if raw, ok := tok.Start.Attr[index].RawValue(); !ok || len(raw) == 0 {
			t.Fatalf("retained attribute %d RawValue() = %q, %v; want retained value", index, raw, ok)
		}
	}
	if _, _, err := reader.Start(); err != nil {
		t.Fatalf("Start() = %v", err)
	}
}

func TestReaderSkipOrdinaryAttributeValuesStillValidatesLexicalContent(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r value="&unknown;"/>`), Config{
		LazyAttrValues:         true,
		SkipOrdinaryAttrValues: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Next(); !IsUnsupportedEntityReference(err) {
		t.Fatalf("Next() = %v, want unsupported entity reference", err)
	}
}

func TestReaderSkipOrdinaryAttributeValuesStillEnforcesTokenLimit(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r value="12345"/>`), Config{
		Limits:                 Limits{MaxTokenBytes: 4},
		LazyAttrValues:         true,
		SkipOrdinaryAttrValues: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Next(); err == nil || !IsTokenLimit(err) {
		t.Fatalf("Next() = %v, want token limit", err)
	}
}

func TestReaderSkipOrdinaryAttrValuesValidatesReferencesAndUTF8(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		valid bool
	}{
		{name: "decoded references", input: []byte(`<r value="a&amp;b&#xA;"/>`), valid: true},
		{name: "invalid character reference", input: []byte(`<r value="&#x0;"/>`)},
		{name: "invalid XML character", input: []byte("<r value=\"\x01\"/>")},
		{name: "invalid UTF-8", input: []byte("<r value=\"\xff\"/>")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reader Reader
			if err := reader.Reset(bytes.NewReader(tt.input), Config{
				LazyAttrValues:         true,
				SkipOrdinaryAttrValues: true,
			}); err != nil {
				t.Fatal(err)
			}
			tok, err := reader.Next()
			if !tt.valid {
				if err == nil {
					t.Fatal("Next() accepted invalid skipped attribute value")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if raw, ok := tok.Start.Attr[0].RawValue(); ok || raw != nil {
				t.Fatalf("ordinary attribute RawValue() = %q, %v; want no retained value", raw, ok)
			}
			handle, _, err := reader.Start()
			if err != nil {
				t.Fatal(err)
			}
			_ = mustNextToken(t, &reader)
			if err := reader.End(handle); err != nil {
				t.Fatal(err)
			}
			if _, err := reader.Next(); !errors.Is(err, io.EOF) {
				t.Fatalf("Next() after root = %v, want EOF", err)
			}
			if err := reader.Complete(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReaderSkipOrdinaryAttrValuesRetainsNamespaceInputs(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r xmlns:p="urn:&amp;test" xml:space="preserve" value="&#x41;"><p:c/></r>`), Config{
		LazyAttrValues:         true,
		SkipOrdinaryAttrValues: true,
	}); err != nil {
		t.Fatal(err)
	}
	root := mustNextToken(t, &reader)
	if raw, ok := root.Start.Attr[2].RawValue(); ok || raw != nil {
		t.Fatalf("ordinary attribute RawValue() = %q, %v; want no retained value", raw, ok)
	}
	if raw, ok := root.Start.Attr[1].RawValue(); !ok || string(raw) != "preserve" {
		t.Fatalf("xml:space RawValue() = %q, %v; want preserve", raw, ok)
	}
	rootHandle, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if uri, ok := reader.Lookup("p"); !ok || uri != "urn:&test" {
		t.Fatalf("Lookup(p) = %q, %v; want decoded namespace URI", uri, ok)
	}
	_ = mustNextToken(t, &reader)
	childHandle, childElement, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if childElement.Name != (xml.Name{Space: "urn:&test", Local: "c"}) {
		t.Fatalf("child expanded name = %+v", childElement.Name)
	}
	_ = mustNextToken(t, &reader)
	if err := reader.End(childHandle); err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	if err := reader.End(rootHandle); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("Next() after root = %v, want EOF", err)
	}
	if err := reader.Complete(); err != nil {
		t.Fatal(err)
	}
}

func TestReaderSkipOrdinaryAttrValuesStillChecksExpandedDuplicates(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r xmlns:p="urn" p:value="one" p:value="two"/>`), Config{
		LazyAttrValues:         true,
		SkipOrdinaryAttrValues: true,
	}); err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	if _, _, err := reader.Start(); err == nil {
		t.Fatal("Start() accepted duplicate expanded attributes")
	} else {
		var boundaryErr *Error
		if !errors.As(err, &boundaryErr) || boundaryErr.Kind != ErrorNamespace {
			t.Fatalf("Start() error = %T %v, want namespace boundary error", err, err)
		}
	}
}

func TestReaderInternBytesReusesShortValues(t *testing.T) {
	var reader Reader
	raw := []byte("value")
	if got := reader.InternBytes(raw); got != "value" {
		t.Fatalf("InternBytes() = %q, want value", got)
	}
	var got string
	if allocs := testing.AllocsPerRun(100, func() {
		got = reader.InternBytes(raw)
	}); allocs != 0 {
		t.Fatalf("InternBytes() allocations = %v, want 0 after warmup", allocs)
	}
	if got != "value" {
		t.Fatalf("InternBytes() after warmup = %q, want value", got)
	}
	if err := reader.Reset(strings.NewReader(`<r/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	if got := reader.InternBytes(raw); got != "value" {
		t.Fatalf("InternBytes() after Reset = %q, want value", got)
	}
}

func TestReaderInternBytesOwnsInput(t *testing.T) {
	var reader Reader
	raw := []byte("owned")
	retained := reader.InternBytes(raw)
	raw[0] = 'x'
	if retained != "owned" {
		t.Fatalf("retained spelling after input mutation = %q, want owned", retained)
	}
	if got := reader.InternBytes([]byte("owned")); got != "owned" {
		t.Fatalf("InternBytes() after input mutation = %q, want owned", got)
	}
}

func TestReaderInternBytesMapLookupAfterRecentEviction(t *testing.T) {
	var reader Reader
	spellings := make([][]byte, recentCacheEntries+2)
	for i := range spellings {
		spellings[i] = []byte("spelling-" + strconv.Itoa(i))
		if got := reader.InternBytes(spellings[i]); got != string(spellings[i]) {
			t.Fatalf("InternBytes(%q) = %q", spellings[i], got)
		}
	}

	var got string
	allocs := testing.AllocsPerRun(100, func() {
		// Eight intervening values evict spelling-0 from the recent ring, so
		// every measured lookup must use the bounded map.
		got = reader.InternBytes(spellings[0])
		for i := 1; i <= recentCacheEntries; i++ {
			reader.InternBytes(spellings[i])
		}
	})
	if allocs != 0 {
		t.Fatalf("warmed map lookup allocations = %v, want 0", allocs)
	}
	if got != "spelling-0" {
		t.Fatalf("warmed map lookup = %q, want spelling-0", got)
	}
}

func TestReaderInternBytesOwnershipSurvivesResetAndDetach(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	raw := []byte("stable")
	retained := reader.InternBytes(raw)
	raw[0] = 'x'
	if err := reader.Reset(strings.NewReader(`<r/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	if got := reader.InternBytes([]byte("stable")); got != "stable" {
		t.Fatalf("InternBytes() after Reset = %q, want stable", got)
	}

	reader.Detach()
	if retained != "stable" {
		t.Fatalf("retained spelling after Reset and Detach = %q, want stable", retained)
	}
	if got := reader.InternBytes([]byte("stable")); got != "stable" {
		t.Fatalf("InternBytes() after Detach = %q, want stable", got)
	}
}

func TestReaderRequiresExplicitTokenTransitions(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r><c/></r>`), Config{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := reader.Start(); !errors.Is(err, ErrPendingStart) {
		t.Fatalf("Start without token = %v, want %v", err, ErrPendingStart)
	}
	_ = mustNextToken(t, &reader)
	if _, err := reader.Next(); !errors.Is(err, ErrPendingStart) {
		t.Fatalf("Next before Start = %v, want %v", err, ErrPendingStart)
	}
	rootFrame, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, startErr := reader.Start(); !errors.Is(startErr, ErrPendingStart) {
		t.Fatalf("repeated Start = %v, want %v", startErr, ErrPendingStart)
	}
	_ = mustNextToken(t, &reader)
	childFrame, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	if _, err := reader.Next(); !errors.Is(err, ErrPendingEnd) {
		t.Fatalf("Next before MatchEnd = %v, want %v", err, ErrPendingEnd)
	}
	if err := reader.CommitEnd(childFrame); !errors.Is(err, ErrPendingEnd) {
		t.Fatalf("CommitEnd before MatchEnd = %v, want %v", err, ErrPendingEnd)
	}
	if err := reader.MatchEnd(childFrame); err != nil {
		t.Fatal(err)
	}
	if err := reader.CommitEnd(childFrame); err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	if err := reader.MatchEnd(rootFrame); err != nil {
		t.Fatal(err)
	}
	if err := reader.CommitEnd(rootFrame); err != nil {
		t.Fatal(err)
	}
}

func TestReaderKeepsSyntaxFrameAfterAdvanceForRecovery(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r><child/></r>`), Config{}); err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	rootFrame, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if abortErr := reader.AbortStart(rootFrame); abortErr != nil {
		t.Fatal(abortErr)
	}
	if reader.Depth() != 0 {
		t.Fatalf("Depth after immediate AbortStart = %d, want 0", reader.Depth())
	}

	if resetErr := reader.Reset(strings.NewReader(`<r><child/></r>`), Config{}); resetErr != nil {
		t.Fatal(resetErr)
	}
	_ = mustNextToken(t, &reader)
	rootFrame, _, err = reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	if err := reader.AbortStart(rootFrame); !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("AbortStart after advance = %v, want %v", err, ErrInvalidFrame)
	}
	if reader.Depth() != 1 {
		t.Fatalf("Depth after semantic recovery frame = %d, want 1", reader.Depth())
	}
}

func mustNextToken(t *testing.T, reader *Reader) *Token {
	t.Helper()
	tok, err := reader.Next()
	if err != nil {
		t.Fatalf("Next() = %v", err)
	}
	return tok
}
