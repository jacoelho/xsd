package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/xsderrors"
)

// DocumentCompleteInput reports end-of-document validation state.
type DocumentCompleteInput struct {
	Context      StartContext
	SeenRoot     bool
	OpenElements int
}

// ValidateDocumentComplete enforces document-level completion rules.
func ValidateDocumentComplete(in DocumentCompleteInput) error {
	if !in.SeenRoot {
		return validation(StartContext{}, xsderrors.CodeValidationRoot, "instance document has no root element")
	}
	if in.OpenElements != 0 {
		return validation(in.Context, xsderrors.CodeValidationXML, "unclosed element")
	}
	return nil
}

// DocumentElementStartInput reports document-level state before a start element.
type DocumentElementStartInput struct {
	Context      StartContext
	SeenRoot     bool
	OpenElements int
}

// ValidateDocumentElementStart rejects a second document element.
func ValidateDocumentElementStart(in DocumentElementStartInput) error {
	if in.SeenRoot && in.OpenElements == 0 {
		return validation(in.Context, xsderrors.CodeValidationXML, "multiple root elements")
	}
	return nil
}

// EndElementInput reports end-element state after namespace translation.
type EndElementInput struct {
	Name         xml.Name
	Expected     xml.Name
	Context      StartContext
	OpenElements int
}

// ValidateEndElement rejects unmatched or out-of-place end elements.
func ValidateEndElement(in EndElementInput) error {
	if in.OpenElements == 0 {
		return validation(in.Context, xsderrors.CodeValidationXML, "unexpected end element")
	}
	if in.Name != in.Expected {
		return validation(in.Context, xsderrors.CodeValidationXML, "end element </"+formatXMLName(in.Name)+"> does not match start element <"+formatXMLName(in.Expected)+">")
	}
	return nil
}

// ValidateNameResolution rejects an XML name whose namespace prefix did not
// resolve in the current document namespace scope.
func ValidateNameResolution(ctx StartContext, name xml.Name, resolved bool) error {
	if resolved {
		return nil
	}
	return validation(ctx, xsderrors.CodeValidationXML, "unbound namespace prefix "+name.Space)
}
