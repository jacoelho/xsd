package xsdxml

import (
	"encoding/xml"
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/types"
)

// EventKind identifies the kind of streaming XML event.
type EventKind int

const (
	EventStartElement EventKind = iota
	EventEndElement
	EventCharData
)

// elementID is a monotonic identifier assigned per document.
type elementID uint64

// Event represents a single streaming XML token.
// Attrs and Text are only valid until the next Next call.
type Event struct {
	Kind       EventKind
	Name       types.QName
	Attrs      []Attr
	Text       []byte
	Line       int
	Column     int
	ID         elementID
	ScopeDepth int
}

type nsScope struct {
	defaultNS  string
	defaultSet bool
	prefixes   map[string]string
}

type nsStack struct {
	scopes []nsScope
}

func (s *nsStack) push(scope nsScope) int {
	s.scopes = append(s.scopes, scope)
	return len(s.scopes) - 1
}

func (s *nsStack) pop() {
	if len(s.scopes) == 0 {
		return
	}
	s.scopes = s.scopes[:len(s.scopes)-1]
}

func (s *nsStack) depth() int {
	return len(s.scopes)
}

func (s *nsStack) lookup(prefix string, depth int) (string, bool) {
	if prefix == "xml" {
		return XMLNamespace, true
	}
	if prefix == "" {
		for i := depth; i >= 0; i-- {
			if i >= len(s.scopes) {
				continue
			}
			scope := s.scopes[i]
			if scope.defaultSet {
				return scope.defaultNS, true
			}
		}
		return "", true
	}
	for i := depth; i >= 0; i-- {
		if i >= len(s.scopes) {
			continue
		}
		scope := s.scopes[i]
		if ns, ok := scope.prefixes[prefix]; ok {
			return ns, true
		}
	}
	return "", false
}

// StreamDecoder provides a streaming XML event interface with namespace tracking.
type StreamDecoder struct {
	dec        *xml.Decoder
	ns         nsStack
	attrBuf    []Attr
	textBuf    []byte
	nextID     elementID
	pendingPop bool
}

// NewStreamDecoder creates a new streaming decoder for the reader.
func NewStreamDecoder(r io.Reader) (*StreamDecoder, error) {
	if r == nil {
		return nil, fmt.Errorf("nil XML reader")
	}
	dec := xml.NewDecoder(r)
	if dec == nil {
		return nil, fmt.Errorf("nil XML decoder")
	}
	return &StreamDecoder{dec: dec}, nil
}

// Next returns the next XML event.
func (d *StreamDecoder) Next() (Event, error) {
	if d == nil || d.dec == nil {
		return Event{}, fmt.Errorf("nil XML decoder")
	}
	if d.pendingPop {
		d.ns.pop()
		d.pendingPop = false
	}

	for {
		tok, err := d.dec.Token()
		if err != nil {
			return Event{}, err
		}
		line, column := d.dec.InputPos()

		switch t := tok.(type) {
		case xml.StartElement:
			scope := d.collectNamespaceScope(t.Attr)
			d.ns.push(scope)

			d.attrBuf = d.attrBuf[:0]
			for _, a := range t.Attr {
				namespace := normalizeAttrNamespace(a.Name.Space, a.Name.Local)
				d.attrBuf = append(d.attrBuf, Attr{
					namespace: namespace,
					local:     a.Name.Local,
					value:     a.Value,
				})
			}

			id := d.nextID
			d.nextID++
			return Event{
				Kind:       EventStartElement,
				Name:       types.QName{Namespace: types.NamespaceURI(t.Name.Space), Local: t.Name.Local},
				Attrs:      d.attrBuf,
				Line:       line,
				Column:     column,
				ID:         id,
				ScopeDepth: d.ns.depth() - 1,
			}, nil

		case xml.EndElement:
			d.pendingPop = true
			return Event{
				Kind:       EventEndElement,
				Name:       types.QName{Namespace: types.NamespaceURI(t.Name.Space), Local: t.Name.Local},
				Line:       line,
				Column:     column,
				ScopeDepth: d.ns.depth() - 1,
			}, nil

		case xml.CharData:
			d.textBuf = append(d.textBuf[:0], t...)
			return Event{
				Kind:       EventCharData,
				Text:       d.textBuf,
				Line:       line,
				Column:     column,
				ScopeDepth: d.ns.depth() - 1,
			}, nil
		}
	}
}

// SkipSubtree skips the current element subtree after a StartElement event.
func (d *StreamDecoder) SkipSubtree() error {
	if d == nil || d.dec == nil {
		return fmt.Errorf("nil XML decoder")
	}
	if d.pendingPop {
		d.ns.pop()
		d.pendingPop = false
	}
	if err := d.dec.Skip(); err != nil {
		return err
	}
	d.ns.pop()
	return nil
}

// CurrentPos returns the line and column of the most recent token.
func (d *StreamDecoder) CurrentPos() (line, column int) {
	if d == nil || d.dec == nil {
		return 0, 0
	}
	return d.dec.InputPos()
}

// LookupNamespace resolves a prefix at a given scope depth.
func (d *StreamDecoder) LookupNamespace(prefix string, depth int) (string, bool) {
	if d == nil {
		return "", false
	}
	return d.ns.lookup(prefix, depth)
}

func (d *StreamDecoder) collectNamespaceScope(attrs []xml.Attr) nsScope {
	scope := nsScope{}
	for _, a := range attrs {
		if isDefaultNamespaceDecl(a.Name.Space, a.Name.Local) {
			scope.defaultNS = a.Value
			scope.defaultSet = true
			continue
		}
		if isPrefixedNamespaceDecl(a.Name.Space) && a.Name.Local != "" {
			prefix := a.Name.Local
			if prefix == "xml" || prefix == "xmlns" {
				continue
			}
			if scope.prefixes == nil {
				scope.prefixes = make(map[string]string, 1)
			}
			scope.prefixes[prefix] = a.Value
		}
	}
	return scope
}

func isDefaultNamespaceDecl(space, local string) bool {
	return (space == "" && local == "xmlns") ||
		(space == "xmlns" && local == "") ||
		(space == XMLNSNamespace && local == "xmlns")
}

func isPrefixedNamespaceDecl(space string) bool {
	return space == "xmlns" || space == XMLNSNamespace
}

func normalizeAttrNamespace(space, local string) string {
	if isDefaultNamespaceDecl(space, local) || isPrefixedNamespaceDecl(space) {
		return XMLNSNamespace
	}
	return space
}
