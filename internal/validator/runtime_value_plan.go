package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

// valueExecutionPlan captures the execution work needed for one value request.
type valueExecutionPlan struct {
	NeedCanonical           bool
	NeedKey                 bool
	NeedLocalMetrics        bool
	UseScratchNormalization bool
	CloneCanonical          bool
}

func buildValueExecutionPlan(meta runtime.ValidatorMeta, opts valueOptions, hasLengthFacet bool) valueExecutionPlan {
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

	return valueExecutionPlan{
		NeedCanonical:           needCanonical,
		NeedKey:                 opts.NeedKey || opts.StoreValue || needEnumKey,
		NeedLocalMetrics:        needLocalMetrics,
		UseScratchNormalization: opts.ApplyWhitespace && (meta.Kind == runtime.VList || meta.Kind == runtime.VUnion),
		CloneCanonical:          !opts.StoreValue && (meta.Kind == runtime.VHexBinary || meta.Kind == runtime.VBase64Binary),
	}
}
