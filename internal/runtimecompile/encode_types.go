package runtimecompile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
)

func toRuntimeAttrUse(use types.AttributeUse) runtime.AttrUseKind {
	switch use {
	case types.Required:
		return runtime.AttrRequired
	case types.Prohibited:
		return runtime.AttrProhibited
	default:
		return runtime.AttrOptional
	}
}

func toRuntimeElemBlock(block types.DerivationSet) runtime.ElemBlock {
	var out runtime.ElemBlock
	if block.Has(types.DerivationSubstitution) {
		out |= runtime.ElemBlockSubstitution
	}
	if block.Has(types.DerivationExtension) {
		out |= runtime.ElemBlockExtension
	}
	if block.Has(types.DerivationRestriction) {
		out |= runtime.ElemBlockRestriction
	}
	return out
}

func toRuntimeDerivation(mask types.DerivationMethod) runtime.DerivationMethod {
	var out runtime.DerivationMethod
	if mask&types.DerivationExtension != 0 {
		out |= runtime.DerExtension
	}
	if mask&types.DerivationRestriction != 0 {
		out |= runtime.DerRestriction
	}
	if mask&types.DerivationList != 0 {
		out |= runtime.DerList
	}
	if mask&types.DerivationUnion != 0 {
		out |= runtime.DerUnion
	}
	return out
}

func toRuntimeDerivationSet(set types.DerivationSet) runtime.DerivationMethod {
	var out runtime.DerivationMethod
	if set.Has(types.DerivationExtension) {
		out |= runtime.DerExtension
	}
	if set.Has(types.DerivationRestriction) {
		out |= runtime.DerRestriction
	}
	if set.Has(types.DerivationList) {
		out |= runtime.DerList
	}
	if set.Has(types.DerivationUnion) {
		out |= runtime.DerUnion
	}
	return out
}

func toRuntimeProcessContents(pc types.ProcessContents) runtime.ProcessContents {
	switch pc {
	case types.Lax:
		return runtime.PCLax
	case types.Skip:
		return runtime.PCSkip
	default:
		return runtime.PCStrict
	}
}
