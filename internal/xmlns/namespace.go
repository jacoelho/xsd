// Package xmlns resolves XML namespace bindings.
package xmlns

import (
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
)

type binding struct {
	Prefix string
	URI    string
	Parent uint32
}

type contextStore struct {
	bindings []binding
}

type stackFrame struct {
	token       Frame
	element     Element
	previous    uint32
	bindingMark int
}

// Stack owns nested XML namespace frames. Every admitted start returns the
// capability required to close or abort that exact frame.
type Stack struct {
	store         *contextStore
	frames        []stackFrame
	resolvedAttrs []xml.Name
	seen          nameSet
	serial        uint64
	head          uint32
	persistent    bool
}

// Frame identifies one admitted namespace frame. Its fields are intentionally
// opaque so callers cannot synthesize ownership of a live frame.
type Frame struct {
	store  *contextStore
	serial uint64
}

// IsZero reports whether f identifies no admitted frame.
func (f Frame) IsZero() bool {
	return f.store == nil && f.serial == 0
}

// LexicalName is an XML name before namespace expansion.
type LexicalName struct {
	Prefix string
	Local  string
}

// Element identifies one admitted element by lexical and expanded name.
type Element struct {
	Lexical LexicalName
	Name    xml.Name
}

// Context is an immutable namespace projection retained beyond stack mutation.
type Context struct {
	store *contextStore
	head  uint32
}

// Lexical converts the repository's lexical xml.Name spelling to an explicit name.
func Lexical(name xml.Name) LexicalName {
	return LexicalName{Prefix: name.Space, Local: name.Local}
}

// StartXML atomically admits an encoding/xml start element.
func (s *Stack) StartXML(start xml.StartElement) (Frame, Element, error) {
	lexical := Lexical(start.Name)
	mark, previous := s.beginAdmission()
	if err := s.appendXMLBindings(start.Attr); err != nil {
		return s.abortAdmission(mark, previous, err)
	}
	element, err := s.resolveElement(lexical)
	if err != nil {
		return s.abortAdmission(mark, previous, err)
	}
	if err := s.resolveXMLAttributes(start.Attr); err != nil {
		return s.abortAdmission(mark, previous, err)
	}
	return s.commitAdmission(mark, previous, element), element, nil
}

// StartStream atomically admits a borrowed stream start element. On success it
// replaces every lexical attribute name with its expanded name.
func (s *Stack) StartStream(start *stream.StartElement, values *stream.Cache) (Frame, Element, error) {
	if start == nil {
		return Frame{}, Element{}, errors.New("nil XML start element")
	}
	lexical := Lexical(start.Name)
	mark, previous := s.beginAdmission()
	for i := range start.Attr {
		attr := &start.Attr[i]
		if !IsNamespaceName(attr.Name) {
			continue
		}
		value, valueAvailable := attr.MaterializeValue(values)
		if err := s.appendStreamBinding(attr.Name, value, valueAvailable); err != nil {
			return s.abortAdmission(mark, previous, err)
		}
	}
	element, err := s.resolveElement(lexical)
	if err != nil {
		return s.abortAdmission(mark, previous, err)
	}
	resolved := s.prepareAttributeAdmission(len(start.Attr))
	for i := range start.Attr {
		name, err := s.resolveStreamAttribute(start.Attr[i].Name)
		if err != nil {
			return s.abortAdmission(mark, previous, err)
		}
		resolved[i] = name
	}
	frame := s.commitAdmission(mark, previous, element)
	start.ReplaceAttributeNames(s.resolvedAttrs)
	s.clearAttributeAdmission()
	return frame, element, nil
}

func (s *Stack) appendXMLBindings(attrs []xml.Attr) error {
	for _, attr := range attrs {
		if !IsNamespaceName(attr.Name) {
			continue
		}
		if err := s.appendBinding(attr.Name, attr.Value); err != nil {
			return err
		}
	}
	return nil
}

func (s *Stack) appendStreamBinding(name xml.Name, value string, valueAvailable bool) error {
	if !valueAvailable {
		return errors.New("namespace declaration requires an attribute value cache")
	}
	return s.appendBinding(name, value)
}

func (s *Stack) resolveXMLAttributes(attrs []xml.Attr) error {
	resolved := s.prepareAttributeAdmission(len(attrs))
	for i, attr := range attrs {
		name, err := s.resolveAttribute(attr.Name)
		if err != nil {
			return err
		}
		if err := s.seen.add(name); err != nil {
			return err
		}
		resolved[i] = name
	}
	return nil
}

func (s *Stack) resolveStreamAttribute(lexical xml.Name) (xml.Name, error) {
	name, err := s.resolveAttribute(lexical)
	if err != nil {
		return xml.Name{}, err
	}
	if err := s.seen.add(name); err != nil {
		return xml.Name{}, err
	}
	return name, nil
}

func (s *Stack) abortAdmission(mark int, previous uint32, err error) (Frame, Element, error) {
	s.rollbackAdmission(mark, previous)
	return Frame{}, Element{}, err
}

// End validates a lexical closing name and releases the identified top frame.
// A failed match leaves the frame live.
func (s *Stack) End(frame Frame, end LexicalName) error {
	if err := s.MatchEnd(frame, end); err != nil {
		return err
	}
	s.pop()
	return nil
}

// MatchEnd validates a lexical closing name without changing stack state.
func (s *Stack) MatchEnd(frame Frame, end LexicalName) error {
	current, err := s.ownedTop(frame)
	if err != nil {
		return err
	}
	resolved, ok := s.resolveName(xml.Name{Space: end.Prefix, Local: end.Local}, elementName)
	if !ok {
		return fmt.Errorf("unbound namespace prefix %s", end.Prefix)
	}
	if end != current.element.Lexical {
		return fmt.Errorf("end element </%s> does not match start element <%s>", formatLexical(end), formatLexical(current.element.Lexical))
	}
	if resolved != current.element.Name {
		return fmt.Errorf("end element </%s> does not match start element <%s>", FormatName(resolved), FormatName(current.element.Name))
	}
	return nil
}

// Abort releases the identified top frame without validating a closing name.
func (s *Stack) Abort(frame Frame) error {
	if _, err := s.ownedTop(frame); err != nil {
		return err
	}
	s.pop()
	return nil
}

func (s *Stack) ownedTop(frame Frame) (*stackFrame, error) {
	if frame.IsZero() {
		return nil, errors.New("namespace frame is empty")
	}
	if len(s.frames) == 0 || s.store != frame.store {
		return nil, errors.New("namespace frame is not owned by this stack")
	}
	current := &s.frames[len(s.frames)-1]
	if current.token != frame {
		return nil, errors.New("namespace frame is not the current frame")
	}
	return current, nil
}

func (s *Stack) beginAdmission() (int, uint32) {
	s.ensureStore()
	return len(s.store.bindings), s.head
}

func (s *Stack) commitAdmission(mark int, previous uint32, element Element) Frame {
	s.serial++
	if s.serial == 0 {
		s.serial++
	}
	frame := Frame{store: s.store, serial: s.serial}
	s.frames = append(s.frames, stackFrame{
		token:       frame,
		element:     element,
		previous:    previous,
		bindingMark: mark,
	})
	return frame
}

func (s *Stack) rollbackAdmission(mark int, previous uint32) {
	clear(s.store.bindings[mark:])
	s.store.bindings = s.store.bindings[:mark]
	s.head = previous
	s.clearAttributeAdmission()
}

func (s *Stack) pop() {
	i := len(s.frames) - 1
	current := s.frames[i]
	s.frames[i] = stackFrame{}
	s.frames = s.frames[:i]
	s.head = current.previous
	if !s.persistent {
		clear(s.store.bindings[current.bindingMark:])
		s.store.bindings = s.store.bindings[:current.bindingMark]
	}
}

func (s *Stack) ensureStore() {
	if s.store == nil {
		s.store = new(contextStore)
	}
}

func (s *Stack) prepareAttributeAdmission(n int) []xml.Name {
	s.seen.reset()
	if cap(s.resolvedAttrs) < n {
		s.resolvedAttrs = make([]xml.Name, n)
	} else {
		s.resolvedAttrs = s.resolvedAttrs[:n]
		clear(s.resolvedAttrs)
	}
	return s.resolvedAttrs
}

func (s *Stack) clearAttributeAdmission() {
	clear(s.resolvedAttrs)
}

func (s *Stack) resolveElement(lexical LexicalName) (Element, error) {
	name, ok := s.resolveName(xml.Name{Space: lexical.Prefix, Local: lexical.Local}, elementName)
	if !ok {
		return Element{}, fmt.Errorf("unbound namespace prefix %s", lexical.Prefix)
	}
	return Element{Lexical: lexical, Name: name}, nil
}

func (s *Stack) resolveAttribute(name xml.Name) (xml.Name, error) {
	if IsNamespaceName(name) {
		return name, nil
	}
	resolved, ok := s.resolveName(name, attributeName)
	if !ok {
		return xml.Name{}, fmt.Errorf("unbound namespace prefix %s", name.Space)
	}
	return resolved, nil
}

func formatLexical(name LexicalName) string {
	if name.Prefix == "" {
		return name.Local
	}
	return name.Prefix + ":" + name.Local
}

// Context returns a constant-time immutable view of all active bindings.
func (s *Stack) Context() Context {
	if s.store == nil {
		return Context{}
	}
	s.persistent = true
	return Context{store: s.store, head: s.head}
}

// Lookup resolves a prefix in an immutable context.
func (c Context) Lookup(prefix string) (string, bool) {
	return lookup(c.store, c.head, prefix)
}

const nameSetLinearLimit = 16

type nameSet struct {
	index map[xml.Name]struct{}
	names [nameSetLinearLimit]xml.Name
	n     int
}

func (s *nameSet) reset() {
	clear(s.index)
	clear(s.names[:s.n])
	s.n = 0
}

func (s *nameSet) add(name xml.Name) error {
	if s.index != nil {
		if _, ok := s.index[name]; ok {
			return duplicateAttributeError(name)
		}
		s.index[name] = struct{}{}
		return nil
	}
	if slices.Contains(s.names[:s.n], name) {
		return duplicateAttributeError(name)
	}
	if s.n < len(s.names) {
		s.names[s.n] = name
		s.n++
		return nil
	}
	s.index = make(map[xml.Name]struct{}, s.n+1)
	for _, existing := range s.names[:s.n] {
		s.index[existing] = struct{}{}
	}
	s.index[name] = struct{}{}
	return nil
}

func duplicateAttributeError(name xml.Name) error {
	return errors.New("duplicate attribute " + FormatName(name))
}

// FormatName formats an XML name using expanded-name notation.
func FormatName(n xml.Name) string {
	if n.Space == "" {
		return n.Local
	}
	return "{" + n.Space + "}" + n.Local
}

// NewStackWithCapacity returns an empty stack with retained slice capacity.
func NewStackWithCapacity(frameCap, bindingCap int) Stack {
	return Stack{
		store:  &contextStore{bindings: make([]binding, 0, bindingCap)},
		frames: make([]stackFrame, 0, frameCap),
	}
}

// Reset invalidates live frames and clears the stack while retaining bounded
// private storage. A store published through Context is detached, never reused.
func (s *Stack) Reset(maxRetainedCap int) {
	s.frames = resetRetainedReferences(s.frames, maxRetainedCap)
	s.resolvedAttrs = resetRetainedReferences(s.resolvedAttrs, maxRetainedCap)
	if len(s.seen.index) > maxRetainedCap {
		s.seen.index = nil
	} else {
		s.seen.reset()
	}
	if s.persistent {
		s.store = nil
	} else if s.store != nil {
		s.store.bindings = resetRetainedReferences(s.store.bindings, maxRetainedCap)
	}
	s.head = 0
	s.persistent = false
}

func resetRetainedReferences[T any](values []T, maxRetainedCap int) []T {
	if cap(values) > maxRetainedCap {
		return nil
	}
	clear(values)
	return values[:0]
}

// FrameCapacity returns the retained frame slice capacity.
func (s *Stack) FrameCapacity() int {
	return cap(s.frames)
}

// BindingCapacity returns the retained binding arena capacity.
func (s *Stack) BindingCapacity() int {
	if s.store == nil {
		return 0
	}
	return cap(s.store.bindings)
}

func (s *Stack) appendBinding(name xml.Name, uri string) error {
	prefix := ""
	var err error
	if name.Space == vocab.XMLNSPrefix {
		prefix = name.Local
		err = validateNamespaceBinding(prefix, uri)
	} else {
		err = validateDefaultNamespaceBinding(uri)
	}
	if err != nil {
		return err
	}
	if uint64(len(s.store.bindings)) >= uint64(math.MaxUint32) {
		return errors.New("namespace binding limit exceeded")
	}
	s.store.bindings = append(s.store.bindings, binding{Prefix: prefix, URI: uri, Parent: s.head})
	s.head = uint32(len(s.store.bindings)) //nolint:gosec // The MaxUint32 guard above proves the conversion safe.
	return nil
}

type nameKind uint8

const (
	elementName nameKind = iota
	attributeName
)

func (s *Stack) resolveName(name xml.Name, kind nameKind) (xml.Name, bool) {
	if name.Space != "" {
		uri, ok := s.Lookup(name.Space)
		if !ok {
			return xml.Name{}, false
		}
		return xml.Name{Space: uri, Local: name.Local}, true
	}
	if kind == elementName {
		uri, _ := s.Lookup("")
		return xml.Name{Space: uri, Local: name.Local}, true
	}
	return name, true
}

// Lookup resolves a namespace prefix in the active stack.
func (s *Stack) Lookup(prefix string) (string, bool) {
	return lookup(s.store, s.head, prefix)
}

func lookup(store *contextStore, head uint32, prefix string) (string, bool) {
	if prefix == vocab.XMLPrefix {
		return vocab.XMLNamespaceURI, true
	}
	for head != 0 {
		current := store.bindings[head-1]
		if current.Prefix == prefix {
			return current.URI, true
		}
		head = current.Parent
	}
	if prefix == "" {
		return "", true
	}
	return "", false
}

// IsNamespaceName reports whether name is an xmlns declaration name.
func IsNamespaceName(name xml.Name) bool {
	return name.Space == vocab.XMLNSPrefix || (name.Space == "" && name.Local == vocab.XMLNSPrefix)
}

func validateNamespaceBinding(prefix, uri string) error {
	if prefix == vocab.XMLNSPrefix {
		return errors.New("xmlns prefix cannot be declared")
	}
	if prefix == vocab.XMLPrefix {
		if uri != vocab.XMLNamespaceURI {
			return errors.New("xml prefix must be bound to " + vocab.XMLNamespaceURI)
		}
		return nil
	}
	if uri == "" {
		return errors.New("prefixed namespace binding cannot be empty")
	}
	if uri == vocab.XMLNamespaceURI {
		return errors.New("xml namespace URI can only be bound to xml prefix")
	}
	if uri == vocab.XMLNSNamespaceURI {
		return errors.New("xmlns namespace URI cannot be declared")
	}
	return nil
}

func validateDefaultNamespaceBinding(uri string) error {
	if uri == vocab.XMLNamespaceURI {
		return errors.New("xml namespace URI cannot be the default namespace")
	}
	if uri == vocab.XMLNSNamespaceURI {
		return errors.New("xmlns namespace URI cannot be declared")
	}
	return nil
}
