package validate

import (
	"encoding/xml"
	"strings"

	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/xmlns"
	"github.com/jacoelho/xsd/xsderrors"
)

type xmlDocumentState struct {
	pathText      string
	elements      []xmlDocumentElement
	ns            xmlns.Stack
	pathTextDepth int
	seenRoot      bool
}

type xmlDocumentElement struct {
	name       xml.Name
	rawName    string
	pathLabel  string
	pathLength int
}

type xmlDocumentLimits struct {
	depth      int
	attributes int
}

func (d *xmlDocumentState) PrepareStart(
	start stream.StartElement,
	values *stream.Cache,
	limits xmlDocumentLimits,
	line, col int,
) (_ stream.StartElement, err error) {
	ctx := d.context(line, col)
	if limits.depth > 0 && d.Depth()+1 > limits.depth {
		return stream.StartElement{}, validation(ctx, xsderrors.CodeValidationLimit, "instance depth limit exceeded")
	}
	if limits.attributes > 0 && len(start.Attr) > limits.attributes {
		return stream.StartElement{}, validation(ctx, xsderrors.CodeValidationLimit, "instance attribute limit exceeded")
	}
	if pushErr := d.ns.PushStream(start.Attr, values); pushErr != nil {
		return stream.StartElement{}, validation(ctx, xsderrors.CodeValidationXML, pushErr.Error())
	}
	accepted := false
	defer func() {
		if !accepted {
			d.ns.Pop()
		}
	}()

	start.Name, err = d.resolveName(start.Name, xmlns.ElementName, ctx)
	if err != nil {
		return stream.StartElement{}, err
	}
	for i := range start.Attr {
		attr := &start.Attr[i]
		if xmlns.IsNamespaceName(attr.Name) || attr.Name.Space == "" {
			continue
		}
		attr.Name, err = d.resolveName(attr.Name, xmlns.AttributeName, ctx)
		if err != nil {
			return stream.StartElement{}, err
		}
	}
	if err := xmlns.ValidateUniqueAttributes(start.Attr); err != nil {
		return stream.StartElement{}, validation(ctx, xsderrors.CodeValidationXML, err.Error())
	}
	if d.seenRoot && d.Depth() == 0 {
		return stream.StartElement{}, validation(ctx, xsderrors.CodeValidationXML, "multiple root elements")
	}

	accepted = true
	return start, nil
}

func (d *xmlDocumentState) CommitStart(name xml.Name, rawName, pathLabel string) {
	pathLength := 1 + len(pathLabel)
	if len(d.elements) != 0 {
		pathLength += d.elements[len(d.elements)-1].pathLength
	}
	d.elements = append(d.elements, xmlDocumentElement{
		name:       name,
		rawName:    rawName,
		pathLabel:  pathLabel,
		pathLength: pathLength,
	})
	d.seenRoot = true
}

func (d *xmlDocumentState) ValidateEnd(end stream.EndElement, line, col int) error {
	ctx := d.context(line, col)
	if d.Depth() == 0 {
		return validation(ctx, xsderrors.CodeValidationXML, "unexpected end element")
	}

	name, err := d.resolveName(end.Name, xmlns.ElementName, ctx)
	if err != nil {
		return err
	}
	expected := d.elements[len(d.elements)-1]
	if (end.RawName != "" || expected.rawName != "") && end.RawName != expected.rawName {
		return validation(ctx, xsderrors.CodeValidationXML, "end element </"+formatElementName(end.RawName, name)+"> does not match start element <"+formatElementName(expected.rawName, expected.name)+">")
	}
	if name != expected.name {
		return validation(ctx, xsderrors.CodeValidationXML, "end element </"+formatXMLName(name)+"> does not match start element <"+formatXMLName(expected.name)+">")
	}
	return nil
}

func (d *xmlDocumentState) CommitEnd() error {
	if d.Depth() == 0 {
		return xsderrors.InternalInvariant("cannot commit XML end element with no open element")
	}

	i := len(d.elements) - 1
	d.elements[i] = xmlDocumentElement{}
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

func (d *xmlDocumentState) Complete() error {
	if !d.seenRoot {
		return validation(StartContext{}, xsderrors.CodeValidationRoot, "instance document has no root element")
	}
	if d.Depth() != 0 {
		return validation(d.context(0, 0), xsderrors.CodeValidationXML, "unclosed element")
	}
	return nil
}

func (d *xmlDocumentState) Reset(maxRetainedCap int) {
	d.ns.Reset(maxRetainedCap)
	if cap(d.elements) > maxRetainedCap {
		d.elements = nil
	} else {
		clear(d.elements[:cap(d.elements)])
		d.elements = d.elements[:0]
	}
	d.pathText = ""
	d.pathTextDepth = 0
	d.seenRoot = false
}

func (d *xmlDocumentState) Depth() int {
	return len(d.elements)
}

func (d *xmlDocumentState) LookupNamespace(prefix string) (string, bool) {
	return d.ns.Lookup(prefix)
}

func (d *xmlDocumentState) PathString() string {
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
		path.WriteString(d.elements[i].pathLabel)
	}
	d.pathText = path.String()
	d.pathTextDepth = depth
	return d.pathText
}

func (d *xmlDocumentState) context(line, col int) StartContext {
	return StartContext{document: d, Line: line, Column: col}
}

func (d *xmlDocumentState) resolveName(name xml.Name, kind xmlns.NameKind, ctx StartContext) (xml.Name, error) {
	resolved, ok := d.ns.ResolveName(name, kind)
	if !ok {
		return xml.Name{}, validation(ctx, xsderrors.CodeValidationXML, "unbound namespace prefix "+name.Space)
	}
	return resolved, nil
}

func formatElementName(raw string, name xml.Name) string {
	if raw != "" {
		return raw
	}
	return formatXMLName(name)
}
