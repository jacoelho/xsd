// Package xmlns resolves XML namespace bindings.
package xmlns

import (
	"encoding/xml"
	"errors"
	"slices"

	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
)

type binding struct {
	Prefix string
	URI    string
}

// Stack tracks nested XML namespace declarations and resolves prefixed names.
type Stack struct {
	frames   []int
	bindings []binding
}

const nameSetLinearLimit = 16

// NameSet tracks resolved XML names and reports duplicate attributes.
type NameSet struct {
	index map[xml.Name]struct{}
	names [nameSetLinearLimit]xml.Name
	n     int
}

// AddAttribute records name or returns a duplicate-attribute error.
func (s *NameSet) AddAttribute(name xml.Name) error {
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

// ValidateUniqueAttributes reports duplicate expanded attribute names.
func ValidateUniqueAttributes(attrs []stream.Attr) error {
	if len(attrs) < 2 {
		return nil
	}
	var seen NameSet
	for _, attr := range attrs {
		if err := seen.AddAttribute(attr.Name); err != nil {
			return err
		}
	}
	return nil
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
		frames:   make([]int, 0, frameCap),
		bindings: make([]binding, 0, bindingCap),
	}
}

// Reset clears the stack, retaining bounded slice capacity.
func (s *Stack) Reset(maxRetainedCap int) {
	s.frames = resetRetainedValues(s.frames, maxRetainedCap)
	s.bindings = resetRetainedReferences(s.bindings, maxRetainedCap)
}

func resetRetainedReferences[T any](s []T, maxRetainedCap int) []T {
	if cap(s) > maxRetainedCap {
		return nil
	}
	clear(s)
	return s[:0]
}

func resetRetainedValues[T any](s []T, maxRetainedCap int) []T {
	if cap(s) > maxRetainedCap {
		return nil
	}
	return s[:0]
}

// FrameCapacity returns the retained frame slice capacity.
func (s *Stack) FrameCapacity() int {
	return cap(s.frames)
}

// BindingCapacity returns the retained binding slice capacity.
func (s *Stack) BindingCapacity() int {
	return cap(s.bindings)
}

// Push applies namespace declarations from encoding/xml attributes.
func (s *Stack) Push(attrs []xml.Attr) error {
	mark := len(s.bindings)
	for _, a := range attrs {
		if !IsNamespaceName(a.Name) {
			continue
		}
		if err := s.appendBinding(mark, a.Name, a.Value); err != nil {
			return err
		}
	}
	s.frames = append(s.frames, mark)
	return nil
}

// PushStream applies namespace declarations from stream attributes.
func (s *Stack) PushStream(attrs []stream.Attr, values *stream.Cache) error {
	mark := len(s.bindings)
	for i := range attrs {
		a := &attrs[i]
		if !IsNamespaceName(a.Name) {
			continue
		}
		if err := s.appendBinding(mark, a.Name, a.StringValue(values)); err != nil {
			return err
		}
	}
	s.frames = append(s.frames, mark)
	return nil
}

// appendBinding validates one xmlns declaration and appends it, rolling back
// bindings added since mark on error.
func (s *Stack) appendBinding(mark int, name xml.Name, uri string) error {
	prefix := ""
	var err error
	if name.Space == vocab.XMLNSPrefix {
		prefix = name.Local
		err = validateNamespaceBinding(prefix, uri)
	} else {
		err = validateDefaultNamespaceBinding(uri)
	}
	if err != nil {
		clear(s.bindings[mark:])
		s.bindings = s.bindings[:mark]
		return err
	}
	s.bindings = append(s.bindings, binding{Prefix: prefix, URI: uri})
	return nil
}

// NameKind identifies how default namespaces apply to a name.
type NameKind uint8

const (
	// ElementName resolves unprefixed names through the default namespace.
	ElementName NameKind = iota
	// AttributeName leaves unprefixed names in no namespace.
	AttributeName
)

// ResolveName resolves name using the current namespace bindings.
func (s *Stack) ResolveName(name xml.Name, kind NameKind) (xml.Name, bool) {
	if name.Space != "" {
		uri, ok := s.Lookup(name.Space)
		if !ok {
			return xml.Name{}, false
		}
		return xml.Name{Space: uri, Local: name.Local}, true
	}
	if kind == ElementName {
		if len(s.bindings) == 0 {
			return name, true
		}
		uri, _ := s.Lookup("")
		return xml.Name{Space: uri, Local: name.Local}, true
	}
	return name, true
}

// Pop removes the bindings pushed for the current element.
func (s *Stack) Pop() {
	if len(s.frames) == 0 {
		return
	}
	i := len(s.frames) - 1
	mark := s.frames[i]
	s.frames[i] = 0
	s.frames = s.frames[:i]
	clear(s.bindings[mark:])
	s.bindings = s.bindings[:mark]
}

// Lookup resolves a namespace prefix.
func (s *Stack) Lookup(prefix string) (string, bool) {
	if prefix == vocab.XMLPrefix {
		return vocab.XMLNamespaceURI, true
	}
	for _, binding := range slices.Backward(s.bindings) {
		if binding.Prefix == prefix {
			return binding.URI, true
		}
	}
	if prefix == "" {
		return "", true
	}
	return "", false
}

// IsNamespaceAttr reports whether a standard xml.Attr declares a namespace.
func IsNamespaceAttr(a xml.Attr) bool {
	return IsNamespaceName(a.Name)
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
