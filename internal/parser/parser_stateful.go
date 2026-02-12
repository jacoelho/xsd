package parser

import (
	"io"

	"github.com/jacoelho/xsd/internal/xmltree"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// Parser parses schemas with reusable parse options and document pool.
type Parser struct {
	pool *xmltree.DocumentPool
	opts []xmlstream.Option
}

// NewParser creates a parser with optional xmlstream parse options.
func NewParser(opts ...xmlstream.Option) *Parser {
	return &Parser{
		pool: xmltree.NewDocumentPool(),
		opts: append([]xmlstream.Option(nil), opts...),
	}
}

// ParseWithOptions parses one schema and returns directive metadata.
func ParseWithOptions(r io.Reader, opts ...xmlstream.Option) (*ParseResult, error) {
	return ParseWithImportsOptions(r, opts...)
}

// ParseWithOptionsWithPool parses one schema with an explicit document pool.
func ParseWithOptionsWithPool(r io.Reader, pool *xmltree.DocumentPool, opts ...xmlstream.Option) (*ParseResult, error) {
	return ParseWithImportsOptionsWithPool(r, pool, opts...)
}

// Parse parses one schema and returns directive metadata.
func (p *Parser) Parse(r io.Reader) (*ParseResult, error) {
	if p == nil {
		return ParseWithOptions(r)
	}
	pool := p.pool
	if pool == nil {
		pool = xmltree.NewDocumentPool()
		p.pool = pool
	}
	return ParseWithOptionsWithPool(r, pool, p.opts...)
}
