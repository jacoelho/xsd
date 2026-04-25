package xmltext

func resolveOptions(opts *Options) decoderOptions {
	resolved := decoderOptions{trackLineColumn: true, bufferSize: defaultBufferSize}
	if opts == nil {
		return resolved
	}
	if opts.charsetReaderSet {
		resolved.charsetReader = opts.charsetReader
	}
	if opts.entityMapSet {
		resolved.entityMap = opts.entityMap
	}
	if opts.resolveEntitiesSet {
		resolved.resolveEntities = opts.resolveEntities
	}
	if opts.emitCommentsSet {
		resolved.emitComments = opts.emitComments
	}
	if opts.emitPISet {
		resolved.emitPI = opts.emitPI
	}
	if opts.emitDirectivesSet {
		resolved.emitDirectives = opts.emitDirectives
	}
	if opts.trackLineColumnSet {
		resolved.trackLineColumn = opts.trackLineColumn
	}
	if opts.coalesceCharDataSet {
		resolved.coalesceCharData = opts.coalesceCharData
	}
	if opts.maxDepthSet {
		resolved.maxDepth = normalizeLimit(opts.maxDepth)
	}
	if opts.maxAttrsSet {
		resolved.maxAttrs = normalizeLimit(opts.maxAttrs)
	}
	if opts.maxTokenSizeSet {
		resolved.maxTokenSize = normalizeLimit(opts.maxTokenSize)
	}
	if opts.maxQNameInternEntriesSet {
		resolved.maxQNameInternEntries = normalizeLimit(opts.maxQNameInternEntries)
	}
	if opts.strictSet {
		resolved.strict = opts.strict
	}
	if opts.debugPoisonSpansSet {
		resolved.debugPoisonSpans = opts.debugPoisonSpans
	}
	if opts.bufferSizeSet {
		resolved.bufferSize = normalizeLimit(opts.bufferSize)
	}
	if resolved.bufferSize == 0 {
		resolved.bufferSize = defaultBufferSize
	}
	return resolved
}

func normalizeLimit(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
