package xmlstream

import (
	"io"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

// Option configures the xmlstream reader.
type Option struct {
	value xmltext.Options
}

func wrapOption(value xmltext.Options) Option {
	return Option{value: value}
}

func buildOptions(opts ...Option) []xmltext.Options {
	base := []Option{
		wrapOption(xmltext.ResolveEntities(false)),
		wrapOption(xmltext.CoalesceCharData(true)),
		wrapOption(xmltext.EmitComments(false)),
		wrapOption(xmltext.EmitPI(false)),
		wrapOption(xmltext.EmitDirectives(false)),
		wrapOption(xmltext.TrackLineColumn(true)),
		wrapOption(xmltext.MaxQNameInternEntries(qnameCacheMaxEntries)),
	}
	all := append(base, opts...)
	out := make([]xmltext.Options, 0, len(all))
	for _, opt := range all {
		out = append(out, opt.value)
	}
	return out
}

// JoinOptions merges xmlstream options into a single xmltext options struct.
func JoinOptions(opts ...Option) xmltext.Options {
	return xmltext.JoinOptions(buildOptions(opts...)...)
}

// CoalesceCharData merges adjacent text tokens into a single CharData event.
func CoalesceCharData(value bool) Option {
	return wrapOption(xmltext.CoalesceCharData(value))
}

// EmitComments controls whether comment events are emitted.
func EmitComments(value bool) Option {
	return wrapOption(xmltext.EmitComments(value))
}

// EmitPI controls whether processing instruction events are emitted.
func EmitPI(value bool) Option {
	return wrapOption(xmltext.EmitPI(value))
}

// EmitDirectives controls whether directive events are emitted.
func EmitDirectives(value bool) Option {
	return wrapOption(xmltext.EmitDirectives(value))
}

// TrackLineColumn controls whether line and column tracking is enabled.
func TrackLineColumn(value bool) Option {
	return wrapOption(xmltext.TrackLineColumn(value))
}

// WithCharsetReader registers a decoder for non-UTF-8/UTF-16 encodings.
func WithCharsetReader(fn func(label string, r io.Reader) (io.Reader, error)) Option {
	return wrapOption(xmltext.WithCharsetReader(fn))
}

// MaxDepth limits element nesting depth.
func MaxDepth(value int) Option {
	return wrapOption(xmltext.MaxDepth(value))
}

// MaxAttrs limits the number of attributes on a start element.
func MaxAttrs(value int) Option {
	return wrapOption(xmltext.MaxAttrs(value))
}

// MaxTokenSize limits the maximum size of a single token in bytes.
func MaxTokenSize(value int) Option {
	return wrapOption(xmltext.MaxTokenSize(value))
}

// MaxQNameInternEntries limits the QName cache size.
// Zero means no limit.
func MaxQNameInternEntries(value int) Option {
	return wrapOption(xmltext.MaxQNameInternEntries(value))
}
