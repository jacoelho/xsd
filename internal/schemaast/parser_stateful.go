package schemaast

import (
	"github.com/jacoelho/xsd/internal/xmlstream"
	"io"
	"slices"
)

// Parser parses schemas with reusable parse options and document pool.
type Parser struct {
	pool *DocumentPool
	opts []xmlstream.Option
}

// NewParser creates a parser with optional xmlstream parse options.
func NewParser(opts ...xmlstream.Option) *Parser {
	var parseOpts []xmlstream.Option
	if len(opts) > 0 {
		parseOpts = slices.Clone(opts)
	}
	return &Parser{
		pool: NewDocumentPool(),
		opts: parseOpts,
	}
}

// Parse parses one schema and returns directive metadata.
func (p *Parser) Parse(r io.Reader) (*ParseResult, error) {
	if p == nil {
		return ParseDocumentWithImportsOptions(r)
	}
	pool := p.pool
	if pool == nil {
		pool = NewDocumentPool()
		p.pool = pool
	}
	return ParseDocumentWithImportsOptionsWithPool(r, pool, p.opts...)
}
