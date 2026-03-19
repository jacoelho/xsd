package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
)

func attrValidationSpecFromAttrUse(use runtime.AttrUse) attrs.ValueSpec {
	return attrs.SpecFromUse(use)
}

func attrValidationSpecFromRuntimeAttribute(attr runtime.Attribute) attrs.ValueSpec {
	return attrs.SpecFromAttribute(attr)
}
