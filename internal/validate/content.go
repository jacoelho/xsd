package validate

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/xsderrors"
)

// DocumentCharacterData is character data encountered outside the root.
type DocumentCharacterData struct {
	Kind       stream.CharacterDataKind
	Whitespace bool
}

// ValidateDocumentCharacterData validates character data outside the root
// element. Element-owned content is validated directly by session.chars.
func ValidateDocumentCharacterData(input DocumentCharacterData, ctx StartContext) error {
	switch input.Kind {
	case stream.CharacterDataCDATA:
		return validation(ctx, xsderrors.CodeValidationXML, "CDATA section outside root element")
	case stream.CharacterDataReference:
		return validation(ctx, xsderrors.CodeValidationXML, "reference outside root element")
	case stream.CharacterDataText:
	case stream.CharacterDataInvalid:
		return xsderrors.InternalInvariant("character data kind is invalid")
	default:
		err := xsderrors.InternalInvariant("character data kind is invalid")
		return err
	}
	if input.Whitespace {
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
}

func childFramePolicy(parent *frame) childStartPolicy {
	if parent.Nilled {
		return childStartPolicy{issue: nilledContentIssue()}
	}
	return childStartPolicy{}
}

func childContentPolicy(typ runtime.TypeID, simpleContent runtime.SimpleTypeID, state runtime.ContentState, name runtime.RuntimeName) validationIssue {
	if !typ.IsComplex() {
		return validationIssue{code: xsderrors.CodeValidationContent, message: "simple type cannot contain child elements"}
	}
	if simpleContent != runtime.NoSimpleType {
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
	return !nilled && typ.IsComplex() && content.HasModel()
}

func validationFromIssue(ctx StartContext, issue validationIssue) error {
	return validation(ctx, issue.code, issue.message)
}
