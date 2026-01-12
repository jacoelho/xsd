package xmltext

import (
	"io"

	"github.com/jacoelho/xsd/pkg/xmlopts"
)

// Options holds decoder configuration values.
type Options = xmlopts.Options

// JoinOptions combines multiple option sets into one.
func JoinOptions(srcs ...Options) Options {
	return xmlopts.JoinOptions(srcs...)
}

// GetOption retrieves the last option value associated with the constructor.
func GetOption[T any](opts Options, constructor func(T) Options) (T, bool) {
	return xmlopts.GetOption(opts, constructor)
}

type charsetReaderOption func(label string, r io.Reader) (io.Reader, error)

type entityMapOption map[string]string

type resolveEntitiesOption bool

type emitCommentsOption bool

type emitPIOption bool

type emitDirectivesOption bool

type trackLineColumnOption bool

type coalesceCharDataOption bool

type maxDepthOption int

type maxAttrsOption int

type maxTokenSizeOption int

type maxQNameInternEntriesOption int

type maxNamespaceInternEntriesOption int

type debugPoisonSpansOption bool

type bufferSizeOption int

// WithCharsetReader registers a decoder for non-UTF-8/UTF-16 encodings.
func WithCharsetReader(fn func(label string, r io.Reader) (io.Reader, error)) Options {
	return xmlopts.New(charsetReaderOption(fn))
}

// WithEntityMap configures custom named entity replacements.
func WithEntityMap(values map[string]string) Options {
	if values == nil {
		return xmlopts.New(entityMapOption(nil))
	}
	copyMap := make(map[string]string, len(values))
	for key, value := range values {
		copyMap[key] = value
	}
	return xmlopts.New(entityMapOption(copyMap))
}

// ResolveEntities controls whether entity references are expanded.
func ResolveEntities(value bool) Options {
	return xmlopts.New(resolveEntitiesOption(value))
}

// EmitComments controls whether comment tokens are emitted.
func EmitComments(value bool) Options {
	return xmlopts.New(emitCommentsOption(value))
}

// EmitPI controls whether processing instruction tokens are emitted.
func EmitPI(value bool) Options {
	return xmlopts.New(emitPIOption(value))
}

// EmitDirectives controls whether directive tokens are emitted.
func EmitDirectives(value bool) Options {
	return xmlopts.New(emitDirectivesOption(value))
}

// TrackLineColumn controls whether line and column tracking is enabled.
func TrackLineColumn(value bool) Options {
	return xmlopts.New(trackLineColumnOption(value))
}

// CoalesceCharData merges adjacent text tokens into a single CharData token.
func CoalesceCharData(value bool) Options {
	return xmlopts.New(coalesceCharDataOption(value))
}

// MaxDepth limits element nesting depth.
func MaxDepth(value int) Options {
	return xmlopts.New(maxDepthOption(value))
}

// MaxAttrs limits the number of attributes on a start element.
func MaxAttrs(value int) Options {
	return xmlopts.New(maxAttrsOption(value))
}

// MaxTokenSize limits the maximum size of a single token in bytes.
func MaxTokenSize(value int) Options {
	return xmlopts.New(maxTokenSizeOption(value))
}

// MaxQNameInternEntries limits the QName interner cache size.
func MaxQNameInternEntries(value int) Options {
	return xmlopts.New(maxQNameInternEntriesOption(value))
}

// MaxNamespaceInternEntries limits the namespace interner cache size.
func MaxNamespaceInternEntries(value int) Options {
	return xmlopts.New(maxNamespaceInternEntriesOption(value))
}

// DebugPoisonSpans invalidates spans after the next decoder call.
func DebugPoisonSpans(value bool) Options {
	return xmlopts.New(debugPoisonSpansOption(value))
}

// BufferSize sets the initial decoder buffer capacity in bytes.
func BufferSize(value int) Options {
	return xmlopts.New(bufferSizeOption(value))
}

// FastValidation is a preset tuned for validation throughput.
var FastValidation = JoinOptions(
	TrackLineColumn(false),
	ResolveEntities(false),
	MaxDepth(256),
)
