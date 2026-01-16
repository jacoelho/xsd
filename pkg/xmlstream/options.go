package xmlstream

import (
	"io"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

// Option configures the xmlstream reader.
type Option = xmltext.Options

func buildOptions(opts ...Option) []xmltext.Options {
	base := []xmltext.Options{
		xmltext.ResolveEntities(false),
		xmltext.CoalesceCharData(true),
		xmltext.EmitComments(false),
		xmltext.EmitPI(false),
		xmltext.EmitDirectives(false),
		xmltext.TrackLineColumn(true),
		xmltext.MaxQNameInternEntries(qnameCacheMaxEntries),
	}
	if len(opts) == 0 {
		return base
	}
	out := make([]xmltext.Options, 0, len(base)+len(opts))
	out = append(out, base...)
	out = append(out, opts...)
	return out
}

// CoalesceCharData merges adjacent text tokens into a single CharData event.
func CoalesceCharData(value bool) Option {
	return xmltext.CoalesceCharData(value)
}

// EmitComments controls whether comment events are emitted.
func EmitComments(value bool) Option {
	return xmltext.EmitComments(value)
}

// EmitPI controls whether processing instruction events are emitted.
func EmitPI(value bool) Option {
	return xmltext.EmitPI(value)
}

// EmitDirectives controls whether directive events are emitted.
func EmitDirectives(value bool) Option {
	return xmltext.EmitDirectives(value)
}

// TrackLineColumn controls whether line and column tracking is enabled.
func TrackLineColumn(value bool) Option {
	return xmltext.TrackLineColumn(value)
}

// WithCharsetReader registers a decoder for non-UTF-8/UTF-16 encodings.
func WithCharsetReader(fn func(label string, r io.Reader) (io.Reader, error)) Option {
	return xmltext.WithCharsetReader(fn)
}

// MaxDepth limits element nesting depth.
func MaxDepth(value int) Option {
	return xmltext.MaxDepth(value)
}

// MaxAttrs limits the number of attributes on a start element.
func MaxAttrs(value int) Option {
	return xmltext.MaxAttrs(value)
}

// MaxTokenSize limits the maximum size of a single token in bytes.
func MaxTokenSize(value int) Option {
	return xmltext.MaxTokenSize(value)
}

// MaxQNameInternEntries limits the QName cache size.
// Zero means no limit.
func MaxQNameInternEntries(value int) Option {
	return xmltext.MaxQNameInternEntries(value)
}
