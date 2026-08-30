package validate

import (
	"encoding/xml"
	"errors"
	"strings"

	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/xmlns"
	"github.com/jacoelho/xsd/xsderrors"
)

type xmlDocument[P any] struct {
	pathText      string
	elements      []xmlDocumentElement[P]
	ns            xmlns.Stack
	pathTextDepth int
	seenRoot      bool
}

type xmlDocumentElement[P any] struct {
	payload    P
	name       xml.Name
	namespace  xmlns.Frame
	prefix     string
	pathLength int
	pathMode   xmlPathMode
}

type xmlPathMode uint8

const (
	xmlPathInvalid xmlPathMode = iota
	xmlPathLexical
	xmlPathExpanded
)

type preparedXMLStart struct {
	name      xml.Name
	namespace xmlns.Frame
	prefix    string
}

type xmlDocumentCheckpoint struct {
	pathText      string
	depth         int
	pathTextDepth int
	seenRoot      bool
}

func (d *xmlDocument[P]) startCheckpoint() xmlDocumentCheckpoint {
	return xmlDocumentCheckpoint{
		pathText:      d.pathText,
		depth:         d.Depth(),
		pathTextDepth: d.pathTextDepth,
		seenRoot:      d.seenRoot,
	}
}

func (d *xmlDocument[P]) rollbackStart(checkpoint xmlDocumentCheckpoint, namespace xmlns.Frame) error {
	clear(d.elements[checkpoint.depth:])
	d.elements = d.elements[:checkpoint.depth]
	d.pathText = checkpoint.pathText
	d.pathTextDepth = checkpoint.pathTextDepth
	d.seenRoot = checkpoint.seenRoot
	if namespace.IsZero() {
		return nil
	}
	return d.ns.Abort(namespace)
}

func (d *xmlDocument[P]) clearCurrentPayload() {
	if len(d.elements) == 0 {
		return
	}
	var zero P
	d.elements[len(d.elements)-1].payload = zero
}

func (d *xmlDocument[P]) PrepareStart(
	start stream.StartElement,
	values *stream.Cache,
	maxDepth int,
	line, col int,
) (preparedXMLStart, error) {
	if maxDepth > 0 && d.Depth()+1 > maxDepth {
		return preparedXMLStart{}, validation(d.context(line, col), xsderrors.CodeValidationLimit, "instance depth limit exceeded")
	}
	namespace, element, err := d.ns.StartStream(&start, values)
	if err != nil {
		return preparedXMLStart{}, validation(d.context(line, col), xsderrors.CodeValidationXML, err.Error())
	}
	if d.seenRoot && d.Depth() == 0 {
		primary := validation(d.context(line, col), xsderrors.CodeValidationXML, "multiple root elements")
		if abortErr := d.ns.Abort(namespace); abortErr != nil {
			return preparedXMLStart{}, errors.Join(primary, abortErr)
		}
		return preparedXMLStart{}, primary
	}

	return preparedXMLStart{name: element.Name, namespace: namespace, prefix: element.Lexical.Prefix}, nil
}

func (d *xmlDocument[P]) CommitStart(start preparedXMLStart, payload P) {
	d.appendStart(start, xmlPathLexical, 1+len(start.name.Local), payload)
}

func (d *xmlDocument[P]) CommitExpandedStart(start preparedXMLStart, payload P) {
	d.appendStart(start, xmlPathExpanded, 3+len(start.name.Space)+len(start.name.Local), payload)
}

func (d *xmlDocument[P]) appendStart(start preparedXMLStart, pathMode xmlPathMode, pathLength int, payload P) {
	if len(d.elements) != 0 {
		pathLength += d.elements[len(d.elements)-1].pathLength
	}
	d.elements = append(d.elements, xmlDocumentElement[P]{
		payload:    payload,
		name:       start.name,
		namespace:  start.namespace,
		prefix:     start.prefix,
		pathLength: pathLength,
		pathMode:   pathMode,
	})
	d.seenRoot = true
}

func (d *xmlDocument[P]) AbortStart(start preparedXMLStart) error {
	if start.namespace.IsZero() {
		return nil
	}
	return d.ns.Abort(start.namespace)
}

func (d *xmlDocument[P]) ValidateEnd(end stream.EndElement, line, col int) error {
	if d.Depth() == 0 {
		return validation(d.context(line, col), xsderrors.CodeValidationXML, "unexpected end element")
	}

	expected := d.elements[len(d.elements)-1]
	if err := d.ns.MatchEnd(expected.namespace, xmlns.Lexical(end.Name)); err != nil {
		return validation(d.context(line, col), xsderrors.CodeValidationXML, err.Error())
	}
	return nil
}

func (d *xmlDocument[P]) CommitEnd() error {
	if d.Depth() == 0 {
		return xsderrors.InternalInvariant("cannot commit XML end element with no open element")
	}

	i := len(d.elements) - 1
	namespace := d.elements[i].namespace
	d.elements[i] = xmlDocumentElement[P]{}
	d.elements = d.elements[:i]
	if !namespace.IsZero() {
		if err := d.ns.Abort(namespace); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
	}
	if d.pathTextDepth <= i {
		return nil
	}
	if i == 0 {
		d.pathText = ""
		d.pathTextDepth = 0
		return nil
	}
	d.pathText = d.pathText[:d.elements[i-1].pathLength]
	d.pathTextDepth = i
	return nil
}

func (d *xmlDocument[P]) Complete() error {
	if !d.seenRoot {
		return validation(StartContext{}, xsderrors.CodeValidationRoot, "instance document has no root element")
	}
	if d.Depth() != 0 {
		return validation(d.context(0, 0), xsderrors.CodeValidationXML, "unclosed element")
	}
	return nil
}

func (d *xmlDocument[P]) Reset(maxRetainedCap int) {
	d.ns.Reset(maxRetainedCap)
	if cap(d.elements) > maxRetainedCap {
		d.elements = nil
	} else {
		clear(d.elements)
		d.elements = d.elements[:0]
	}
	d.pathText = ""
	d.pathTextDepth = 0
	d.seenRoot = false
}

func (d *xmlDocument[P]) Depth() int {
	return len(d.elements)
}

func (d *xmlDocument[P]) Current() (*P, bool) {
	if len(d.elements) == 0 {
		return nil, false
	}
	return &d.elements[len(d.elements)-1].payload, true
}

func (d *xmlDocument[P]) clearPayloads() {
	var zero P
	for i := range d.elements {
		d.elements[i].payload = zero
	}
}

func (d *xmlDocument[P]) LookupNamespace(prefix string) (string, bool) {
	return d.ns.Lookup(prefix)
}

func (d *xmlDocument[P]) PathString() string {
	depth := d.Depth()
	if depth == 0 {
		return "/"
	}
	if d.pathTextDepth == depth {
		return d.pathText
	}

	var path strings.Builder
	path.Grow(d.elements[depth-1].pathLength)
	start := 0
	if d.pathTextDepth != 0 {
		path.WriteString(d.pathText)
		start = d.pathTextDepth
	}
	for i := start; i < depth; i++ {
		path.WriteByte('/')
		element := d.elements[i]
		switch element.pathMode {
		case xmlPathExpanded:
			path.WriteByte('{')
			path.WriteString(element.name.Space)
			path.WriteByte('}')
		case xmlPathLexical:
		case xmlPathInvalid:
			panic("XML path mode is invalid")
		default:
			message := "XML path mode is invalid"
			panic(message)
		}
		path.WriteString(element.name.Local)
	}
	d.pathText = path.String()
	d.pathTextDepth = depth
	return d.pathText
}

func (d *xmlDocument[P]) context(line, col int) StartContext {
	return StartContext{document: d, Line: line, Column: col}
}
