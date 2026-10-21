package validate

import (
	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/xsderrors"
)

type validationIssue struct {
	code    xsderrors.Code
	message string
}

func (i validationIssue) valid() bool {
	return i.code != ""
}

type childStartPolicy struct {
	issue validationIssue
}

func childFramePolicy(parent *frame) childStartPolicy {
	if parent.Nilled {
		return childStartPolicy{issue: nilledContentIssue()}
	}
	return childStartPolicy{}
}

func childContentPolicy(typ xsdSchema.TypeID, simpleContent xsdSchema.SimpleTypeID, state xsdSchema.ContentState, name xsdSchema.RuntimeName) validationIssue {
	if !typ.IsComplex() {
		return validationIssue{code: xsderrors.CodeValidationContent, message: msgSimpleTypeChild}
	}
	if simpleContent != xsdSchema.NoSimpleType {
		return validationIssue{code: xsderrors.CodeValidationContent, message: msgSimpleContentChild}
	}
	if !state.HasModel() {
		return unexpectedChildIssue(name)
	}
	return validationIssue{}
}

func contentCompletionRequired(nilled bool, typ xsdSchema.TypeID, content xsdSchema.ContentState) bool {
	return !nilled && typ.IsComplex() && content.HasModel()
}
