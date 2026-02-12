package runtimeassemble

import (
	"github.com/jacoelho/xsd/internal/runtime"
	model "github.com/jacoelho/xsd/internal/types"
)

func toRuntimeAttrUse(use model.AttributeUse) runtime.AttrUseKind {
	switch use {
	case model.Required:
		return runtime.AttrRequired
	case model.Prohibited:
		return runtime.AttrProhibited
	default:
		return runtime.AttrOptional
	}
}

func toRuntimeElemBlock(block model.DerivationSet) runtime.ElemBlock {
	var out runtime.ElemBlock
	if block.Has(model.DerivationSubstitution) {
		out |= runtime.ElemBlockSubstitution
	}
	if block.Has(model.DerivationExtension) {
		out |= runtime.ElemBlockExtension
	}
	if block.Has(model.DerivationRestriction) {
		out |= runtime.ElemBlockRestriction
	}
	return out
}

func toRuntimeDerivation(mask model.DerivationMethod) runtime.DerivationMethod {
	var out runtime.DerivationMethod
	if mask&model.DerivationExtension != 0 {
		out |= runtime.DerExtension
	}
	if mask&model.DerivationRestriction != 0 {
		out |= runtime.DerRestriction
	}
	if mask&model.DerivationList != 0 {
		out |= runtime.DerList
	}
	if mask&model.DerivationUnion != 0 {
		out |= runtime.DerUnion
	}
	return out
}

func toRuntimeDerivationSet(set model.DerivationSet) runtime.DerivationMethod {
	var out runtime.DerivationMethod
	if set.Has(model.DerivationExtension) {
		out |= runtime.DerExtension
	}
	if set.Has(model.DerivationRestriction) {
		out |= runtime.DerRestriction
	}
	if set.Has(model.DerivationList) {
		out |= runtime.DerList
	}
	if set.Has(model.DerivationUnion) {
		out |= runtime.DerUnion
	}
	return out
}

func toRuntimeProcessContents(pc model.ProcessContents) runtime.ProcessContents {
	switch pc {
	case model.Lax:
		return runtime.PCLax
	case model.Skip:
		return runtime.PCSkip
	default:
		return runtime.PCStrict
	}
}
