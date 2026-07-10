package validate

import (
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// ValidateDocumentCharacterData validates character data outside the root
// element. Element-owned content is validated directly by session.chars.
func ValidateDocumentCharacterData(data []byte, cdata bool, ctx StartContext) error {
	if cdata {
		return validation(ctx, xsderrors.CodeValidationXML, "CDATA section outside root element")
	}
	if lex.IsXMLWhitespaceBytes(data) {
		return nil
	}
	return validation(ctx, xsderrors.CodeValidationText, "text outside root element")
}

type validationIssue struct {
	code    xsderrors.Code
	message string
}

func (i validationIssue) valid() bool {
	return i.code != ""
}

type childStartPolicy struct {
	issue validationIssue
	skip  bool
}

func childFramePolicy(skip, nilled bool) childStartPolicy {
	if skip {
		return childStartPolicy{skip: true}
	}
	if nilled {
		return childStartPolicy{issue: nilledContentIssue()}
	}
	return childStartPolicy{}
}

func childContentPolicy(content runtime.ChildContentInfo, state runtime.ContentState, name runtime.RuntimeName) validationIssue {
	if !content.Complex {
		return validationIssue{code: xsderrors.CodeValidationContent, message: "simple type cannot contain child elements"}
	}
	if content.Simple {
		return validationIssue{code: xsderrors.CodeValidationContent, message: "simple content cannot contain child elements"}
	}
	if !state.HasModel() {
		return unexpectedChildIssue(name)
	}
	return validationIssue{}
}

func unexpectedChildIssue(name runtime.RuntimeName) validationIssue {
	return validationIssue{code: xsderrors.CodeValidationElement, message: "unexpected child element " + name.Label()}
}

func strictMissingChildIssue(name runtime.RuntimeName) validationIssue {
	return validationIssue{code: xsderrors.CodeValidationElement, message: "wildcard requires declared element " + name.Label()}
}

func nilledContentIssue() validationIssue {
	return validationIssue{code: xsderrors.CodeValidationNil, message: "nilled element must be empty"}
}

func missingRequiredChildIssue() validationIssue {
	return validationIssue{code: xsderrors.CodeValidationContent, message: "missing required child element"}
}

func contentCompletionRequired(nilled bool, typ runtime.TypeID, content runtime.ContentState) bool {
	return !nilled && typ.Kind == runtime.TypeComplex && content.HasModel()
}

func validationFromIssue(ctx StartContext, issue validationIssue) error {
	return validation(ctx, issue.code, issue.message)
}
