package valruntime

import "github.com/jacoelho/xsd/internal/runtime"

// Options controls whitespace, canonicalization, storage, and key derivation
// for one value-validation request.
type Options struct {
	ApplyWhitespace  bool
	TrackIDs         bool
	RequireCanonical bool
	StoreValue       bool
	NeedKey          bool
}

// Plan summarizes the runtime work needed for one validator request.
type Plan struct {
	NeedCanonical           bool
	NeedKey                 bool
	NeedLocalMetrics        bool
	UseScratchNormalization bool
	CloneCanonical          bool
}

// Build derives one execution plan from validator metadata and request options.
func Build(meta runtime.ValidatorMeta, opts Options, hasLengthFacet bool) Plan {
	needEnumKey := meta.Flags&runtime.ValidatorHasEnum != 0
	needLocalMetrics := needEnumKey
	if !needLocalMetrics && (meta.Kind == runtime.VHexBinary || meta.Kind == runtime.VBase64Binary) {
		needLocalMetrics = hasLengthFacet
	}
	if !needLocalMetrics && opts.TrackIDs && meta.Kind == runtime.VUnion && meta.Flags&runtime.ValidatorMayTrackIDs != 0 {
		needLocalMetrics = true
	}

	needCanonical := opts.RequireCanonical || meta.Facets.Len != 0 || meta.Kind == runtime.VUnion || meta.Kind == runtime.VQName || meta.Kind == runtime.VNotation
	if opts.StoreValue || opts.NeedKey {
		needCanonical = true
	}

	return Plan{
		NeedCanonical:           needCanonical,
		NeedKey:                 opts.NeedKey || opts.StoreValue || needEnumKey,
		NeedLocalMetrics:        needLocalMetrics,
		UseScratchNormalization: opts.ApplyWhitespace && (meta.Kind == runtime.VList || meta.Kind == runtime.VUnion),
		CloneCanonical:          !opts.StoreValue && (meta.Kind == runtime.VHexBinary || meta.Kind == runtime.VBase64Binary),
	}
}
