package xsd

import (
	"cmp"
	"fmt"

	"github.com/jacoelho/xsd/pkg/xmlstream"
)

const (
	defaultXMLMaxDepth     = 256
	defaultXMLMaxAttrs     = 256
	defaultXMLMaxTokenSize = 4 << 20
)

type xmlParseLimits struct {
	maxDepth              int
	maxAttrs              int
	maxTokenSize          int
	maxQNameInternEntries int
}

func resolveXMLParseLimits(maxDepth, maxAttrs, maxTokenSize, maxQName int) (xmlParseLimits, error) {
	if maxDepth < 0 {
		return xmlParseLimits{}, fmt.Errorf("xml max depth must be >= 0")
	}
	if maxAttrs < 0 {
		return xmlParseLimits{}, fmt.Errorf("xml max attrs must be >= 0")
	}
	if maxTokenSize < 0 {
		return xmlParseLimits{}, fmt.Errorf("xml max token size must be >= 0")
	}
	if maxQName < 0 {
		return xmlParseLimits{}, fmt.Errorf("xml max qname intern entries must be >= 0")
	}
	return xmlParseLimits{
		maxDepth:              defaultXMLLimit(maxDepth, defaultXMLMaxDepth),
		maxAttrs:              defaultXMLLimit(maxAttrs, defaultXMLMaxAttrs),
		maxTokenSize:          defaultXMLLimit(maxTokenSize, defaultXMLMaxTokenSize),
		maxQNameInternEntries: maxQName,
	}, nil
}

func (l xmlParseLimits) options() []xmlstream.Option {
	depth := defaultXMLLimit(l.maxDepth, defaultXMLMaxDepth)
	attrs := defaultXMLLimit(l.maxAttrs, defaultXMLMaxAttrs)
	tokenSize := defaultXMLLimit(l.maxTokenSize, defaultXMLMaxTokenSize)

	opts := []xmlstream.Option{
		xmlstream.MaxDepth(depth),
		xmlstream.MaxAttrs(attrs),
		xmlstream.MaxTokenSize(tokenSize),
	}
	if l.maxQNameInternEntries != 0 {
		opts = append(opts, xmlstream.MaxQNameInternEntries(l.maxQNameInternEntries))
	}
	return opts
}

func defaultXMLLimit(value, fallback int) int {
	return cmp.Or(value, fallback)
}
