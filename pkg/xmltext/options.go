package xmltext

import "io"

// Options holds decoder configuration values.
// The zero value means no overrides.
type Options struct {
	charsetReader         func(label string, r io.Reader) (io.Reader, error)
	entityMap             map[string]string
	resolveEntities       bool
	emitComments          bool
	emitPI                bool
	emitDirectives        bool
	trackLineColumn       bool
	coalesceCharData      bool
	maxDepth              int
	maxAttrs              int
	maxTokenSize          int
	maxQNameInternEntries int
	strict                bool
	debugPoisonSpans      bool
	bufferSize            int

	charsetReaderSet         bool
	entityMapSet             bool
	resolveEntitiesSet       bool
	emitCommentsSet          bool
	emitPISet                bool
	emitDirectivesSet        bool
	trackLineColumnSet       bool
	coalesceCharDataSet      bool
	maxDepthSet              bool
	maxAttrsSet              bool
	maxTokenSizeSet          bool
	maxQNameInternEntriesSet bool
	strictSet                bool
	debugPoisonSpansSet      bool
	bufferSizeSet            bool
}

// JoinOptions combines multiple option sets into one in declaration order.
// Later options override earlier ones when set.
func JoinOptions(srcs ...Options) Options {
	var merged Options
	for _, src := range srcs {
		merged.merge(src)
	}
	return merged
}

func (opts *Options) merge(src Options) {
	if src.charsetReaderSet {
		opts.charsetReader = src.charsetReader
		opts.charsetReaderSet = true
	}
	if src.entityMapSet {
		opts.entityMap = src.entityMap
		opts.entityMapSet = true
	}
	if src.resolveEntitiesSet {
		opts.resolveEntities = src.resolveEntities
		opts.resolveEntitiesSet = true
	}
	if src.emitCommentsSet {
		opts.emitComments = src.emitComments
		opts.emitCommentsSet = true
	}
	if src.emitPISet {
		opts.emitPI = src.emitPI
		opts.emitPISet = true
	}
	if src.emitDirectivesSet {
		opts.emitDirectives = src.emitDirectives
		opts.emitDirectivesSet = true
	}
	if src.trackLineColumnSet {
		opts.trackLineColumn = src.trackLineColumn
		opts.trackLineColumnSet = true
	}
	if src.coalesceCharDataSet {
		opts.coalesceCharData = src.coalesceCharData
		opts.coalesceCharDataSet = true
	}
	if src.maxDepthSet {
		opts.maxDepth = src.maxDepth
		opts.maxDepthSet = true
	}
	if src.maxAttrsSet {
		opts.maxAttrs = src.maxAttrs
		opts.maxAttrsSet = true
	}
	if src.maxTokenSizeSet {
		opts.maxTokenSize = src.maxTokenSize
		opts.maxTokenSizeSet = true
	}
	if src.maxQNameInternEntriesSet {
		opts.maxQNameInternEntries = src.maxQNameInternEntries
		opts.maxQNameInternEntriesSet = true
	}
	if src.strictSet {
		opts.strict = src.strict
		opts.strictSet = true
	}
	if src.debugPoisonSpansSet {
		opts.debugPoisonSpans = src.debugPoisonSpans
		opts.debugPoisonSpansSet = true
	}
	if src.bufferSizeSet {
		opts.bufferSize = src.bufferSize
		opts.bufferSizeSet = true
	}
}

// WithCharsetReader registers a decoder for non-UTF-8/UTF-16 encodings.
func WithCharsetReader(fn func(label string, r io.Reader) (io.Reader, error)) Options {
	return Options{charsetReader: fn, charsetReaderSet: true}
}

// WithEntityMap configures custom named entity replacements.
func WithEntityMap(values map[string]string) Options {
	if values == nil {
		return Options{entityMapSet: true}
	}
	copyMap := make(map[string]string, len(values))
	for key, value := range values {
		copyMap[key] = value
	}
	return Options{entityMap: copyMap, entityMapSet: true}
}

// ResolveEntities controls whether entity references are expanded.
func ResolveEntities(value bool) Options {
	return Options{resolveEntities: value, resolveEntitiesSet: true}
}

// EmitComments controls whether comment tokens are emitted.
func EmitComments(value bool) Options {
	return Options{emitComments: value, emitCommentsSet: true}
}

// EmitPI controls whether processing instruction tokens are emitted.
func EmitPI(value bool) Options {
	return Options{emitPI: value, emitPISet: true}
}

// EmitDirectives controls whether directive tokens are emitted.
func EmitDirectives(value bool) Options {
	return Options{emitDirectives: value, emitDirectivesSet: true}
}

// TrackLineColumn controls whether line and column tracking is enabled.
func TrackLineColumn(value bool) Options {
	return Options{trackLineColumn: value, trackLineColumnSet: true}
}

// CoalesceCharData merges adjacent text tokens into a single CharData token.
func CoalesceCharData(value bool) Options {
	return Options{coalesceCharData: value, coalesceCharDataSet: true}
}

// MaxDepth limits element nesting depth.
func MaxDepth(value int) Options {
	return Options{maxDepth: value, maxDepthSet: true}
}

// MaxAttrs limits the number of attributes on a start element.
func MaxAttrs(value int) Options {
	return Options{maxAttrs: value, maxAttrsSet: true}
}

// MaxTokenSize limits the maximum size of a single token in bytes.
// Tokens exactly MaxTokenSize bytes long are allowed.
func MaxTokenSize(value int) Options {
	return Options{maxTokenSize: value, maxTokenSizeSet: true}
}

// Strict enables XML declaration validation.
// It enforces version and encoding/standalone ordering and values.
func Strict(value bool) Options {
	return Options{strict: value, strictSet: true}
}

func debugPoisonSpans(value bool) Options {
	return Options{debugPoisonSpans: value, debugPoisonSpansSet: true}
}

func bufferSize(value int) Options {
	return Options{bufferSize: value, bufferSizeSet: true}
}

// FastValidation returns a preset tuned for validation throughput.
func FastValidation() Options {
	return JoinOptions(
		TrackLineColumn(false),
		ResolveEntities(false),
		MaxDepth(256),
	)
}
