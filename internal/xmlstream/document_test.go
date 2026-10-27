package xmlstream

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/vocab"
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
	if root != (xml.Name{Local: "r"}) {
		t.Fatalf("root expanded name = %+v", root)
	}
	if uri, ok := reader.Lookup("p"); !ok || uri != "urn:test" {
		t.Fatalf("Lookup(p) = %q, %v", uri, ok)
	}
	if got := rootToken.Start.Attr[0].Name; got != (xml.Name{Space: vocab.XMLNSNamespaceURI, Local: "p"}) {
		t.Fatalf("namespace attribute = %+v", got)
	}
	context := reader.Context()

	childToken := mustNextToken(t, &reader)
	childFrame, child, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if child != (xml.Name{Space: "urn:test", Local: "child"}) {
		t.Fatalf("child expanded name = %+v", child)
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
	if root != (xml.Name{Space: "urn:root", Local: "root"}) {
		t.Fatalf("root expanded name = %+v", root)
	}

	childToken := mustNextToken(t, &reader)
	if childToken.Start.Name.Local != "child" {
		t.Fatalf("child lexical name = %+v", childToken.Start.Name)
	}
	childFrame, child, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if child != (xml.Name{Space: "urn:child", Local: "child"}) {
		t.Fatalf("child expanded name = %+v", child)
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
	if sibling != (xml.Name{Space: "urn:root", Local: "sibling"}) {
		t.Fatalf("sibling expanded name = %+v", sibling)
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

	assertPublicationTokenZero(t, end)

	if err := reader.Reset(strings.NewReader(`<fresh/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	old := mustNextToken(t, &reader)
	if err := reader.Reset(strings.NewReader(`<new/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	assertPublicationTokenZero(t, old)
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

func TestReaderNamespaceDeclarationIdentityDoesNotCollideWithRelativeAttribute(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r xmlns="xmlns" xmlns:p="xmlns" p:value="v"/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	tok := mustNextToken(t, &reader)
	frame, element, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if element != (xml.Name{Space: "xmlns", Local: "r"}) {
		t.Fatalf("relative default element namespace = %+v", element)
	}
	want := []xml.Name{
		{Space: vocab.XMLNSNamespaceURI, Local: vocab.XMLNSPrefix},
		{Space: vocab.XMLNSNamespaceURI, Local: "p"},
		{Space: "xmlns", Local: "value"},
	}
	if len(tok.Start.Attr) != len(want) {
		t.Fatalf("expanded attribute count = %d, want %d", len(tok.Start.Attr), len(want))
	}
	for i, attr := range tok.Start.Attr {
		if attr.Name != want[i] {
			t.Fatalf("expanded attribute %d = %+v, want %+v", i, attr.Name, want[i])
		}
	}
	_ = mustNextToken(t, &reader)
	if err := reader.End(frame); err != nil {
		t.Fatal(err)
	}
	if err := reader.Complete(); err != nil {
		t.Fatal(err)
	}
}

func TestReaderNamespaceAdmissionRollbackKeepsLexicalAttributes(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<r xmlns:p="xmlns" p:value="v" q:later="x"/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	tok := mustNextToken(t, &reader)
	if _, _, err := reader.Start(); err == nil {
		t.Fatal("Start() accepted an unbound later attribute")
	} else {
		var boundaryErr *Error
		if !errors.As(err, &boundaryErr) || boundaryErr.Kind != ErrorNamespace {
			t.Fatalf("Start() error = %T %v, want namespace boundary error", err, err)
		}
	}
	if reader.Depth() != 0 {
		t.Fatalf("Depth after failed start = %d, want 0", reader.Depth())
	}
	if _, ok := reader.Lookup("p"); ok {
		t.Fatal("failed start retained namespace binding")
	}
	wantLexical := []xml.Name{
		{Space: vocab.XMLNSPrefix, Local: "p"},
		{Space: "p", Local: "value"},
		{Space: "q", Local: "later"},
	}
	for i, attr := range tok.Start.Attr {
		if attr.Name != wantLexical[i] {
			t.Fatalf("failed-start attribute %d = %+v, want lexical %+v", i, attr.Name, wantLexical[i])
		}
	}

	if err := reader.Reset(strings.NewReader(`<fresh/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	frame, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	if err := reader.End(frame); err != nil {
		t.Fatal(err)
	}
	if err := reader.Complete(); err != nil {
		t.Fatal(err)
	}
}

func TestReaderRejectsReservedXMLNSNamespaceBindings(t *testing.T) {
	for _, input := range []string{
		`<r xmlns:p="` + vocab.XMLNSNamespaceURI + `"/>`,
		`<r xmlns="` + vocab.XMLNSNamespaceURI + `"/>`,
	} {
		t.Run(input, func(t *testing.T) {
			var reader Reader
			if err := reader.Reset(strings.NewReader(input), Config{}); err != nil {
				t.Fatal(err)
			}
			_ = mustNextToken(t, &reader)
			if _, _, err := reader.Start(); err == nil {
				t.Fatal("Start() accepted the reserved XMLNS namespace URI")
			} else {
				var boundaryErr *Error
				if !errors.As(err, &boundaryErr) || boundaryErr.Kind != ErrorNamespace {
					t.Fatalf("Start() error = %T %v, want namespace boundary error", err, err)
				}
			}
			if reader.Depth() != 0 {
				t.Fatalf("Depth after rejected binding = %d, want 0", reader.Depth())
			}
		})
	}
}

func TestReaderRejectsDuplicateNamespaceDeclarationAndExpandedAlias(t *testing.T) {
	for _, input := range []string{
		`<r xmlns:p="urn:one" xmlns:p="urn:two"/>`,
		`<r xmlns:p="xmlns" xmlns:q="xmlns" p:value="one" q:value="two"/>`,
	} {
		t.Run(input, func(t *testing.T) {
			var reader Reader
			if err := reader.Reset(strings.NewReader(input), Config{}); err != nil {
				t.Fatal(err)
			}
			_ = mustNextToken(t, &reader)
			if _, _, err := reader.Start(); err == nil {
				t.Fatal("Start() accepted duplicate expanded names")
			} else {
				var boundaryErr *Error
				if !errors.As(err, &boundaryErr) || boundaryErr.Kind != ErrorNamespace {
					t.Fatalf("Start() error = %T %v, want namespace boundary error", err, err)
				}
			}
			if reader.Depth() != 0 {
				t.Fatalf("Depth after duplicate = %d, want 0", reader.Depth())
			}
		})
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
	if element.Local != "r" {
		t.Fatalf("root expanded name = %+v", element)
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
	t.Parallel()
	for _, mode := range []struct {
		name string
		skip bool
	}{{"retained", false}, {"skipped", true}} {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			input := &scriptedReader{reads: []scriptedRead{
				{data: []byte("<r xmlns:p=\"urn:p\" a='x&am")},
				{data: []byte("p;\r")},
				{data: []byte("\n\xc3")},
				{data: []byte("\xa9' xmlns=\"\" xml:space=\"preserve\" b=\"\"><p:c/></r>")},
			}}
			var reader Reader
			t.Cleanup(reader.Detach)
			if err := reader.Reset(input, Config{LazyAttrValues: true, SkipOrdinaryAttrValues: mode.skip}); err != nil {
				t.Fatal(err)
			}
			token := mustNextToken(t, &reader)
			if token.Kind != KindStart || len(token.Start.Attr) != 5 {
				t.Fatalf("root token = %+v, want start with five attributes", token)
			}
			for i, want := range [5]string{"urn:p", "x& é", "", "preserve", ""} {
				raw, retained := token.Start.Attr[i].RawValue()
				if mode.skip && (i == 1 || i == 4) {
					if retained || raw != nil {
						t.Fatalf("ordinary attribute %d = %q, %v, want omitted", i, raw, retained)
					}
					continue
				}
				if want == "" {
					text, ok := reader.MaterializeValue(&token.Start.Attr[i])
					if retained || raw != nil || !ok || text != "" {
						t.Fatalf("empty attribute %d = %q/%v, %q/%v", i, raw, retained, text, ok)
					}
					continue
				}
				if !retained || string(raw) != want {
					t.Fatalf("attribute %d = %q, %v, want retained %q", i, raw, retained, want)
				}
			}
			root, _, err := reader.Start()
			if err != nil {
				t.Fatal(err)
			}
			_ = mustNextToken(t, &reader)
			child, element, err := reader.Start()
			if err != nil {
				t.Fatal(err)
			}
			if element != (xml.Name{Space: "urn:p", Local: "c"}) {
				t.Fatalf("child = %+v, want {urn:p}c", element)
			}
			for _, handle := range [2]Handle{child, root} {
				if token := mustNextToken(t, &reader); token.Kind != KindEnd {
					t.Fatalf("token = %+v, want end", token)
				}
				if err := reader.End(handle); err != nil {
					t.Fatal(err)
				}
			}
			if token, err := reader.Next(); token != nil || !errors.Is(err, io.EOF) {
				t.Fatalf("after root = %v, %v, want EOF", token, err)
			}
			if err := reader.Complete(); err != nil {
				t.Fatal(err)
			}
		})
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
	t.Parallel()
	for _, mode := range []struct {
		name string
		skip bool
	}{{"retained", false}, {"skipped", true}} {
		for _, test := range []struct {
			name  string
			limit int64
			valid bool
		}{{"exact", 5, true}, {"below", 4, false}} {
			t.Run(mode.name+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				var reader Reader
				t.Cleanup(reader.Detach)
				config := Config{LazyAttrValues: true, SkipOrdinaryAttrValues: mode.skip, Limits: Limits{MaxTokenBytes: test.limit}}
				if err := reader.Reset(strings.NewReader(`<r a="abc"/>`), config); err != nil {
					t.Fatal(err)
				}
				token, err := reader.Next()
				if test.valid {
					if err != nil || token == nil || token.Kind != KindStart {
						t.Fatalf("exact admission = %v, %v, want start", token, err)
					}
				} else {
					boundary, ok := errors.AsType[*Error](err)
					if token != nil || !ok || boundary.Kind != ErrorLimit || !IsTokenLimit(err) || boundary.Line != 1 || boundary.Column != 6 {
						t.Fatalf("below admission = %v, %#v, want token limit at 1:6", token, boundary)
					}
				}
				assertAttributeReaderReset(t, &reader, config)
			})
		}
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
	if childElement != (xml.Name{Space: "urn:&test", Local: "c"}) {
		t.Fatalf("child expanded name = %+v", childElement)
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

func TestReaderInternBytesRotatingVocabulary(t *testing.T) {
	var reader Reader
	spellings := make([][]byte, 10)
	for i := range spellings {
		spellings[i] = []byte("spelling-" + strconv.Itoa(i))
		if got := reader.InternBytes(spellings[i]); got != string(spellings[i]) {
			t.Fatalf("InternBytes(%q) = %q", spellings[i], got)
		}
	}

	var got string
	allocs := testing.AllocsPerRun(100, func() {
		for _, spelling := range spellings {
			got = reader.InternBytes(spelling)
		}
	})
	if allocs != 0 {
		t.Fatalf("warmed map lookup allocations = %v, want 0", allocs)
	}
	if got != "spelling-9" {
		t.Fatalf("warmed rotating lookup = %q, want spelling-9", got)
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
	startToken := mustNextToken(t, &reader)
	wantStart := snapshotPublicationToken(startToken)
	if _, err := reader.Next(); !errors.Is(err, ErrPendingStart) {
		t.Fatalf("Next before Start = %v, want %v", err, ErrPendingStart)
	}
	assertPublicationSnapshot(t, startToken, wantStart)
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
	endToken := mustNextToken(t, &reader)
	wantEnd := snapshotPublicationToken(endToken)
	if _, err := reader.Next(); !errors.Is(err, ErrPendingEnd) {
		t.Fatalf("Next before MatchEnd = %v, want %v", err, ErrPendingEnd)
	}
	assertPublicationSnapshot(t, endToken, wantEnd)
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

func TestReaderAttributeScannerTerminalFailures(t *testing.T) {
	t.Parallel()
	sourceErr := errors.New("attribute source failed")
	for _, mode := range []struct {
		name string
		skip bool
	}{{"retained", false}, {"skipped", true}} {
		for _, test := range []struct {
			name       string
			terminal   error
			wantSource bool
			wantEOF    bool
		}{
			{name: "EOF", terminal: io.EOF},
			{name: "source", terminal: sourceErr, wantSource: true},
			{name: "joined", terminal: errors.Join(io.EOF, sourceErr), wantSource: true, wantEOF: true},
		} {
			t.Run(mode.name+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				input := &scriptedReader{reads: []scriptedRead{
					{data: []byte(`<r a="ab`)},
					{data: []byte("c"), err: test.terminal},
				}}
				var reader Reader
				t.Cleanup(reader.Detach)
				config := Config{LazyAttrValues: true, SkipOrdinaryAttrValues: mode.skip}
				if err := reader.Reset(input, config); err != nil {
					t.Fatal(err)
				}
				token, err := reader.Next()
				boundary, ok := errors.AsType[*Error](err)
				if token != nil || !ok || boundary.Kind != ErrorSyntax || boundary.Line != 1 || boundary.Column != 9 {
					t.Fatalf("Next() = %v, %#v, want syntax failure at 1:9", token, boundary)
				}
				if !test.wantSource && err.Error() != "unexpected EOF in attribute value" {
					t.Fatalf("error = %q, want attribute EOF context", err)
				}
				if errors.Is(err, sourceErr) != test.wantSource || errors.Is(err, io.EOF) != test.wantEOF {
					t.Fatalf("error causes = %v, want source=%v EOF=%v", err, test.wantSource, test.wantEOF)
				}
				assertAttributeReaderReset(t, &reader, config)
			})
		}
	}
}

func assertAttributeReaderReset(t *testing.T, reader *Reader, config Config) {
	t.Helper()
	reader.Detach()
	if err := reader.Reset(strings.NewReader(`<ok/>`), config); err != nil {
		t.Fatal(err)
	}
	if token := mustNextToken(t, reader); token.Kind != KindStart || token.Start.Name.Local != "ok" {
		t.Fatalf("reset token = %+v, want ok start", token)
	}
	handle, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if token := mustNextToken(t, reader); token.Kind != KindEnd {
		t.Fatalf("reset token = %+v, want end", token)
	}
	if err := reader.End(handle); err != nil {
		t.Fatal(err)
	}
	if token, err := reader.Next(); token != nil || !errors.Is(err, io.EOF) {
		t.Fatalf("reset completion = %v, %v, want EOF", token, err)
	}
	if err := reader.Complete(); err != nil {
		t.Fatal(err)
	}
}

func TestReaderAttributeErrorsPreserveSyntaxAndSourceFailure(t *testing.T) {
	t.Parallel()
	sourceErr := errors.New("attribute CR lookahead failed")
	for _, mode := range []struct {
		name string
		skip bool
	}{{"retained", false}, {"skipped", true}} {
		for _, test := range []struct {
			name       string
			reads      []scriptedRead
			message    string
			wantSource bool
			line       int
			column     int
			limit      int64
		}{
			{
				name:    "forbidden-markup",
				reads:   []scriptedRead{{data: []byte(`<r a="x<"/>`)}},
				message: "attribute value cannot contain <",
				line:    1,
				column:  8,
			},
			{
				name: "CR-source-before-token-limit",
				reads: []scriptedRead{
					{data: []byte(`<r a="x`)},
					{data: []byte("\r"), err: sourceErr},
				},
				wantSource: true,
				line:       2,
				limit:      3,
			},
		} {
			t.Run(mode.name+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				var reader Reader
				t.Cleanup(reader.Detach)
				config := Config{LazyAttrValues: true, SkipOrdinaryAttrValues: mode.skip, Limits: Limits{MaxTokenBytes: test.limit}}
				if err := reader.Reset(&scriptedReader{reads: test.reads}, config); err != nil {
					t.Fatal(err)
				}
				token, err := reader.Next()
				boundary, ok := errors.AsType[*Error](err)
				if token != nil || !ok || boundary.Kind != ErrorSyntax || boundary.Line != test.line || boundary.Column != test.column {
					t.Fatalf("Next() = %v, %#v, want syntax failure at %d:%d", token, boundary, test.line, test.column)
				}
				if test.message != "" && err.Error() != test.message {
					t.Fatalf("error = %q, want %q", err, test.message)
				}
				if errors.Is(err, sourceErr) != test.wantSource || errors.Is(err, io.EOF) {
					t.Fatalf("error causes = %v, want source=%v and no EOF", err, test.wantSource)
				}
				assertAttributeReaderReset(t, &reader, config)
			})
		}
	}
}

func TestReaderStartLocationUsesAdmittedNamespacesAndTokenPosition(t *testing.T) {
	for _, test := range []struct {
		name, start string
	}{
		{"prefixed", `<p:root xmlns:p="urn:root"/>`},
		{"default", `<root xmlns="urn:root"/>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var reader Reader
			if err := reader.Reset(strings.NewReader("\n  "+test.start), Config{}); err != nil {
				t.Fatal(err)
			}
			mustNextToken(t, &reader)
			mustNextToken(t, &reader)
			if _, _, err := reader.Start(); err != nil {
				t.Fatal(err)
			}
			want := StartLocation{Name: xml.Name{Space: "urn:root", Local: "root"}, Line: 2, Column: 3}
			for range 2 {
				got, err := reader.StartLocation()
				if err != nil || got != want {
					t.Fatalf("StartLocation = %+v, %v; want %+v", got, err, want)
				}
			}
		})
	}
}

func TestReaderStartLocationRejectsUnavailableStart(t *testing.T) {
	for _, state := range []string{"before admission", "failed admission", "advance", "abort", "reset", "detach"} {
		t.Run(state, func(t *testing.T) {
			var reader Reader
			input := `<root/>`
			if state == "failed admission" {
				input = `<p:root/>`
			}
			if err := reader.Reset(strings.NewReader(input), Config{}); err != nil {
				t.Fatal(err)
			}
			mustNextToken(t, &reader)
			if state != "before admission" {
				handle, _, err := reader.Start()
				if state == "failed admission" {
					if err == nil {
						t.Fatal("unbound prefix accepted")
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					switch state {
					case "advance":
						mustNextToken(t, &reader)
					case "abort":
						if err := reader.AbortStart(handle); err != nil {
							t.Fatal(err)
						}
					case "reset":
						if err := reader.Reset(strings.NewReader(`<other/>`), Config{}); err != nil {
							t.Fatal(err)
						}
					case "detach":
						reader.Detach()
					}
				}
			}
			_, err := reader.StartLocation()
			cause := ErrPendingStart
			if state == "detach" {
				cause = ErrReaderState
			}
			boundary, ok := errors.AsType[*Error](err)
			if !ok || boundary.Kind != ErrorState || !errors.Is(err, cause) {
				t.Fatalf("StartLocation error = %v, want state error wrapping %v", err, cause)
			}
		})
	}
}
