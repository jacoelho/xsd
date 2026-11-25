package schema

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// schemaElementPlacement is where an XSD element appears, as far as the
// unqualified attributes it admits or requires depend on it.
type schemaElementPlacement uint8

const (
	schemaPlacementOther schemaElementPlacement = iota
	// schemaPlacementTopLevel is a child of xs:schema.
	schemaPlacementTopLevel
	// schemaPlacementReference is a reference; see isSchemaReference.
	schemaPlacementReference
	// schemaPlacementLocal is an xs:element or xs:attribute declaration, or
	// an xs:complexType or xs:simpleType definition, that is neither
	// top-level nor a reference. Whether its parent admits it there is the
	// parent's content model; see schemaElementPlacementOf.
	schemaPlacementLocal
	// schemaPlacementNamedGroupModel is the model group of a top-level
	// xs:group. A nested xs:group is a reference, whose model group child
	// admitChildContent rejects before attributes are checked.
	schemaPlacementNamedGroupModel
	// schemaPlacementSimpleContentDerivation is an xs:restriction or
	// xs:extension child of xs:simpleContent.
	schemaPlacementSimpleContentDerivation
	// schemaPlacementComplexContentDerivation is an xs:restriction or
	// xs:extension child of xs:complexContent.
	schemaPlacementComplexContentDerivation
)

// schemaElementPlacementOf returns the placement of n, an XSD element whose
// parent is named parent. It classifies each start tag once; its content
// (childContentFor) and its attributes (admitXSDChildStart) both read the
// result. The placement says nothing about whether parent admits n:
// admitChildContent rejects a misplaced element before its attributes are
// read, so the content model of each parent stays the single owner of where a
// component may appear. xs:redefine is rejected at its own start, so no child
// of it is classified.
func schemaElementPlacementOf(n *schemaSyntaxNode, parent xml.Name) schemaElementPlacement {
	switch {
	case isXSDName(parent, vocab.XSDElemSchema):
		return schemaPlacementTopLevel
	case isSchemaReference(n):
		return schemaPlacementReference
	}
	switch n.Name.Local {
	case vocab.XSDElemElement, vocab.XSDElemAttribute, vocab.XSDElemComplexType, vocab.XSDElemSimpleType:
		return schemaPlacementLocal
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll:
		if isXSDName(parent, vocab.XSDElemGroup) {
			return schemaPlacementNamedGroupModel
		}
	case vocab.XSDElemRestriction, vocab.XSDElemExtension:
		switch {
		case isXSDName(parent, vocab.XSDElemSimpleContent):
			return schemaPlacementSimpleContentDerivation
		case isXSDName(parent, vocab.XSDElemComplexContent):
			return schemaPlacementComplexContentDerivation
		}
	}
	return schemaPlacementOther
}

// isSchemaReference reports whether start, an XSD element that is not a
// child of xs:schema, is a reference rather than a declaration or definition:
// an xs:group or xs:attributeGroup, or an xs:element or xs:attribute carrying
// ref (src-element.2.2, src-attribute.3.2). A reference has (annotation?)
// content and admits only the attributes misplacedReferenceAttribute admits.
func isSchemaReference(start *schemaSyntaxNode) bool {
	switch start.Name.Local {
	case vocab.XSDElemGroup, vocab.XSDElemAttributeGroup:
		return true
	case vocab.XSDElemElement, vocab.XSDElemAttribute:
		_, hasRef := start.attr(vocab.XSDAttrRef)
		return hasRef
	default:
		return false
	}
}

// misplacedSchemaAttributeError returns the error for attr, an XSD 1.0
// attribute of n, when the placement of n forbids it, or nil.
// misplacedSchemaAttributeError and checkRequiredSchemaAttributes are the
// single owner of which unqualified XSD 1.0 attributes each XSD element
// admits and requires in each placement, as the schema for schemas types of
// that placement and src-element.2.1 and src-attribute.3.1 declare them. Both
// run on the start tag, so their violations precede every child in document
// order. Unsupported vocabulary never reaches them; see
// rejectUnsupportedVocabulary. Elements outside the XSD 1.0 vocabulary never
// reach them: their parent's content rejects them first; see
// childContentViolation.
func misplacedSchemaAttributeError(n *schemaSyntaxNode, attr string, placement schemaElementPlacement) error {
	element := n.Name.Local
	var message string
	switch placement {
	case schemaPlacementOther, schemaPlacementSimpleContentDerivation, schemaPlacementComplexContentDerivation:
	case schemaPlacementTopLevel:
		if topLevelAttributeForbidden(element, attr) {
			message = "top-level " + diagnostictext.Quote(element) + " cannot have " + diagnostictext.Quote(attr)
		}
	case schemaPlacementReference:
		message = misplacedReferenceAttribute(element, attr)
	case schemaPlacementLocal:
		message = misplacedLocalAttribute(element, attr)
	case schemaPlacementNamedGroupModel:
		message = misplacedNamedGroupModelAttribute(attr)
	default:
		return withSchemaCompileLocation(n, xsderrors.InternalInvariant("schema element placement is unknown", nil))
	}
	if message == "" {
		return nil
	}
	return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, message)
}

// checkRequiredSchemaAttributes returns the error for n, an XSD element in
// placement, when it lacks an attribute its placement requires
// (placementAttributeRequirement) or one its element requires in every
// placement (elementAttributeRequirements), in that order, or nil.
func checkRequiredSchemaAttributes(n *schemaSyntaxNode, placement schemaElementPlacement) error {
	requirement, required, err := placementAttributeRequirement(n.Name.Local, placement)
	if err != nil {
		return withSchemaCompileLocation(n, err)
	}
	if required && !requirement.satisfiedBy(n) {
		return requirement.missingError(n)
	}
	return checkElementRequiredAttributes(n)
}

// checkValueConstraintAttributes returns the error for n when it is an
// xs:element or xs:attribute whose default excludes another of its attributes:
// a fixed (src-element.1, src-attribute.1), or a use other than optional
// (src-attribute.2). It is the only owner of these rules; use is the mode
// admitParsedAttributeValue admitted from n.
func checkValueConstraintAttributes(n *schemaSyntaxNode, use attributeUseMode) error {
	element := n.Name.Local
	if element != vocab.XSDElemElement && element != vocab.XSDElemAttribute {
		return nil
	}
	if _, ok := n.attr(vocab.XSDAttrDefault); !ok {
		return nil
	}
	if _, ok := n.attr(vocab.XSDAttrFixed); ok {
		message := "attribute cannot have both default and fixed"
		if element == vocab.XSDElemElement {
			message = "element cannot have both default and fixed"
		}
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, message)
	}
	return withSchemaCompileLocation(n, invalidAttributeError(checkAttributeUseDefault(use)))
}

// checkAttributeDeclarationName returns the error for n when it is an
// attribute declaration named xmlns (xmlns Not Allowed) whose use is not
// prohibited. A prohibited declaration corresponds to no component (I-008),
// so its name constrains nothing. The name is compared after whitespace
// collapse, as its NCName value. It is the only owner of the rule; use is the
// mode admitParsedAttributeValue admitted from n.
func checkAttributeDeclarationName(n *schemaSyntaxNode, use attributeUseMode) error {
	if n.Name.Local != vocab.XSDElemAttribute || use == attributeUseProhibited {
		return nil
	}
	name, ok := n.attr(vocab.XSDAttrName)
	if !ok || lex.CollapseXMLWhitespace(name) != vocab.XMLNSPrefix {
		return nil
	}
	return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "attribute cannot be named xmlns")
}

// elementAttributeRequirement is an attribute that an XSD element requires,
// with the diagnostic of its absence; when alternative is set, either
// attribute satisfies it. When facetLabelled is set, the diagnostic is message
// after the facet's label, so one requirement serves every facet and builds
// its text only when the attribute is missing.
type elementAttributeRequirement struct {
	attr          string
	alternative   string
	message       string
	code          xsderrors.Code
	facetLabelled bool
}

var (
	includeAttributeRequirements = []elementAttributeRequirement{
		{attr: vocab.XSDAttrSchemaLocation, message: "include missing schemaLocation", code: xsderrors.CodeSchemaReference},
	}
	notationAttributeRequirements = []elementAttributeRequirement{
		{attr: vocab.XSDAttrName, message: "notation missing name", code: xsderrors.CodeSchemaInvalidAttribute},
		{attr: vocab.XSDAttrPublic, alternative: vocab.XSDAttrSystem, message: "notation requires public or system", code: xsderrors.CodeSchemaInvalidAttribute},
	}
	identityAttributeRequirements = []elementAttributeRequirement{
		{attr: vocab.XSDAttrName, message: "identity constraint missing name", code: xsderrors.CodeSchemaIdentity},
	}
	keyrefAttributeRequirements = []elementAttributeRequirement{
		identityAttributeRequirements[0],
		{attr: vocab.XSDAttrRefer, message: "keyref missing refer", code: xsderrors.CodeSchemaIdentity},
	}
	selectorAttributeRequirements = []elementAttributeRequirement{
		{attr: vocab.XSDAttrXPath, message: `"selector" missing xpath`, code: xsderrors.CodeSchemaIdentity},
	}
	fieldAttributeRequirements = []elementAttributeRequirement{
		{attr: vocab.XSDAttrXPath, message: `"field" missing xpath`, code: xsderrors.CodeSchemaIdentity},
	}
	facetAttributeRequirements = []elementAttributeRequirement{
		{attr: vocab.XSDAttrValue, message: " missing value", code: xsderrors.CodeSchemaFacet, facetLabelled: true},
	}
)

// checkElementRequiredAttributes returns the error for the first attribute,
// in requirement order, that n lacks and its element requires in every
// placement, or nil.
func checkElementRequiredAttributes(n *schemaSyntaxNode) error {
	for _, requirement := range elementAttributeRequirements(n.Name.Local) {
		if !requirement.satisfiedBy(n) {
			return requirement.missingError(n)
		}
	}
	return nil
}

func (r elementAttributeRequirement) missingError(n *schemaSyntaxNode) error {
	if r.facetLabelled {
		return schemaCompileAt(n, r.code, facetLabel(n.Name.Local)+r.message)
	}
	return schemaCompileAt(n, r.code, r.message)
}

func (r elementAttributeRequirement) satisfiedBy(n *schemaSyntaxNode) bool {
	if _, ok := n.attr(r.attr); ok {
		return true
	}
	if r.alternative == "" {
		return false
	}
	_, ok := n.attr(r.alternative)
	return ok
}

func elementAttributeRequirements(element string) []elementAttributeRequirement {
	switch element {
	case vocab.XSDElemInclude:
		return includeAttributeRequirements
	case vocab.XSDElemNotation:
		return notationAttributeRequirements
	case vocab.XSDElemKey, vocab.XSDElemUnique:
		return identityAttributeRequirements
	case vocab.XSDElemKeyref:
		return keyrefAttributeRequirements
	case vocab.XSDElemSelector:
		return selectorAttributeRequirements
	case vocab.XSDElemField:
		return fieldAttributeRequirements
	}
	if isFacetLocal(element) {
		return facetAttributeRequirements
	}
	return nil
}

// placementAttributeRequirement returns the attribute that an XSD element
// named element requires in placement, and whether placement requires one. A
// reference to an element or attribute carries ref by definition of its
// placement, so it requires none; a local declaration has no ref by
// definition of its placement, so it requires name; a content derivation is
// an xs:restriction or xs:extension by definition of its placement, so it
// requires base. A missing attribute that the placement requires is a
// reference violation. Every message is a constant, so no raw schema text
// reaches it.
func placementAttributeRequirement(element string, placement schemaElementPlacement) (elementAttributeRequirement, bool, error) {
	var requirement elementAttributeRequirement
	switch placement {
	case schemaPlacementOther, schemaPlacementNamedGroupModel:
	case schemaPlacementTopLevel:
		if message := missingTopLevelNameMessage(element); message != "" {
			requirement = elementAttributeRequirement{attr: vocab.XSDAttrName, message: message}
		}
	case schemaPlacementLocal:
		switch element {
		case vocab.XSDElemElement:
			requirement = elementAttributeRequirement{attr: vocab.XSDAttrName, message: "local element missing name or ref"}
		case vocab.XSDElemAttribute:
			requirement = elementAttributeRequirement{attr: vocab.XSDAttrName, message: "attribute missing name or ref"}
		}
	case schemaPlacementReference:
		switch element {
		case vocab.XSDElemGroup:
			requirement = elementAttributeRequirement{attr: vocab.XSDAttrRef, message: "group use missing ref"}
		case vocab.XSDElemAttributeGroup:
			requirement = elementAttributeRequirement{attr: vocab.XSDAttrRef, message: "attributeGroup use missing ref"}
		}
	case schemaPlacementSimpleContentDerivation:
		requirement = derivationBaseRequirement(element, "simpleContent restriction missing base", "simpleContent extension missing base")
	case schemaPlacementComplexContentDerivation:
		requirement = derivationBaseRequirement(element, "complexContent restriction missing base", "complexContent extension missing base")
	default:
		return elementAttributeRequirement{}, false, xsderrors.InternalInvariant("schema element placement is unknown", nil)
	}
	if requirement.attr == "" {
		return elementAttributeRequirement{}, false, nil
	}
	requirement.code = xsderrors.CodeSchemaReference
	return requirement, true, nil
}

// derivationBaseRequirement is the base that derivation, the local name of an
// xs:restriction or xs:extension, requires; its diagnostic is restriction for
// an xs:restriction and extension for an xs:extension.
func derivationBaseRequirement(derivation, restriction, extension string) elementAttributeRequirement {
	message := extension
	if derivation == vocab.XSDElemRestriction {
		message = restriction
	}
	return elementAttributeRequirement{attr: vocab.XSDAttrBase, message: message}
}

// missingTopLevelNameMessage describes a top-level component named element
// that lacks name, or returns "" when element names no component that
// requires one. Each message is a constant, so a start tag that has its name
// builds no diagnostic text.
func missingTopLevelNameMessage(element string) string {
	switch element {
	case vocab.XSDElemSimpleType:
		return `top-level "simpleType" missing name`
	case vocab.XSDElemComplexType:
		return `top-level "complexType" missing name`
	case vocab.XSDElemAttributeGroup:
		return `top-level "attributeGroup" missing name`
	case vocab.XSDElemGroup:
		return `top-level "group" missing name`
	case vocab.XSDElemAttribute:
		return `top-level "attribute" missing name`
	case vocab.XSDElemElement:
		return `top-level "element" missing name`
	}
	return ""
}

func topLevelAttributeForbidden(element, attr string) bool {
	switch element {
	case vocab.XSDElemAttributeGroup:
		return attr == vocab.XSDAttrRef
	case vocab.XSDElemGroup:
		return attr == vocab.XSDAttrRef || attr == vocab.XSDAttrMinOccurs || attr == vocab.XSDAttrMaxOccurs
	case vocab.XSDElemAttribute:
		return attr == vocab.XSDAttrRef || attr == vocab.XSDAttrForm || attr == vocab.XSDAttrUse
	case vocab.XSDElemElement:
		return attr == vocab.XSDAttrRef || attr == vocab.XSDAttrForm || attr == vocab.XSDAttrMinOccurs || attr == vocab.XSDAttrMaxOccurs
	default:
		return false
	}
}

// misplacedLocalAttribute returns why a local declaration or definition named
// element cannot have attr, or "" when it can. Each message is a constant, so
// no schema text reaches it.
func misplacedLocalAttribute(element, attr string) string {
	switch (schemaElementAttribute{element: element, attr: attr}) {
	case schemaElementAttribute{vocab.XSDElemElement, vocab.XSDAttrAbstract}:
		return "local element cannot have abstract"
	case schemaElementAttribute{vocab.XSDElemElement, vocab.XSDAttrFinal}:
		return "local element cannot have final"
	case schemaElementAttribute{vocab.XSDElemElement, vocab.XSDAttrSubstitutionGroup}:
		return "local element cannot have substitutionGroup"
	case schemaElementAttribute{vocab.XSDElemComplexType, vocab.XSDAttrName}:
		return "local complexType cannot have name"
	case schemaElementAttribute{vocab.XSDElemComplexType, vocab.XSDAttrAbstract}:
		return "local complexType cannot have abstract"
	case schemaElementAttribute{vocab.XSDElemComplexType, vocab.XSDAttrFinal}:
		return "local complexType cannot have final"
	case schemaElementAttribute{vocab.XSDElemComplexType, vocab.XSDAttrBlock}:
		return "local complexType cannot have block"
	case schemaElementAttribute{vocab.XSDElemSimpleType, vocab.XSDAttrName}:
		return "local simpleType cannot have name"
	case schemaElementAttribute{vocab.XSDElemSimpleType, vocab.XSDAttrFinal}:
		return "local simpleType cannot have final"
	default:
		return ""
	}
}

// schemaElementAttribute is an unqualified attribute named attr of the XSD
// element named element.
type schemaElementAttribute struct {
	element string
	attr    string
}

// misplacedNamedGroupModelAttribute returns why the model group of a
// top-level xs:group cannot have attr, or "" when it can.
func misplacedNamedGroupModelAttribute(attr string) string {
	switch attr {
	case vocab.XSDAttrMinOccurs:
		return "top-level model group cannot have " + vocab.XSDAttrMinOccurs
	case vocab.XSDAttrMaxOccurs:
		return "top-level model group cannot have " + vocab.XSDAttrMaxOccurs
	}
	return ""
}

// misplacedReferenceAttribute returns why a reference named element, one of
// those isSchemaReference admits, cannot have attr, or "" when it can.
func misplacedReferenceAttribute(element, attr string) string {
	switch element {
	case vocab.XSDElemElement:
		switch attr {
		case vocab.XSDAttrID, vocab.XSDAttrRef, vocab.XSDAttrMinOccurs, vocab.XSDAttrMaxOccurs:
			return ""
		}
		return "element ref cannot have attribute " + diagnostictext.Quote(attr)
	case vocab.XSDElemGroup:
		switch attr {
		case vocab.XSDAttrID, vocab.XSDAttrRef, vocab.XSDAttrMinOccurs, vocab.XSDAttrMaxOccurs:
			return ""
		}
		return "group cannot have attribute " + diagnostictext.Quote(attr)
	case vocab.XSDElemAttribute:
		switch attr {
		case vocab.XSDAttrID, vocab.XSDAttrRef, vocab.XSDAttrUse, vocab.XSDAttrDefault, vocab.XSDAttrFixed:
			return ""
		}
		return "attribute ref cannot have attribute " + diagnostictext.Quote(attr)
	case vocab.XSDElemAttributeGroup:
		if attr == vocab.XSDAttrID || attr == vocab.XSDAttrRef {
			return ""
		}
		return "attributeGroup use cannot have attribute " + diagnostictext.Quote(attr)
	}
	return ""
}

func schemaElementAttributeAllowed(element, attr string) bool {
	switch element {
	case vocab.XSDElemSchema, vocab.XSDElemInclude, vocab.XSDElemImport, vocab.XSDElemAppinfo, vocab.XSDElemDocumentation:
		return schemaDocumentAttributeAllowed(element, attr)
	case vocab.XSDElemSimpleType, vocab.XSDElemRestriction, vocab.XSDElemExtension, vocab.XSDElemList, vocab.XSDElemUnion:
		return simpleDerivationAttributeAllowed(element, attr)
	case vocab.XSDElemComplexType, vocab.XSDElemAnnotation, vocab.XSDElemSimpleContent, vocab.XSDElemComplexContent:
		return complexDerivationAttributeAllowed(element, attr)
	case vocab.XSDElemGroup, vocab.XSDElemAll, vocab.XSDElemChoice, vocab.XSDElemSequence:
		return modelGroupAttributeAllowed(element, attr)
	case vocab.XSDElemElement:
		return isElementAttribute(attr)
	case vocab.XSDElemAttribute:
		return isAttributeAttribute(attr)
	case vocab.XSDElemAttributeGroup:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrName || attr == vocab.XSDAttrRef
	case vocab.XSDElemAny:
		return isAnyParticleAttribute(attr)
	case vocab.XSDElemAnyAttribute:
		return isAnyAttributeAttribute(attr)
	case vocab.XSDElemUnique, vocab.XSDElemKey:
		return isIdentityAttribute(attr)
	case vocab.XSDElemKeyref:
		return isKeyrefAttribute(attr)
	case vocab.XSDElemSelector, vocab.XSDElemField:
		return isIdentityXPathAttribute(attr)
	case vocab.XSDElemNotation:
		return isNotationAttribute(attr)
	default:
		if _, ok := facetMaskForLocal(element); ok {
			return facetAttributeAllowed(element, attr)
		}
		return false
	}
}

// checkSchemaAttributeValueSyntax checks the value of attr, an unqualified
// attribute of the XSD element named element, against the lexical form the
// element gives it: a boolean, a form, a derivation set, a wildcard namespace
// list, a processContents mode, or a
// namespace name that cannot be empty. An attribute with none of these forms
// passes. Start-tag admission decides these violations; compilation parses the
// admitted value with the same parsers.
func checkSchemaAttributeValueSyntax(element, attr, value string) error {
	if rule, ok := derivationAttrRuleOf(element, attr); ok {
		_, err := parseDerivationSet(value, rule.Label, rule.Allowed)
		return err
	}
	var err error
	switch attr {
	case vocab.XSDAttrNillable, vocab.XSDAttrAbstract, vocab.XSDAttrMixed:
		_, err = parseBooleanAttr(booleanAttr{Name: attr, Value: value, HasValue: true})
	case vocab.XSDAttrForm:
		err = checkFormSyntax(element, value)
	case vocab.XSDAttrElementFormDefault, vocab.XSDAttrAttributeFormDefault:
		_, err = parseFormDefaultAttr(formAttr{Name: attr, Value: value, HasValue: true})
	case vocab.XSDAttrProcessContents:
		_, err = parseWildcardProcessContents(wildcardAttrs{ProcessContents: value, HasProcessContents: true})
	case vocab.XSDAttrNamespace:
		err = checkNamespaceAttributeSyntax(element, value)
	case vocab.XSDAttrTargetNamespace:
		err = validateSchemaTargetNamespace(lexicalAttribute{Value: value, Present: true})
	}
	return err
}

func checkFormSyntax(element, value string) error {
	var err error
	switch element {
	case vocab.XSDElemElement:
		_, err = parseElementFormAttr(formAttr{Value: value, HasValue: true})
	case vocab.XSDElemAttribute:
		_, err = parseAttributeFormAttr(formAttr{Value: value, HasValue: true})
	}
	return err
}

func checkNamespaceAttributeSyntax(element, value string) error {
	switch element {
	case vocab.XSDElemAny, vocab.XSDElemAnyAttribute:
		return checkWildcardNamespaceSyntax(value)
	case vocab.XSDElemImport:
		return validateSchemaImportNamespace(lexicalAttribute{Value: value, Present: true})
	}
	return nil
}

func schemaAnyURIAttribute(element, attr string) bool {
	switch element {
	case vocab.XSDElemSchema:
		return attr == vocab.XSDAttrTargetNamespace
	case vocab.XSDElemInclude:
		return attr == vocab.XSDAttrSchemaLocation
	case vocab.XSDElemImport:
		return attr == vocab.XSDAttrNamespace || attr == vocab.XSDAttrSchemaLocation
	case vocab.XSDElemAppinfo, vocab.XSDElemDocumentation:
		return attr == vocab.XSDAttrSource
	case vocab.XSDElemNotation:
		return attr == vocab.XSDAttrSystem
	default:
		return false
	}
}

func schemaDocumentAttributeAllowed(element, attr string) bool {
	switch element {
	case vocab.XSDElemSchema:
		switch attr {
		case vocab.XSDAttrID, vocab.XSDAttrTargetNamespace, vocab.XSDAttrVersion, vocab.XSDAttrFinalDefault, vocab.XSDAttrBlockDefault, vocab.XSDAttrAttributeFormDefault, vocab.XSDAttrElementFormDefault:
			return true
		}
	case vocab.XSDElemInclude:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrSchemaLocation
	case vocab.XSDElemImport:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrNamespace || attr == vocab.XSDAttrSchemaLocation
	case vocab.XSDElemAppinfo, vocab.XSDElemDocumentation:
		return attr == vocab.XSDAttrSource
	}
	return false
}

func simpleDerivationAttributeAllowed(element, attr string) bool {
	switch element {
	case vocab.XSDElemSimpleType:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrName || attr == vocab.XSDAttrFinal
	case vocab.XSDElemRestriction, vocab.XSDElemExtension:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrBase
	case vocab.XSDElemList:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrItemType
	case vocab.XSDElemUnion:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrMemberTypes
	}
	return false
}

func complexDerivationAttributeAllowed(element, attr string) bool {
	switch element {
	case vocab.XSDElemComplexType:
		switch attr {
		case vocab.XSDAttrID, vocab.XSDAttrName, vocab.XSDAttrMixed, vocab.XSDAttrAbstract, vocab.XSDAttrBlock, vocab.XSDAttrFinal:
			return true
		}
	case vocab.XSDElemAnnotation, vocab.XSDElemSimpleContent:
		return attr == vocab.XSDAttrID
	case vocab.XSDElemComplexContent:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrMixed
	}
	return false
}

func modelGroupAttributeAllowed(element, attr string) bool {
	switch element {
	case vocab.XSDElemGroup:
		switch attr {
		case vocab.XSDAttrID, vocab.XSDAttrName, vocab.XSDAttrRef, vocab.XSDAttrMinOccurs, vocab.XSDAttrMaxOccurs:
			return true
		}
	case vocab.XSDElemAll, vocab.XSDElemChoice, vocab.XSDElemSequence:
		return attr == vocab.XSDAttrID || attr == vocab.XSDAttrMinOccurs || attr == vocab.XSDAttrMaxOccurs
	}
	return false
}

func facetAttributeAllowed(element, attr string) bool {
	if attr == vocab.XSDAttrID || attr == vocab.XSDAttrValue {
		return true
	}
	if attr != vocab.XSDAttrFixed {
		return false
	}
	return element != vocab.XSDFacetPattern && element != vocab.XSDFacetEnumeration
}

func isElementAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrName, vocab.XSDAttrRef, vocab.XSDAttrType, vocab.XSDAttrSubstitutionGroup,
		vocab.XSDAttrNillable, vocab.XSDAttrDefault, vocab.XSDAttrFixed, vocab.XSDAttrForm,
		vocab.XSDAttrBlock, vocab.XSDAttrFinal, vocab.XSDAttrAbstract, vocab.XSDAttrMinOccurs, vocab.XSDAttrMaxOccurs:
		return true
	default:
		return false
	}
}

func isAttributeAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrName, vocab.XSDAttrRef, vocab.XSDAttrType, vocab.XSDAttrUse, vocab.XSDAttrDefault, vocab.XSDAttrFixed, vocab.XSDAttrForm:
		return true
	default:
		return false
	}
}

func isAnyParticleAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrNamespace, vocab.XSDAttrProcessContents, vocab.XSDAttrMinOccurs, vocab.XSDAttrMaxOccurs:
		return true
	default:
		return false
	}
}

func isAnyAttributeAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrNamespace, vocab.XSDAttrProcessContents:
		return true
	default:
		return false
	}
}

func isIdentityAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrName:
		return true
	default:
		return false
	}
}

func isKeyrefAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrName, vocab.XSDAttrRefer:
		return true
	default:
		return false
	}
}

func isIdentityXPathAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrXPath:
		return true
	default:
		return false
	}
}

func isNotationAttribute(name string) bool {
	switch name {
	case vocab.XSDAttrID, vocab.XSDAttrName, vocab.XSDAttrPublic, vocab.XSDAttrSystem:
		return true
	default:
		return false
	}
}
