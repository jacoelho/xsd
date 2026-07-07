package validate

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/xmlns"
	"github.com/jacoelho/xsd/xsderrors"
)

// CheckXMLWellFormed checks XML instance syntax without compiling or using a schema runtime.
func CheckXMLWellFormed(r io.Reader, opts Options) error {
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return err
	}
	reader, err := PrepareInstanceReaderWithBuffer(r, nil)
	if err != nil {
		return err
	}

	c := xmlWellFormedChecker{
		maxDepth:      limits.InstanceDepth,
		maxAttributes: limits.InstanceAttributes,
		maxTokenBytes: limits.InstanceTokenBytes,
	}
	return c.check(reader)
}

type xmlWellFormedChecker struct {
	ns            xmlns.Stack
	elementNames  []xml.Name
	elementRaw    []string
	path          []string
	maxDepth      int
	maxAttributes int
	maxTokenBytes int64
	seenRoot      bool
}

func (c *xmlWellFormedChecker) check(r io.Reader) error {
	names := stream.NewCache()
	values := stream.NewCache()
	var parser stream.Parser
	parser.ResetWithLimit(r, &names, &values, c.maxTokenBytes)
	parser.SetLazyAttrValue(true)
	parser.SetMaxAttrs(c.maxAttributes)
	for {
		tok, err := parser.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return c.streamError(&parser, tok, err)
		}
		switch tok.Kind {
		case stream.KindStart:
			if err := c.start(tok.Line, tok.Column, tok.Start, &values); err != nil {
				return err
			}
		case stream.KindEnd:
			if err := c.end(tok.Line, tok.Column, tok.End); err != nil {
				return err
			}
		case stream.KindCharData:
			if err := c.chars(tok.Line, tok.Column, tok.Data, tok.CDATA); err != nil {
				return err
			}
		case stream.KindDirective:
			return ValidateDirective(c.context(tok.Line, tok.Column), tok.Directive)
		case stream.KindComment, stream.KindPI:
		}
	}
	return ValidateDocumentComplete(DocumentCompleteInput{
		Context:      c.context(0, 0),
		SeenRoot:     c.seenRoot,
		OpenElements: len(c.elementNames),
	})
}

func (c *xmlWellFormedChecker) start(line, col int, se stream.StartElement, values *stream.Cache) error {
	ctx := c.context(line, col)
	if c.maxDepth > 0 && len(c.elementNames)+1 > c.maxDepth {
		return validation(ctx, xsderrors.CodeValidationLimit, "instance depth limit exceeded")
	}
	if err := ValidateDocumentElementStart(DocumentElementStartInput{
		Context:      ctx,
		SeenRoot:     c.seenRoot,
		OpenElements: len(c.elementNames),
	}); err != nil {
		return err
	}
	if err := c.pushNamespaces(se.Attr, values); err != nil {
		return xsderrors.Validation(xsderrors.CodeValidationXML, line, col, c.pathString(), err.Error())
	}
	translated, err := c.translateStart(se, line, col)
	if err != nil {
		return err
	}
	c.seenRoot = true
	c.elementNames = append(c.elementNames, translated.Name)
	c.elementRaw = append(c.elementRaw, translated.RawName)
	c.path = append(c.path, translated.Name.Local)
	return nil
}

func (c *xmlWellFormedChecker) end(line, col int, ee stream.EndElement) error {
	ctx := c.context(line, col)
	if len(c.elementNames) == 0 {
		return ValidateEndElement(EndElementInput{Context: ctx})
	}
	name, err := c.translateName(ee.Name, xmlElementName, line, col)
	if err != nil {
		return err
	}
	expected := c.elementNames[len(c.elementNames)-1]
	expectedRaw := c.elementRaw[len(c.elementRaw)-1]
	if err := ValidateEndElement(EndElementInput{
		Name:            name,
		Expected:        expected,
		RawName:         ee.RawName,
		ExpectedRawName: expectedRaw,
		Context:         ctx,
		OpenElements:    len(c.elementNames),
	}); err != nil {
		return err
	}
	c.elementNames = c.elementNames[:len(c.elementNames)-1]
	c.elementRaw = c.elementRaw[:len(c.elementRaw)-1]
	c.path = c.path[:len(c.path)-1]
	c.ns.Pop()
	return nil
}

func (c *xmlWellFormedChecker) pushNamespaces(attrs []stream.Attr, values *stream.Cache) error {
	return c.ns.PushStream(attrs, values)
}

func (c *xmlWellFormedChecker) chars(line, col int, data []byte, cdata bool) error {
	if len(c.elementNames) != 0 {
		return nil
	}
	_, err := ValidateCharacterData(CharacterDataInput{
		Data:    data,
		Context: c.context(line, col),
		CDATA:   cdata,
	})
	return err
}

func (c *xmlWellFormedChecker) translateStart(se stream.StartElement, line, col int) (stream.StartElement, error) {
	name, err := c.translateName(se.Name, xmlElementName, line, col)
	if err != nil {
		return stream.StartElement{}, err
	}
	se.Name = name
	for i, attr := range se.Attr {
		if xmlns.IsNamespaceName(attr.Name) || attr.Name.Space == "" {
			continue
		}
		name, err := c.translateName(attr.Name, xmlAttributeName, line, col)
		if err != nil {
			return stream.StartElement{}, err
		}
		se.Attr[i].Name = name
	}
	if err := xmlns.ValidateUniqueAttributes(se.Attr); err != nil {
		return stream.StartElement{}, xsderrors.Validation(xsderrors.CodeValidationXML, line, col, c.pathString(), err.Error())
	}
	return se, nil
}

func (c *xmlWellFormedChecker) translateName(name xml.Name, kind xmlNameKind, line, col int) (xml.Name, error) {
	resolved, ok := c.ns.ResolveName(name, xmlns.NameKind(kind))
	if !ok {
		return xml.Name{}, validation(c.context(line, col), xsderrors.CodeValidationXML, "unbound namespace prefix "+name.Space)
	}
	return resolved, nil
}

func (c *xmlWellFormedChecker) streamError(parser *stream.Parser, tok stream.Token, err error) error {
	line, col := tok.Line, tok.Column
	if line == 0 {
		line, col = parser.Pos()
	}
	return StreamError(line, col, c.pathString(), err)
}

func (c *xmlWellFormedChecker) context(line, col int) StartContext {
	return StartContext{Path: c.pathString(), Line: line, Column: col}
}

func (c *xmlWellFormedChecker) pathString() string {
	if len(c.path) == 0 {
		return "/"
	}
	var path strings.Builder
	for _, elem := range c.path {
		path.WriteByte('/')
		path.WriteString(elem)
	}
	return path.String()
}
