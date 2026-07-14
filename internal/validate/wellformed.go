package validate

import (
	"context"
	"io"

	"github.com/jacoelho/xsd/internal/stream"
)

// CheckXMLWellFormed checks XML instance syntax without compiling or using a schema runtime.
func CheckXMLWellFormed(ctx context.Context, r io.Reader, opts Options) error {
	if err := validationContextError(ctx); err != nil {
		return err
	}
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return err
	}
	c := xmlWellFormedChecker{
		maxDepth:      limits.InstanceDepth,
		maxAttributes: limits.InstanceAttributes,
		maxTokenBytes: limits.InstanceTokenBytes,
		maxInputBytes: limits.InstanceBytes,
	}
	return c.check(ctx, r)
}

type xmlWellFormedChecker struct {
	doc           xmlDocument[struct{}]
	maxDepth      int
	maxAttributes int
	maxTokenBytes int64
	maxInputBytes int64
}

func (c *xmlWellFormedChecker) check(ctx context.Context, r io.Reader) error {
	done := ctx.Done()
	names := stream.NewCache()
	values := stream.NewCache()
	var parser stream.Parser
	if err := parser.ResetWithLimits(r, &names, &values, stream.Limits{
		Context:       ctx,
		MaxInputBytes: c.maxInputBytes,
		MaxTokenBytes: c.maxTokenBytes,
		MaxAttrs:      c.maxAttributes,
	}); err != nil {
		if done != nil {
			if contextErr := validationContextDoneError(ctx, done, err); contextErr != nil {
				return contextErr
			}
		}
		return instanceReaderError(err)
	}
	defer parser.Detach()
	parser.SetLazyAttrValue(true)
	for {
		tok, err := parser.Next()
		if done != nil {
			if contextErr := validationContextDoneError(ctx, done, err); contextErr != nil {
				return contextErr
			}
		}
		if err != nil {
			if stream.IsOnlyEOF(err) {
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
			return ValidateDirective(c.doc.context(tok.Line, tok.Column), tok.Directive)
		case stream.KindComment, stream.KindPI:
		}
	}
	if done != nil {
		if err := validationContextDoneError(ctx, done, nil); err != nil {
			return err
		}
	}
	return c.doc.Complete()
}

func (c *xmlWellFormedChecker) start(line, col int, se stream.StartElement, values *stream.Cache) error {
	translated, err := c.doc.PrepareStart(se, values, c.maxDepth, line, col)
	if err != nil {
		return err
	}
	c.doc.CommitStart(translated, false, struct{}{})
	return nil
}

func (c *xmlWellFormedChecker) end(line, col int, ee stream.EndElement) error {
	if err := c.doc.ValidateEnd(ee, line, col); err != nil {
		return err
	}
	return c.doc.CommitEnd()
}

func (c *xmlWellFormedChecker) chars(line, col int, data []byte, cdata bool) error {
	if c.doc.Depth() != 0 {
		return nil
	}
	return ValidateDocumentCharacterData(data, cdata, c.doc.context(line, col))
}

func (c *xmlWellFormedChecker) streamError(parser *stream.Parser, tok stream.Token, err error) error {
	line, col := tok.Line, tok.Column
	if line == 0 {
		line, col = parser.Pos()
	}
	return StreamError(line, col, c.doc.PathString(), err)
}
