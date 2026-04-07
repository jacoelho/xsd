package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

func attrValidationSpecFromAttrUse(use runtime.AttrUse) ValueSpec {
	return SpecFromUse(use)
}

func attrValidationSpecFromRuntimeAttribute(attr runtime.Attribute) ValueSpec {
	return SpecFromAttribute(attr)
}
