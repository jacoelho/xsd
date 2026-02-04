package xsd

import "github.com/jacoelho/xsd/pkg/xmlstream"

const (
	defaultXMLMaxDepth     = 256
	defaultXMLMaxAttrs     = 256
	defaultXMLMaxTokenSize = 4 << 20
)

func buildXMLParseOptions(maxDepth, maxAttrs, maxTokenSize, maxQName int) []xmlstream.Option {
	depth := defaultXMLLimit(maxDepth, defaultXMLMaxDepth)
	attrs := defaultXMLLimit(maxAttrs, defaultXMLMaxAttrs)
	tokenSize := defaultXMLLimit(maxTokenSize, defaultXMLMaxTokenSize)

	opts := []xmlstream.Option{
		xmlstream.MaxDepth(depth),
		xmlstream.MaxAttrs(attrs),
		xmlstream.MaxTokenSize(tokenSize),
	}
	if maxQName != 0 {
		opts = append(opts, xmlstream.MaxQNameInternEntries(maxQName))
	}
	return opts
}

func defaultXMLLimit(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}
