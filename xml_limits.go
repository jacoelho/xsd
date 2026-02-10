package xsd

import (
	"cmp"
	"fmt"

	"github.com/jacoelho/xsd/pkg/xmlstream"
	"github.com/jacoelho/xsd/pkg/xmltext"
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
		maxDepth:              cmp.Or(maxDepth, defaultXMLMaxDepth),
		maxAttrs:              cmp.Or(maxAttrs, defaultXMLMaxAttrs),
		maxTokenSize:          cmp.Or(maxTokenSize, defaultXMLMaxTokenSize),
		maxQNameInternEntries: maxQName,
	}, nil
}

func (l xmlParseLimits) options() []xmlstream.Option {
	opts := []xmlstream.Option{
		xmltext.MaxDepth(l.maxDepth),
		xmltext.MaxAttrs(l.maxAttrs),
		xmltext.MaxTokenSize(l.maxTokenSize),
	}
	if l.maxQNameInternEntries != 0 {
		opts = append(opts, xmltext.MaxQNameInternEntries(l.maxQNameInternEntries))
	}
	return opts
}
