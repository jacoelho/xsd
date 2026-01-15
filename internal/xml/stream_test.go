package xsdxml

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/pkg/xmltext"
)

func TestStreamDecoderNamespaceLookup(t *testing.T) {
	xmlData := `<root xmlns="urn:root" xmlns:r="urn:root2">
<child xmlns:p="urn:child" p:attr="v"/></root>`
	dec, err := NewStreamDecoder(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStreamDecoder() error = %v", err)
	}

	event, err := nextStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if got := event.Name.Namespace; got != types.NamespaceURI("urn:root") {
		t.Fatalf("root namespace = %q, want %q", got, "urn:root")
	}
	if event.ScopeDepth != 0 {
		t.Fatalf("root scope depth = %d, want 0", event.ScopeDepth)
	}
	if ns, ok := dec.LookupNamespace("", event.ScopeDepth); !ok || ns != "urn:root" {
		t.Fatalf("default namespace = %q (ok=%v), want urn:root", ns, ok)
	}

	event, err = nextStartEvent(dec)
	if err != nil {
		t.Fatalf("next start child error = %v", err)
	}
	if event.ScopeDepth != 1 {
		t.Fatalf("child scope depth = %d, want 1", event.ScopeDepth)
	}
	if ns, ok := dec.LookupNamespace("p", event.ScopeDepth); !ok || ns != "urn:child" {
		t.Fatalf("prefix p = %q (ok=%v), want urn:child", ns, ok)
	}
	if ns, ok := dec.LookupNamespace("", event.ScopeDepth); !ok || ns != "urn:root" {
		t.Fatalf("default namespace = %q (ok=%v), want urn:root", ns, ok)
	}

	foundXMLNS := false
	for _, attr := range event.Attrs {
		if attr.NamespaceURI() == XMLNSNamespace && attr.LocalName() == "p" {
			foundXMLNS = true
			break
		}
	}
	if !foundXMLNS {
		t.Fatalf("expected xmlns:p attribute in child attributes")
	}
}

func TestStreamDecoderDefaultNamespaceUndeclare(t *testing.T) {
	xmlData := `<root xmlns="urn:root"><child xmlns=""><grand/></child></root>`
	dec, err := NewStreamDecoder(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStreamDecoder() error = %v", err)
	}

	event, err := nextStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if ns, ok := dec.LookupNamespace("", event.ScopeDepth); !ok || ns != "urn:root" {
		t.Fatalf("root default namespace = %q (ok=%v), want urn:root", ns, ok)
	}

	event, err = nextStartEvent(dec)
	if err != nil {
		t.Fatalf("next start child error = %v", err)
	}
	if ns, ok := dec.LookupNamespace("", event.ScopeDepth); !ok || ns != "" {
		t.Fatalf("child default namespace = %q (ok=%v), want empty", ns, ok)
	}

	event, err = nextStartEvent(dec)
	if err != nil {
		t.Fatalf("next start grand error = %v", err)
	}
	if ns, ok := dec.LookupNamespace("", event.ScopeDepth); !ok || ns != "" {
		t.Fatalf("grand default namespace = %q (ok=%v), want empty", ns, ok)
	}
}

func TestStreamDecoderSkipSubtree(t *testing.T) {
	xmlData := `<root><skip><inner/><inner2/></skip><after/></root>`
	dec, err := NewStreamDecoder(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStreamDecoder() error = %v", err)
	}

	event, err := nextStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if event.Name.Local != "root" {
		t.Fatalf("first element = %q, want root", event.Name.Local)
	}

	event, err = nextStartEvent(dec)
	if err != nil {
		t.Fatalf("next start skip error = %v", err)
	}
	if event.Name.Local != "skip" {
		t.Fatalf("second element = %q, want skip", event.Name.Local)
	}
	err = dec.SkipSubtree()
	if err != nil {
		t.Fatalf("SkipSubtree() error = %v", err)
	}

	event, err = nextStartEvent(dec)
	if err != nil {
		t.Fatalf("next start after error = %v", err)
	}
	if event.Name.Local != "after" {
		t.Fatalf("after element = %q, want after", event.Name.Local)
	}
}

func TestStreamDecoderAttrValueCopy(t *testing.T) {
	xmlData := `<doc><root attr="foo"/><next attr="bar"/></doc>`
	dec, err := NewStreamDecoder(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStreamDecoder() error = %v", err)
	}

	_, err = nextStartEvent(dec)
	if err != nil {
		t.Fatalf("next start doc error = %v", err)
	}

	event, err := nextStartEvent(dec)
	if err != nil {
		t.Fatalf("next start root error = %v", err)
	}
	if len(event.Attrs) != 1 {
		t.Fatalf("root attr count = %d, want 1", len(event.Attrs))
	}
	value := event.Attrs[0].Value()

	event, err = nextStartEvent(dec)
	if err != nil {
		t.Fatalf("next start next error = %v", err)
	}
	if len(event.Attrs) != 1 {
		t.Fatalf("next attr count = %d, want 1", len(event.Attrs))
	}
	if value != "foo" {
		t.Fatalf("root attr value = %q, want foo", value)
	}
}

func nextStartEvent(dec *StreamDecoder) (Event, error) {
	for {
		event, err := dec.Next()
		if err != nil {
			return Event{}, err
		}
		if event.Kind == EventStartElement {
			return event, nil
		}
	}
}

func TestStreamDecoderEOF(t *testing.T) {
	dec, err := NewStreamDecoder(strings.NewReader(`<root/>`))
	if err != nil {
		t.Fatalf("NewStreamDecoder() error = %v", err)
	}
	for {
		_, err := dec.Next()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
	}
}

func TestStreamDecoderUnboundPrefixElement(t *testing.T) {
	xmlData := `<root><p:child/></root>`
	dec, err := NewStreamDecoder(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStreamDecoder() error = %v", err)
	}
	for {
		_, err := dec.Next()
		if err == nil {
			continue
		}
		var syntax *xmltext.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("Next() error type = %T, want *xmltext.SyntaxError", err)
		}
		if !errors.Is(err, errUnboundPrefix) {
			t.Fatalf("Next() error = %v, want unbound prefix", err)
		}
		return
	}
}

func TestStreamDecoderUnboundPrefixAttr(t *testing.T) {
	xmlData := `<root><child p:attr="v"/></root>`
	dec, err := NewStreamDecoder(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("NewStreamDecoder() error = %v", err)
	}
	for {
		_, err := dec.Next()
		if err == nil {
			continue
		}
		var syntax *xmltext.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("Next() error type = %T, want *xmltext.SyntaxError", err)
		}
		if !errors.Is(err, errUnboundPrefix) {
			t.Fatalf("Next() error = %v, want unbound prefix", err)
		}
		return
	}
}
