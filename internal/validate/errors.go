package validate

import (
	"encoding/xml"
	"errors"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	xsdValue "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

// Message construction belongs here; assessment and recovery stay with their
// state owners. Call diagnostic formatting only after a check has failed.
const (
	elementNotNillableMessage     = "xsi:nil is not allowed because the element declaration is not nillable"
	msgNilledFixed                = "xsi:nil cannot be true because the element has a fixed value"
	msgAbstractType               = "the selected complex type is abstract; use a concrete derived xsi:type"
	msgNilledContent              = "an element with xsi:nil=\"true\" must have no text or child elements"
	msgSimpleTypeChild            = "child elements are not allowed because the element has a simple type"
	msgSimpleContentChild         = "child elements are not allowed because the element has simple content"
	msgEmptyText                  = "character data, including whitespace, is not allowed because the content type is empty"
	msgElementOnlyText            = "non-whitespace character data is not allowed because the content type is element-only"
	msgAbstractElement            = "the element declaration is abstract; use a concrete substitution-group member"
	msgSchemaLocationPairs        = "xsi:schemaLocation must contain namespace/location pairs"
	msgSessionInUse               = "validation session is already in use"
	msgReaderNil                  = "instance reader is nil"
	msgInstanceUTF8               = "instance documents must be UTF-8"
	msgExternalEntity             = "external or undeclared entity resolution is not supported"
	msgTextOutsideRoot            = "text outside root element"
	msgCDATAOutsideRoot           = "CDATA section outside root element"
	msgReferenceOutsideRoot       = "reference outside root element"
	msgMultipleRoots              = "multiple root elements"
	msgUnexpectedEnd              = "unexpected end element"
	msgNoRoot                     = "instance document has no root element"
	msgUnclosedElement            = "unclosed element"
	msgSimpleValueLimit           = "simple value validation limit exceeded"
	msgInvalidSimpleContent       = "invalid simple content"
	msgInvalidXSINil              = "invalid xsi:nil value"
	msgInvalidXSIType             = "invalid xsi:type"
	msgContentExpectationMetadata = "content expectation metadata is invalid"
	optionMaxInstanceBytes        = "MaxInstanceBytes"
	optionMaxInstanceTokenBytes   = "MaxInstanceTokenBytes"
	optionMaxInstanceAttributes   = "MaxInstanceAttributes"
)

func validation(ctx StartContext, code xsderrors.Code, msg string) error {
	return xsderrors.WithLocation(ctx.PathString(), ctx.Line, ctx.Column, xsderrors.Validation(code, msg, nil))
}

func (s *session) locateElementStartError(err error) error {
	location, locationErr := s.reader.StartLocation()
	if locationErr != nil {
		return xsderrors.InternalInvariant("element-start diagnostic requires an admitted XML start: " + locationErr.Error())
	}
	parent := s.doc.PathString()
	mode := expandedInstancePath(elementAssessed, s.runtimeName(location.Name))
	var path strings.Builder
	path.Grow(len(parent) + xmlPathSegmentLength(location.Name, mode))
	if parent != "/" {
		path.WriteString(parent)
	}
	appendXMLPathSegment(&path, location.Name, mode)
	return xsderrors.WithLocation(path.String(), location.Line, location.Column, err)
}

func quotedName(name xsdSchema.RuntimeName) string {
	if name.NS != "" || name.Local != "" {
		return diagnostictext.ExpandedName(name.NS, name.Local)
	}
	return diagnostictext.Quote(name.Label())
}
func invalidAttributeMessage(name xsdSchema.RuntimeName) string {
	return "invalid attribute " + quotedName(name)
}
func invalidWildcardAttributeMessage(name xsdSchema.RuntimeName) string {
	return "invalid wildcard attribute " + quotedName(name)
}
func undeclaredAttributeMessage(name xsdSchema.RuntimeName) string {
	return "attribute " + quotedName(name) + " is not allowed by the element's attribute declarations or wildcard"
}
func strictAttributeMessage(name xsdSchema.RuntimeName) string {
	return "attribute " + quotedName(name) + " matches a strict wildcard, but no global attribute declaration is available"
}
func fixedAttributeMessage(label, actual, expected string) string {
	return "attribute " + diagnostictext.Quote(label) + " has value " + diagnostictext.Quote(actual) + "; expected fixed value " + diagnostictext.Quote(expected)
}
func fixedElementMessage[T ~string | ~[]byte](actual T, expected string) string {
	return "element value " + diagnostictext.Quote(actual) + " does not match fixed value " + diagnostictext.Quote(expected)
}
func invalidNilMessage(actual string) string {
	return "xsi:nil has value " + diagnostictext.Quote(actual) + "; expected one of [\"true\", \"false\", \"1\", \"0\"]"
}
func invalidURIMessage(attribute, actual string) string {
	return "invalid " + attribute + " URI " + diagnostictext.Quote(actual)
}
func optionNegativeMessage(name string, actual int64) string {
	return name + " must be non-negative; got " + strconv.FormatInt(actual, 10)
}
func limitMessage(name, option string, observed, configured int64) string {
	return name + " limit exceeded: " + strconv.FormatInt(observed, 10) + " exceeds " + option + "=" + strconv.FormatInt(configured, 10)
}

func unexpectedChildIssue(name xsdSchema.RuntimeName) validationIssue {
	return validationIssue{code: xsderrors.CodeValidationElement, message: "unexpected child element " + quotedName(name) + "; no child elements are allowed here"}
}
func strictMissingChildIssue(name xsdSchema.RuntimeName) validationIssue {
	return validationIssue{code: xsderrors.CodeValidationElement, message: "element " + quotedName(name) + " matches a strict wildcard, but no global element declaration or xsi:type is available"}
}
func nilledContentIssue() validationIssue {
	return validationIssue{code: xsderrors.CodeValidationNil, message: msgNilledContent}
}

func contentExpectationMessage(expected xsdSchema.ContentExpectations) string {
	var text strings.Builder
	text.WriteByte('[')
	for i := range expected.Len() {
		if i != 0 {
			text.WriteString(", ")
		}
		item, _ := expected.At(i)
		if item.IsWildcard {
			text.WriteString(wildcardExpectationMessage(item))
		} else {
			text.WriteString(diagnostictext.ExpandedName(item.Name.Namespace, item.Name.Local))
		}
	}
	if expected.More {
		if expected.Len() != 0 {
			text.WriteString(", ")
		}
		text.WriteString("...")
	}
	text.WriteByte(']')
	return text.String()
}
func wildcardExpectationMessage(item xsdSchema.ContentExpectation) string {
	switch item.WildcardMode {
	case xsdSchema.WildcardAny:
		return "any element"
	case xsdSchema.WildcardLocal:
		return "an element with no namespace"
	case xsdSchema.WildcardOther:
		return "an element in a non-empty namespace other than " + diagnostictext.Quote(item.ExcludedNamespace)
	case xsdSchema.WildcardTargetNamespace, xsdSchema.WildcardList:
		var b strings.Builder
		b.WriteString("an element in namespace [")
		for i, namespace := range item.Namespaces {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(diagnostictext.Quote(namespace))
		}
		if item.MoreNamespaces {
			b.WriteString(", ...")
		}
		b.WriteByte(']')
		return b.String()
	default:
		return "an element matching the namespace wildcard"
	}
}

func invalidXSITypeQNameMessage(lexical string) string {
	return "xsi:type value " + diagnostictext.Quote(lexical) + " is not a valid QName or uses an undeclared namespace prefix"
}

func unknownXSITypeMessage(lexical string, name xsdValue.ExpandedName) string {
	return "xsi:type " + diagnostictext.Quote(lexical) + " resolves to " + diagnostictext.ExpandedName(name.Namespace, name.Local) + ", but no global type declaration is available"
}
func typeDiagnosticLabel(rt *xsdSchema.Schema, id xsdSchema.TypeID) string {
	if name, ok := rt.DiagnosticTypeName(id); ok {
		return diagnostictext.ExpandedName(name.Namespace, name.Local)
	}
	return "(anonymous)"
}
func xsiTypeDerivationMessage(rt *xsdSchema.Schema, actual, declared xsdSchema.TypeID) string {
	return "xsi:type " + typeDiagnosticLabel(rt, actual) + " is not validly derived from declared type " + typeDiagnosticLabel(rt, declared)
}
func xsiTypeBlockedByDeclarationMessage(rt *xsdSchema.Schema, actual xsdSchema.TypeID, method string) string {
	return "xsi:type " + typeDiagnosticLabel(rt, actual) + " uses " + method + ", which the element declaration blocks"
}
func xsiTypeBlockedByTypeMessage(rt *xsdSchema.Schema, actual, declared xsdSchema.TypeID, method string) string {
	return "xsi:type " + typeDiagnosticLabel(rt, actual) + " uses " + method + ", which declared type " + typeDiagnosticLabel(rt, declared) + " blocks"
}
func identityConstraintLabel(rt *xsdSchema.Schema, id xsdSchema.IdentityConstraintID) string {
	if name, ok := rt.DiagnosticIdentityName(id); ok {
		return "identity constraint " + diagnostictext.ExpandedName(name.Namespace, name.Local)
	}
	return "identity constraint"
}
func identityFieldMessage(rt *xsdSchema.Schema, id xsdSchema.IdentityConstraintID, index int, problem string) string {
	return identityConstraintLabel(rt, id) + ", field " + strconv.Itoa(index+1) + ": " + problem
}
func identityDuplicateMessage(rt *xsdSchema.Schema, id xsdSchema.IdentityConstraintID, path string) string {
	return "duplicate value for " + identityConstraintLabel(rt, id) + "; first seen at " + path
}
func identityReferenceMessage(rt *xsdSchema.Schema, id xsdSchema.IdentityConstraintID) string {
	return "keyref value does not match any value of referenced " + identityConstraintLabel(rt, id)
}

func missingKeyFieldMessage(rt *xsdSchema.Schema, id xsdSchema.IdentityConstraintID, fields []identityFieldValue) string {
	for index, field := range fields {
		if field.state == identityFieldAbsent || field.state == identityFieldNilled {
			return identityFieldMessage(rt, id, index, "a key requires a value, but the selected node is absent or nilled")
		}
	}
	return identityConstraintLabel(rt, id) + ": a key field is missing"
}

const (
	msgIdentityMultipleNodes = "selects multiple nodes; at most one is allowed"
	msgIdentityNoSimpleValue = "selects a node without a simple value"
	msgIdentityNillableKey   = "a key field must not select a nillable element declaration"
	msgMultipleIDAttributes  = "an element must not have more than one attribute of type xs:ID"
	msgWildcardIDDeclaredUse = "wildcard ID attribute conflicts with a declared ID attribute use"
	msgInstanceDepthLimit    = "instance depth limit exceeded"
)

func duplicateAttributeMessage(name xsdSchema.RuntimeName) string {
	return "duplicate attribute " + quotedName(name)
}
func simpleTypeAttributeMessage(name xsdSchema.RuntimeName) string {
	return "attribute " + quotedName(name) + " is not allowed because the element has a simple type"
}
func unavailableAttributeTypeMessage(name string) string {
	return "attribute " + diagnostictext.Quote(name) + " has a type whose schema declaration was not loaded"
}
func requiredAttributeMessage(name string) string {
	return "missing required attribute " + diagnostictext.Quote(name)
}
func undeclaredRootMessage(name xml.Name) string {
	return "root element " + diagnostictext.ExpandedName(name.Space, name.Local) + " has no global declaration in the compiled schema"
}
func fixedChildContentMessage(expected string) string {
	return "element has child content but requires fixed value " + diagnostictext.Quote(expected)
}
func unresolvedIDREFMessage(value string) string {
	return "IDREF " + diagnostictext.Quote(value) + " does not match any xs:ID value in the document"
}
func duplicateIDMessage(value, path string) string {
	return "duplicate ID " + diagnostictext.Quote(value) + "; first seen at " + path
}

func (s *session) unexpectedContentIssue(parent *frame, name xsdSchema.RuntimeName) validationIssue {
	scratch := s.contentScratch(parent)
	expected, valid := s.rt.ExpectedContent(parent.Content, &scratch, xsdSchema.ExpectedNextChild)
	if !valid {
		return validationIssue{code: xsderrors.CodeInternalInvariant, message: msgContentExpectationMetadata}
	}
	if expected.Len() == 0 && !expected.More {
		return unexpectedChildIssue(name)
	}
	return validationIssue{code: xsderrors.CodeValidationElement, message: "unexpected child element " + quotedName(name) + "; expected next child " + contentExpectationMessage(expected)}
}
func (s *session) incompleteContentIssue(parent *frame) validationIssue {
	scratch := s.contentScratch(parent)
	expected, valid := s.rt.ExpectedContent(parent.Content, &scratch, xsdSchema.MissingRequiredContent)
	if !valid {
		return validationIssue{code: xsderrors.CodeInternalInvariant, message: msgContentExpectationMetadata}
	}
	return validationIssue{code: xsderrors.CodeValidationContent, message: "content is incomplete; expected next child " + contentExpectationMessage(expected)}
}

func attributeValidation(ctx StartContext, msg string) error {
	return validation(ctx, xsderrors.CodeValidationAttribute, msg)
}

// terminalValueError preserves operational failures before ordinary value errors
// are rendered with an assessment-specific code and message.
func terminalValueError(ctx StartContext, err error, workLimit uint64) error {
	if errors.Is(err, xsdValue.ErrMetadata) {
		return xsderrors.InternalInvariant("simple value metadata is invalid")
	}
	if errors.Is(err, xsdValue.ErrLimit) {
		return validation(ctx, xsderrors.CodeValidationLimit, msgSimpleValueLimit+": MaxInstanceValueWork="+strconv.FormatUint(workLimit, 10))
	}
	if xsderrors.IsUnsupported(err) {
		return err
	}
	return nil
}

func simpleValueError(ctx StartContext, code xsderrors.Code, message string, err error, workLimit uint64) error {
	if terminalErr := terminalValueError(ctx, err, workLimit); terminalErr != nil {
		return terminalErr
	}
	return validation(ctx, code, message+": "+err.Error())
}

func optionError(msg string) error {
	return xsderrors.Validation(xsderrors.CodeValidationOption, msg, nil)
}

func validationFromIssue(ctx StartContext, issue validationIssue) error {
	if issue.code == xsderrors.CodeInternalInvariant {
		return xsderrors.InternalInvariant(issue.message)
	}
	return validation(ctx, issue.code, issue.message)
}

func abstractElementMessage(rt *xsdSchema.Schema, id xsdSchema.ElementID) string {
	if name, ok := rt.DiagnosticElementName(id); ok {
		return "element " + diagnostictext.ExpandedName(name.Namespace, name.Local) + " is abstract; use a concrete substitution-group member"
	}
	return msgAbstractElement
}
func elementStartDiagnosticIssue(rt *xsdSchema.Schema, start *schemaStart, issue validationIssue) validationIssue {
	if issue.code == xsderrors.CodeValidationType {
		issue.message = "complex type " + typeDiagnosticLabel(rt, start.typ) + " is abstract; use a concrete derived xsi:type"
		return issue
	}
	if name, ok := rt.DiagnosticElementName(start.element); ok {
		issue.message = "element " + diagnostictext.ExpandedName(name.Namespace, name.Local) + ": " + issue.message
	}
	return issue
}
func unavailableElementTypeMessage(rt *xsdSchema.Schema, id xsdSchema.TypeID) string {
	return "element type " + typeDiagnosticLabel(rt, id) + " is unavailable because its schema declaration was not loaded"
}

func depthLimitMessage(cause error) string { return msgInstanceDepthLimit + ": " + cause.Error() }
func instanceLimitError(err error, ctx StartContext, limits Limits) error {
	var option string
	var configured int64
	switch {
	case xmlstream.IsInputLimit(err):
		option = optionMaxInstanceBytes
		configured = limits.InstanceBytes
	case xmlstream.IsTokenLimit(err):
		option = optionMaxInstanceTokenBytes
		configured = limits.InstanceTokenBytes
	case xmlstream.IsAttributeLimit(err):
		option = optionMaxInstanceAttributes
		configured = int64(limits.InstanceAttributes)
	default:
		return nil
	}
	return xsderrors.WithLocation(ctx.PathString(), ctx.Line, ctx.Column, xsderrors.Validation(xsderrors.CodeValidationLimit, option+"="+strconv.FormatInt(configured, 10)+" exceeded", err))
}
