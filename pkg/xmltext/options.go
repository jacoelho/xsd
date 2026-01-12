package xmltext

import "io"

// Options holds decoder configuration values.
// The zero value means no overrides.
type Options struct {
	charsetReader             func(label string, r io.Reader) (io.Reader, error)
	entityMap                 map[string]string
	resolveEntities           bool
	emitComments              bool
	emitPI                    bool
	emitDirectives            bool
	trackLineColumn           bool
	coalesceCharData          bool
	maxDepth                  int
	maxAttrs                  int
	maxTokenSize              int
	maxQNameInternEntries     int
	maxNamespaceInternEntries int
	debugPoisonSpans          bool
	bufferSize                int

	charsetReaderSet             bool
	entityMapSet                 bool
	resolveEntitiesSet           bool
	emitCommentsSet              bool
	emitPISet                    bool
	emitDirectivesSet            bool
	trackLineColumnSet           bool
	coalesceCharDataSet          bool
	maxDepthSet                  bool
	maxAttrsSet                  bool
	maxTokenSizeSet              bool
	maxQNameInternEntriesSet     bool
	maxNamespaceInternEntriesSet bool
	debugPoisonSpansSet          bool
	bufferSizeSet                bool
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
	if src.maxNamespaceInternEntriesSet {
		opts.maxNamespaceInternEntries = src.maxNamespaceInternEntries
		opts.maxNamespaceInternEntriesSet = true
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
func MaxTokenSize(value int) Options {
	return Options{maxTokenSize: value, maxTokenSizeSet: true}
}

// MaxQNameInternEntries limits the QName interner cache size.
func MaxQNameInternEntries(value int) Options {
	return Options{maxQNameInternEntries: value, maxQNameInternEntriesSet: true}
}

// MaxNamespaceInternEntries limits the namespace interner cache size.
func MaxNamespaceInternEntries(value int) Options {
	return Options{maxNamespaceInternEntries: value, maxNamespaceInternEntriesSet: true}
}

// DebugPoisonSpans invalidates spans after the next decoder call.
func DebugPoisonSpans(value bool) Options {
	return Options{debugPoisonSpans: value, debugPoisonSpansSet: true}
}

// BufferSize sets the initial decoder buffer capacity in bytes.
func BufferSize(value int) Options {
	return Options{bufferSize: value, bufferSizeSet: true}
}

// CharsetReader returns the configured charset reader, if any.
func (opts Options) CharsetReader() (func(label string, r io.Reader) (io.Reader, error), bool) {
	return opts.charsetReader, opts.charsetReaderSet
}

// EntityMap returns the configured custom entity map, if any.
func (opts Options) EntityMap() (map[string]string, bool) {
	return opts.entityMap, opts.entityMapSet
}

// ResolveEntities reports whether entity expansion is configured.
func (opts Options) ResolveEntities() (bool, bool) {
	return opts.resolveEntities, opts.resolveEntitiesSet
}

// EmitComments reports whether comment emission is configured.
func (opts Options) EmitComments() (bool, bool) {
	return opts.emitComments, opts.emitCommentsSet
}

// EmitPI reports whether processing instruction emission is configured.
func (opts Options) EmitPI() (bool, bool) {
	return opts.emitPI, opts.emitPISet
}

// EmitDirectives reports whether directive emission is configured.
func (opts Options) EmitDirectives() (bool, bool) {
	return opts.emitDirectives, opts.emitDirectivesSet
}

// TrackLineColumn reports whether line/column tracking is configured.
func (opts Options) TrackLineColumn() (bool, bool) {
	return opts.trackLineColumn, opts.trackLineColumnSet
}

// CoalesceCharData reports whether char data coalescing is configured.
func (opts Options) CoalesceCharData() (bool, bool) {
	return opts.coalesceCharData, opts.coalesceCharDataSet
}

// MaxDepth reports whether a max depth is configured.
func (opts Options) MaxDepth() (int, bool) {
	return opts.maxDepth, opts.maxDepthSet
}

// MaxAttrs reports whether a max attribute count is configured.
func (opts Options) MaxAttrs() (int, bool) {
	return opts.maxAttrs, opts.maxAttrsSet
}

// MaxTokenSize reports whether a max token size is configured.
func (opts Options) MaxTokenSize() (int, bool) {
	return opts.maxTokenSize, opts.maxTokenSizeSet
}

// MaxQNameInternEntries reports whether a max QName interner size is configured.
func (opts Options) MaxQNameInternEntries() (int, bool) {
	return opts.maxQNameInternEntries, opts.maxQNameInternEntriesSet
}

// MaxNamespaceInternEntries reports whether a max namespace interner size is configured.
func (opts Options) MaxNamespaceInternEntries() (int, bool) {
	return opts.maxNamespaceInternEntries, opts.maxNamespaceInternEntriesSet
}

// DebugPoisonSpans reports whether span poisoning is configured.
func (opts Options) DebugPoisonSpans() (bool, bool) {
	return opts.debugPoisonSpans, opts.debugPoisonSpansSet
}

// BufferSize reports whether a buffer size is configured.
func (opts Options) BufferSize() (int, bool) {
	return opts.bufferSize, opts.bufferSizeSet
}

// FastValidation is a preset tuned for validation throughput.
var FastValidation = JoinOptions(
	TrackLineColumn(false),
	ResolveEntities(false),
	MaxDepth(256),
)
