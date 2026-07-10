package validate

import (
	"encoding/xml"
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
	payload      P
	name         xml.Name
	rawName      string
	pathLength   int
	expandedPath bool
}

type xmlDocumentLimits struct {
	depth      int
	attributes int
}

func (d *xmlDocument[P]) PrepareStart(
	start stream.StartElement,
	values *stream.Cache,
	limits xmlDocumentLimits,
	line, col int,
) (stream.StartElement, error) {
	if limits.depth > 0 && d.Depth()+1 > limits.depth {
		return stream.StartElement{}, validation(d.context(line, col), xsderrors.CodeValidationLimit, "instance depth limit exceeded")
	}
	if limits.attributes > 0 && len(start.Attr) > limits.attributes {
		return stream.StartElement{}, validation(d.context(line, col), xsderrors.CodeValidationLimit, "instance attribute limit exceeded")
	}
	if pushErr := d.ns.PushStream(start.Attr, values); pushErr != nil {
		return stream.StartElement{}, validation(d.context(line, col), xsderrors.CodeValidationXML, pushErr.Error())
	}
	var err error
	start.Name, err = d.resolveName(start.Name, xmlns.ElementName, line, col)
	if err != nil {
		d.ns.Pop()
		return stream.StartElement{}, err
	}
	for i := range start.Attr {
		attr := &start.Attr[i]
		if xmlns.IsNamespaceName(attr.Name) || attr.Name.Space == "" {
			continue
		}
		attr.Name, err = d.resolveName(attr.Name, xmlns.AttributeName, line, col)
		if err != nil {
			d.ns.Pop()
			return stream.StartElement{}, err
		}
	}
	if err := xmlns.ValidateUniqueAttributes(start.Attr); err != nil {
		d.ns.Pop()
		return stream.StartElement{}, validation(d.context(line, col), xsderrors.CodeValidationXML, err.Error())
	}
	if d.seenRoot && d.Depth() == 0 {
		d.ns.Pop()
		return stream.StartElement{}, validation(d.context(line, col), xsderrors.CodeValidationXML, "multiple root elements")
	}

	return start, nil
}

func (d *xmlDocument[P]) CommitStart(name xml.Name, rawName string, expandedPath bool, payload P) {
	pathLength := 1 + len(name.Local)
	if expandedPath {
		pathLength += len(name.Space) + 2
	}
	if len(d.elements) != 0 {
		pathLength += d.elements[len(d.elements)-1].pathLength
	}
	d.elements = append(d.elements, xmlDocumentElement[P]{
		payload:      payload,
		name:         name,
		rawName:      rawName,
		pathLength:   pathLength,
		expandedPath: expandedPath,
	})
	d.seenRoot = true
}

func (d *xmlDocument[P]) AbortStart() {
	d.ns.Pop()
}

func (d *xmlDocument[P]) ValidateEnd(end stream.EndElement, line, col int) error {
	if d.Depth() == 0 {
		return validation(d.context(line, col), xsderrors.CodeValidationXML, "unexpected end element")
	}

	name, err := d.resolveName(end.Name, xmlns.ElementName, line, col)
	if err != nil {
		return err
	}
	expected := d.elements[len(d.elements)-1]
	if (end.RawName != "" || expected.rawName != "") && end.RawName != expected.rawName {
		return validation(d.context(line, col), xsderrors.CodeValidationXML, "end element </"+formatElementName(end.RawName, name)+"> does not match start element <"+formatElementName(expected.rawName, expected.name)+">")
	}
	if name != expected.name {
		return validation(d.context(line, col), xsderrors.CodeValidationXML, "end element </"+formatXMLName(name)+"> does not match start element <"+formatXMLName(expected.name)+">")
	}
	return nil
}

func (d *xmlDocument[P]) CommitEnd() error {
	if d.Depth() == 0 {
		return xsderrors.InternalInvariant("cannot commit XML end element with no open element")
	}

	i := len(d.elements) - 1
	d.elements[i] = xmlDocumentElement[P]{}
	d.elements = d.elements[:i]
	d.ns.Pop()
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
		if element.expandedPath {
			path.WriteByte('{')
			path.WriteString(element.name.Space)
			path.WriteByte('}')
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

func (d *xmlDocument[P]) resolveName(name xml.Name, kind xmlns.NameKind, line, col int) (xml.Name, error) {
	resolved, ok := d.ns.ResolveName(name, kind)
	if !ok {
		return xml.Name{}, validation(d.context(line, col), xsderrors.CodeValidationXML, "unbound namespace prefix "+name.Space)
	}
	return resolved, nil
}

func formatElementName(raw string, name xml.Name) string {
	if raw != "" {
		return raw
	}
	return formatXMLName(name)
}
