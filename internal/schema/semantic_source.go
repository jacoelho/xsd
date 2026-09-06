package schema

import (
	"encoding/xml"
	"iter"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/xmlstream"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// schemaNode is the compact immutable semantic source record consumed by the
// compiler. XML syntax nodes are used only during admission; compiler state
// reaches child components through document-local IDs.
type schemaNode struct {
	semantic schemaSemanticSource
	doc      *schemaDocument
	local    string
	// namespace is retained only for deferred QName-valued literals and
	// identity XPath names; schema QName attributes are resolved at admission.
	namespace xmlstream.Context
	children  []schemaNodeID
	// annotationLang is retained only for schema annotation admission. Other
	// source attributes are normalized into typed capability records and are
	// not retained as a generic attribute map.
	annotationLang LexicalAttribute
	// attributeMask records the admitted unqualified XSD attributes. The
	// parser rejects unknown attributes before publishing the node; retaining
	// this presence projection lets compiler checks inspect only attributes that
	// were actually present.
	attributeMask        uint64
	line                 int
	column               int
	id                   schemaNodeID
	kind                 schemaNodeKind
	hasNonWhitespaceText bool
}

func (n *schemaNode) ID() schemaNodeID {
	if n == nil {
		return 0
	}
	return n.id
}

func (n *schemaNode) schemaLocation() (path string, line int, column int) {
	if n == nil || n.doc == nil {
		return "", 0, 0
	}
	return n.doc.name, n.line, n.column
}

func (n *schemaNode) HasNonWhitespaceText() bool {
	return n != nil && n.hasNonWhitespaceText
}

func (n *schemaNode) attr(local string) (string, bool) {
	if n == nil {
		return "", false
	}
	if value, ok := semanticCommonAttribute(&n.semantic, local); ok {
		return value.Value, value.Present
	}
	return "", false
}

func (n *schemaNode) attrValue(local string) string {
	value, _ := n.attr(local)
	return value
}

func (n *schemaNode) attrNS(namespace, local string) (string, bool) {
	if n == nil || namespace != vocab.XMLNamespaceURI || local != vocab.XMLAttrBase {
		return "", false
	}
	return n.semantic.XMLBase.Value, n.semantic.XMLBase.Present
}

type semanticAttributeLookup struct {
	value   LexicalAttribute
	matched bool
}

func semanticCommonAttribute(source *schemaSemanticSource, local string) (LexicalAttribute, bool) {
	if local == vocab.XSDAttrID {
		return source.ID, true
	}
	var lookup semanticAttributeLookup
	switch local {
	case vocab.XSDAttrTargetNamespace, vocab.XSDAttrVersion, vocab.XSDAttrFinalDefault,
		vocab.XSDAttrBlockDefault, vocab.XSDAttrElementFormDefault, vocab.XSDAttrAttributeFormDefault:
		lookup = semanticDocumentAttribute(source.Document, local)
	case vocab.XSDAttrNamespace, vocab.XSDAttrSchemaLocation:
		lookup = semanticReferenceOrWildcardAttribute(source, local)
	case vocab.XSDAttrName:
		lookup = semanticNameAttribute(source, local)
	case vocab.XSDAttrFinal:
		lookup = semanticFinalAttribute(source, local)
	case vocab.XSDAttrBase, vocab.XSDAttrItemType, vocab.XSDAttrMemberTypes:
		lookup = semanticSimpleOrDerivationAttribute(source, local)
	case vocab.XSDAttrDefault, vocab.XSDAttrForm, vocab.XSDAttrType:
		lookup = semanticElementOrAttributeAttribute(source, local)
	case vocab.XSDAttrFixed:
		lookup = semanticElementOrAttributeOrFacetAttribute(source, local)
	case vocab.XSDAttrNillable, vocab.XSDAttrSubstitutionGroup:
		lookup = semanticElementAttribute(source.Element, local)
	case vocab.XSDAttrAbstract, vocab.XSDAttrBlock:
		lookup = semanticElementOrComplexTypeAttribute(source, local)
	case vocab.XSDAttrMinOccurs, vocab.XSDAttrMaxOccurs:
		lookup = semanticElementOrParticleAttribute(source, local)
	case vocab.XSDAttrRef:
		lookup = semanticRefAttribute(source, local)
	case vocab.XSDAttrUse:
		lookup = semanticAttributeSourceAttribute(source.Attribute, local)
	case vocab.XSDAttrMixed:
		lookup = semanticComplexOrDerivationAttribute(source, local)
	case vocab.XSDAttrRefer:
		lookup = semanticIdentityAttribute(source.Identity, local)
	case vocab.XSDAttrXPath:
		lookup = semanticIdentityXPathAttribute(source.IdentityXPath, local)
	case vocab.XSDAttrValue:
		lookup = semanticFacetAttribute(source.Facet, local)
	case vocab.XSDAttrNotNamespace, vocab.XSDAttrNotQName, vocab.XSDAttrProcessContents:
		lookup = semanticWildcardAttribute(source.Wildcard, local)
	case vocab.XSDAttrPublic, vocab.XSDAttrSystem:
		lookup = semanticNotationAttribute(source.Notation, local)
	}
	return lookup.value, lookup.matched
}

func semanticReferenceOrWildcardAttribute(source *schemaSemanticSource, local string) semanticAttributeLookup {
	if source.Reference != nil {
		return semanticReferenceAttribute(source.Reference, local)
	}
	return semanticWildcardAttribute(source.Wildcard, local)
}

func semanticFinalAttribute(source *schemaSemanticSource, local string) semanticAttributeLookup {
	switch {
	case source.SimpleType != nil:
		return semanticSimpleTypeAttribute(source.SimpleType, local)
	case source.Element != nil:
		return semanticElementAttribute(source.Element, local)
	case source.ComplexType != nil:
		return semanticComplexTypeAttribute(source.ComplexType, local)
	default:
		return semanticAttributeLookup{}
	}
}

func semanticSimpleOrDerivationAttribute(source *schemaSemanticSource, local string) semanticAttributeLookup {
	if source.SimpleType != nil {
		return semanticSimpleTypeAttribute(source.SimpleType, local)
	}
	return semanticDerivationAttribute(source.Derivation, local)
}

func semanticElementOrAttributeAttribute(source *schemaSemanticSource, local string) semanticAttributeLookup {
	if source.Element != nil {
		return semanticElementAttribute(source.Element, local)
	}
	return semanticAttributeSourceAttribute(source.Attribute, local)
}

func semanticElementOrAttributeOrFacetAttribute(source *schemaSemanticSource, local string) semanticAttributeLookup {
	if source.Element != nil {
		return semanticElementAttribute(source.Element, local)
	}
	if source.Attribute != nil {
		return semanticAttributeSourceAttribute(source.Attribute, local)
	}
	return semanticFacetAttribute(source.Facet, local)
}

func semanticElementOrComplexTypeAttribute(source *schemaSemanticSource, local string) semanticAttributeLookup {
	if source.Element != nil {
		return semanticElementAttribute(source.Element, local)
	}
	return semanticComplexTypeAttribute(source.ComplexType, local)
}

func semanticElementOrParticleAttribute(source *schemaSemanticSource, local string) semanticAttributeLookup {
	if source.Element != nil {
		return semanticElementAttribute(source.Element, local)
	}
	return semanticParticleAttribute(source.Particle, local)
}

func semanticRefAttribute(source *schemaSemanticSource, local string) semanticAttributeLookup {
	switch {
	case source.Element != nil:
		return semanticElementAttribute(source.Element, local)
	case source.Attribute != nil:
		return semanticAttributeSourceAttribute(source.Attribute, local)
	case source.Particle != nil:
		return semanticParticleAttribute(source.Particle, local)
	case source.Group != nil:
		return semanticGroupAttribute(source.Group, local)
	case source.AttributeGroup != nil:
		return semanticAttributeGroupAttribute(source.AttributeGroup, local)
	default:
		return semanticAttributeLookup{}
	}
}

func semanticComplexOrDerivationAttribute(source *schemaSemanticSource, local string) semanticAttributeLookup {
	if source.ComplexType != nil {
		return semanticComplexTypeAttribute(source.ComplexType, local)
	}
	return semanticDerivationAttribute(source.Derivation, local)
}

func semanticDocumentAttribute(source *schemaDocumentSource, local string) semanticAttributeLookup {
	if source == nil {
		return semanticAttributeLookup{}
	}
	switch local {
	case vocab.XSDAttrTargetNamespace:
		return semanticAttributeLookup{source.TargetNamespace, true}
	case vocab.XSDAttrVersion:
		return semanticAttributeLookup{source.Version, true}
	case vocab.XSDAttrFinalDefault:
		return semanticAttributeLookup{source.FinalDefault, true}
	case vocab.XSDAttrBlockDefault:
		return semanticAttributeLookup{source.BlockDefault, true}
	case vocab.XSDAttrElementFormDefault:
		return semanticAttributeLookup{source.ElementFormDefault, true}
	case vocab.XSDAttrAttributeFormDefault:
		return semanticAttributeLookup{source.AttributeFormDefault, true}
	default:
		return semanticAttributeLookup{}
	}
}

func semanticReferenceAttribute(source *schemaReferenceSource, local string) semanticAttributeLookup {
	if source == nil {
		return semanticAttributeLookup{}
	}
	switch local {
	case vocab.XSDAttrNamespace:
		return semanticAttributeLookup{source.Namespace, true}
	case vocab.XSDAttrSchemaLocation:
		return semanticAttributeLookup{source.SchemaLocation, true}
	default:
		return semanticAttributeLookup{}
	}
}

func semanticNameAttribute(source *schemaSemanticSource, local string) semanticAttributeLookup {
	if local != vocab.XSDAttrName {
		return semanticAttributeLookup{}
	}
	switch {
	case source.SimpleType != nil:
		return semanticAttributeLookup{source.SimpleType.Global.Name, true}
	case source.Element != nil:
		return semanticAttributeLookup{source.Element.Global.Name, true}
	case source.Attribute != nil:
		return semanticAttributeLookup{source.Attribute.Global.Name, true}
	case source.ComplexType != nil:
		return semanticAttributeLookup{source.ComplexType.Global.Name, true}
	case source.Group != nil:
		return semanticAttributeLookup{source.Group.Global.Name, true}
	case source.AttributeGroup != nil:
		return semanticAttributeLookup{source.AttributeGroup.Global.Name, true}
	case source.Notation != nil:
		return semanticAttributeLookup{source.Notation.Global.Name, true}
	case source.Identity != nil:
		return semanticAttributeLookup{source.Identity.Name, true}
	default:
		return semanticAttributeLookup{}
	}
}

func semanticSimpleTypeAttribute(source *schemaSimpleTypeSource, local string) semanticAttributeLookup {
	if source == nil {
		return semanticAttributeLookup{}
	}
	switch local {
	case vocab.XSDAttrFinal:
		return semanticAttributeLookup{source.Final, true}
	case vocab.XSDAttrBase:
		return semanticAttributeLookup{source.Base.Lexical, true}
	case vocab.XSDAttrItemType:
		return semanticAttributeLookup{source.ItemType.Lexical, true}
	case vocab.XSDAttrMemberTypes:
		return semanticAttributeLookup{source.MemberTypes.Lexical, true}
	default:
		return semanticAttributeLookup{}
	}
}

func semanticElementAttribute(source *schemaElementSource, local string) semanticAttributeLookup {
	if source == nil {
		return semanticAttributeLookup{}
	}
	switch local {
	case vocab.XSDAttrDefault:
		return semanticAttributeLookup{source.Default, true}
	case vocab.XSDAttrFixed:
		return semanticAttributeLookup{source.Fixed, true}
	case vocab.XSDAttrForm:
		return semanticAttributeLookup{source.Form, true}
	case vocab.XSDAttrNillable:
		return semanticAttributeLookup{source.Nillable, true}
	case vocab.XSDAttrAbstract:
		return semanticAttributeLookup{source.Abstract, true}
	case vocab.XSDAttrBlock:
		return semanticAttributeLookup{source.Block, true}
	case vocab.XSDAttrFinal:
		return semanticAttributeLookup{source.Final, true}
	case vocab.XSDAttrMinOccurs:
		return semanticAttributeLookup{source.MinOccurs, true}
	case vocab.XSDAttrMaxOccurs:
		return semanticAttributeLookup{source.MaxOccurs, true}
	case vocab.XSDAttrRef:
		return semanticAttributeLookup{source.Ref.Lexical, true}
	case vocab.XSDAttrType:
		return semanticAttributeLookup{source.Type.Lexical, true}
	case vocab.XSDAttrSubstitutionGroup:
		return semanticAttributeLookup{source.SubstitutionGroup.Lexical, true}
	default:
		return semanticAttributeLookup{}
	}
}

func semanticAttributeSourceAttribute(source *schemaAttributeSource, local string) semanticAttributeLookup {
	if source == nil {
		return semanticAttributeLookup{}
	}
	switch local {
	case vocab.XSDAttrDefault:
		return semanticAttributeLookup{source.Default, true}
	case vocab.XSDAttrFixed:
		return semanticAttributeLookup{source.Fixed, true}
	case vocab.XSDAttrForm:
		return semanticAttributeLookup{source.Form, true}
	case vocab.XSDAttrUse:
		return semanticAttributeLookup{source.Use, true}
	case vocab.XSDAttrRef:
		return semanticAttributeLookup{source.Ref.Lexical, true}
	case vocab.XSDAttrType:
		return semanticAttributeLookup{source.Type.Lexical, true}
	default:
		return semanticAttributeLookup{}
	}
}

func semanticComplexTypeAttribute(source *schemaComplexTypeSource, local string) semanticAttributeLookup {
	if source == nil {
		return semanticAttributeLookup{}
	}
	switch local {
	case vocab.XSDAttrMixed:
		return semanticAttributeLookup{source.Mixed, true}
	case vocab.XSDAttrAbstract:
		return semanticAttributeLookup{source.Abstract, true}
	case vocab.XSDAttrBlock:
		return semanticAttributeLookup{source.Block, true}
	case vocab.XSDAttrFinal:
		return semanticAttributeLookup{source.Final, true}
	default:
		return semanticAttributeLookup{}
	}
}

func semanticDerivationAttribute(source *schemaDerivationSource, local string) semanticAttributeLookup {
	if source == nil {
		return semanticAttributeLookup{}
	}
	switch local {
	case vocab.XSDAttrBase:
		return semanticAttributeLookup{source.Base.Lexical, true}
	case vocab.XSDAttrItemType:
		return semanticAttributeLookup{source.ItemType.Lexical, true}
	case vocab.XSDAttrMemberTypes:
		return semanticAttributeLookup{source.MemberTypes.Lexical, true}
	case vocab.XSDAttrMixed:
		return semanticAttributeLookup{source.Mixed, true}
	default:
		return semanticAttributeLookup{}
	}
}

func semanticParticleAttribute(source *schemaParticleSource, local string) semanticAttributeLookup {
	if source == nil {
		return semanticAttributeLookup{}
	}
	switch local {
	case vocab.XSDAttrMinOccurs:
		return semanticAttributeLookup{source.MinOccurs, true}
	case vocab.XSDAttrMaxOccurs:
		return semanticAttributeLookup{source.MaxOccurs, true}
	case vocab.XSDAttrRef:
		return semanticAttributeLookup{source.Ref.Lexical, true}
	default:
		return semanticAttributeLookup{}
	}
}

func semanticIdentityAttribute(source *schemaIdentitySource, local string) semanticAttributeLookup {
	if source == nil {
		return semanticAttributeLookup{}
	}
	switch local {
	case vocab.XSDAttrName:
		return semanticAttributeLookup{source.Name, true}
	case vocab.XSDAttrRefer:
		return semanticAttributeLookup{source.Refer.Lexical, true}
	default:
		return semanticAttributeLookup{}
	}
}

func semanticIdentityXPathAttribute(source *schemaIdentityXPathSource, local string) semanticAttributeLookup {
	if source != nil && local == vocab.XSDAttrXPath {
		return semanticAttributeLookup{source.XPath, true}
	}
	return semanticAttributeLookup{}
}

func semanticFacetAttribute(source *schemaFacetSource, local string) semanticAttributeLookup {
	if source == nil {
		return semanticAttributeLookup{}
	}
	switch local {
	case vocab.XSDAttrValue:
		return semanticAttributeLookup{source.Value, true}
	case vocab.XSDAttrFixed:
		return semanticAttributeLookup{source.Fixed, true}
	default:
		return semanticAttributeLookup{}
	}
}

func semanticWildcardAttribute(source *schemaWildcardSource, local string) semanticAttributeLookup {
	if source == nil {
		return semanticAttributeLookup{}
	}
	switch local {
	case vocab.XSDAttrNamespace:
		return semanticAttributeLookup{source.Namespace, true}
	case vocab.XSDAttrNotNamespace:
		return semanticAttributeLookup{source.NotNamespace, true}
	case vocab.XSDAttrNotQName:
		return semanticAttributeLookup{source.NotQName, true}
	case vocab.XSDAttrProcessContents:
		return semanticAttributeLookup{source.ProcessContents, true}
	default:
		return semanticAttributeLookup{}
	}
}

func semanticGroupAttribute(source *schemaGroupSource, local string) semanticAttributeLookup {
	if source != nil && local == vocab.XSDAttrRef {
		return semanticAttributeLookup{source.Ref.Lexical, true}
	}
	return semanticAttributeLookup{}
}

func semanticAttributeGroupAttribute(source *schemaAttributeGroupSource, local string) semanticAttributeLookup {
	if source != nil && local == vocab.XSDAttrRef {
		return semanticAttributeLookup{source.Ref.Lexical, true}
	}
	return semanticAttributeLookup{}
}

func semanticNotationAttribute(source *schemaNotationSource, local string) semanticAttributeLookup {
	if source == nil {
		return semanticAttributeLookup{}
	}
	switch local {
	case vocab.XSDAttrPublic:
		return semanticAttributeLookup{source.Public, true}
	case vocab.XSDAttrSystem:
		return semanticAttributeLookup{source.System, true}
	default:
		return semanticAttributeLookup{}
	}
}

func (n *schemaNode) xsdChildren() iter.Seq[*schemaNode] {
	return func(yield func(*schemaNode) bool) {
		if n == nil {
			return
		}
		n.yieldXSDChildren(yield)
	}
}

func (n *schemaNode) yieldXSDChildren(yield func(*schemaNode) bool) {
	if n.doc == nil {
		return
	}
	for _, id := range n.children {
		child := n.doc.node(id)
		if child == nil || child.kind == schemaKindForeign {
			continue
		}
		if !yield(child) {
			return
		}
	}
}

func (n *schemaNode) firstXS(local string) *schemaNode {
	for child := range n.xsdChildren() {
		if child.local == local {
			return child
		}
	}
	return nil
}

func (n *schemaNode) resolveQName(lexical string) (xml.Name, error) {
	if name, ok := n.resolvedQName(lexical); ok {
		return name, nil
	}
	parts, err := checkSchemaQNameParts(n, lexical)
	if err != nil {
		return xml.Name{}, err
	}
	if !parts.Prefixed {
		ns, _ := n.namespace.Lookup("")
		return xml.Name{Space: ns, Local: parts.Local}, nil
	}
	ns, ok := n.namespace.Lookup(parts.Prefix)
	if !ok {
		return xml.Name{}, schemaCompileAt(n, xsderrors.CodeSchemaReference, "unbound QName prefix "+parts.Prefix)
	}
	return xml.Name{Space: ns, Local: parts.Local}, nil
}

// schemaQNameAttribute is the parser-owned representation of a QName-valued
// schema attribute. The expanded name is resolved while the namespace frame
// for the source element is live; later compiler stages only validate schema
// reference rules and intern the expanded name.
type schemaQNameAttribute struct {
	Name     xml.Name
	Lexical  LexicalAttribute
	Resolved bool
}

type schemaQNameListAttribute struct {
	Lexical LexicalAttribute
	Names   []xml.Name
}

type semanticQNameProjection struct {
	memberTypes       schemaQNameListAttribute
	base              schemaQNameAttribute
	itemType          schemaQNameAttribute
	ref               schemaQNameAttribute
	refer             schemaQNameAttribute
	substitutionGroup schemaQNameAttribute
	typeName          schemaQNameAttribute
}

// schemaGlobalSource contains the common source contract for named global
// declarations. It is deliberately separate from schemaNode so capability
// compilers consume typed fields rather than interpreting an XML tree.
type schemaGlobalSource struct {
	ID   LexicalAttribute
	Name LexicalAttribute
}

type schemaSimpleTypeSource struct {
	Global      schemaGlobalSource
	MemberTypes schemaQNameListAttribute
	Final       LexicalAttribute
	Base        schemaQNameAttribute
	ItemType    schemaQNameAttribute
}

type schemaFacetSource struct {
	Value LexicalAttribute
	Fixed LexicalAttribute
}

type schemaElementSource struct {
	Global            schemaGlobalSource
	Default           LexicalAttribute
	Fixed             LexicalAttribute
	Form              LexicalAttribute
	Nillable          LexicalAttribute
	Abstract          LexicalAttribute
	Block             LexicalAttribute
	Final             LexicalAttribute
	MinOccurs         LexicalAttribute
	MaxOccurs         LexicalAttribute
	Ref               schemaQNameAttribute
	Type              schemaQNameAttribute
	SubstitutionGroup schemaQNameAttribute
}

type schemaAttributeSource struct {
	Global  schemaGlobalSource
	Default LexicalAttribute
	Fixed   LexicalAttribute
	Form    LexicalAttribute
	Use     LexicalAttribute
	Ref     schemaQNameAttribute
	Type    schemaQNameAttribute
}

type schemaComplexTypeSource struct {
	Global   schemaGlobalSource
	Mixed    LexicalAttribute
	Abstract LexicalAttribute
	Block    LexicalAttribute
	Final    LexicalAttribute
}

type schemaDerivationSource struct {
	MemberTypes schemaQNameListAttribute
	ID          LexicalAttribute
	// Mixed belongs to xs:complexContent. Keeping it with the derivation
	// container preserves that attribute without a generic source map.
	Mixed    LexicalAttribute
	Base     schemaQNameAttribute
	ItemType schemaQNameAttribute
}

// schemaParticleSource is shared by element/group/any and model-group
// particles. It stores only particle vocabulary; declaration-specific fields
// stay in their owning records above.
type schemaParticleSource struct {
	MinOccurs LexicalAttribute
	MaxOccurs LexicalAttribute
	Ref       schemaQNameAttribute
}

// schemaModelSource is the typed source record for sequence, choice, and all
// model groups. Child IDs preserve document order without making the model
// compiler inspect a generic XML child tree.
type schemaModelSource struct {
	ChildIDs []schemaNodeID
	Kind     ModelKind
}

type schemaIdentitySource struct {
	Selector LexicalAttribute
	Fields   []schemaNodeID
	Name     LexicalAttribute
	Refer    schemaQNameAttribute
}

type schemaIdentityXPathSource struct {
	XPath LexicalAttribute
}

type schemaWildcardSource struct {
	Namespace       LexicalAttribute
	NotNamespace    LexicalAttribute
	NotQName        LexicalAttribute
	ProcessContents LexicalAttribute
}

type schemaGroupSource struct {
	Global schemaGlobalSource
	Ref    schemaQNameAttribute
}

type schemaAttributeGroupSource struct {
	Global schemaGlobalSource
	Ref    schemaQNameAttribute
}

type schemaDocumentSource struct {
	ID                   LexicalAttribute
	TargetNamespace      LexicalAttribute
	Version              LexicalAttribute
	FinalDefault         LexicalAttribute
	BlockDefault         LexicalAttribute
	ElementFormDefault   LexicalAttribute
	AttributeFormDefault LexicalAttribute
}

type schemaReferenceSource struct {
	ID             LexicalAttribute
	Namespace      LexicalAttribute
	SchemaLocation LexicalAttribute
}

type schemaNotationSource struct {
	Global schemaGlobalSource
	Public LexicalAttribute
	System LexicalAttribute
}

// schemaSemanticSource is the typed source view for one schema node. At most
// one capability record is populated for a node, while common particle and
// derivation records may accompany a declaration record.
type schemaSemanticSource struct {
	ComplexType    *schemaComplexTypeSource
	Particle       *schemaParticleSource
	Reference      *schemaReferenceSource
	Notation       *schemaNotationSource
	SimpleType     *schemaSimpleTypeSource
	Facet          *schemaFacetSource
	Element        *schemaElementSource
	Attribute      *schemaAttributeSource
	Document       *schemaDocumentSource
	AttributeGroup *schemaAttributeGroupSource
	Derivation     *schemaDerivationSource
	Model          *schemaModelSource
	Identity       *schemaIdentitySource
	IdentityXPath  *schemaIdentityXPathSource
	Wildcard       *schemaWildcardSource
	Group          *schemaGroupSource
	ID             LexicalAttribute
	XMLBase        LexicalAttribute
}

// typedAttributeNames is the admitted XSD attribute vocabulary. The parser
// rejects unknown attributes before semantic nodes are published; retaining
// this list here lets compiler admission checks inspect presence without a
// generic attribute map.
var typedAttributeNames = [...]string{
	vocab.XSDAttrID,
	vocab.XSDAttrName,
	vocab.XSDAttrRef,
	vocab.XSDAttrType,
	vocab.XSDAttrBase,
	vocab.XSDAttrItemType,
	vocab.XSDAttrMemberTypes,
	vocab.XSDAttrRefer,
	vocab.XSDAttrSubstitutionGroup,
	vocab.XSDAttrDefault,
	vocab.XSDAttrFixed,
	vocab.XSDAttrForm,
	vocab.XSDAttrNillable,
	vocab.XSDAttrAbstract,
	vocab.XSDAttrBlock,
	vocab.XSDAttrFinal,
	vocab.XSDAttrMixed,
	vocab.XSDAttrUse,
	vocab.XSDAttrMinOccurs,
	vocab.XSDAttrMaxOccurs,
	vocab.XSDAttrNamespace,
	vocab.XSDAttrNotNamespace,
	vocab.XSDAttrNotQName,
	vocab.XSDAttrProcessContents,
	vocab.XSDAttrXPath,
	vocab.XSDAttrValue,
	vocab.XSDAttrTargetNamespace,
	vocab.XSDAttrVersion,
	vocab.XSDAttrFinalDefault,
	vocab.XSDAttrBlockDefault,
	vocab.XSDAttrElementFormDefault,
	vocab.XSDAttrAttributeFormDefault,
	vocab.XSDAttrSchemaLocation,
	vocab.XSDAttrPublic,
	vocab.XSDAttrSystem,
}

func schemaModelChildren(n *schemaNode) []*schemaNode {
	if n == nil || n.semantic.Model == nil || n.doc == nil {
		return nil
	}
	children := make([]*schemaNode, 0, len(n.semantic.Model.ChildIDs))
	for _, id := range n.semantic.Model.ChildIDs {
		if child := n.doc.node(id); child != nil {
			children = append(children, child)
		}
	}
	return children
}

func schemaModelKind(n *schemaNode) (ModelKind, bool) {
	if n == nil || n.semantic.Model == nil {
		return 0, false
	}
	return n.semantic.Model.Kind, true
}

func schemaSimpleTypeChildren(n *schemaNode) []*schemaNode {
	if n == nil || len(n.children) == 0 {
		return nil
	}
	children := make([]*schemaNode, 0, len(n.children))
	for child := range n.xsdChildren() {
		if child.kind == schemaKindSimpleType {
			children = append(children, child)
		}
	}
	return children
}

func schemaElementRef(n *schemaNode) (string, bool) {
	if source := n.semantic.Element; source != nil {
		return source.Ref.Lexical.Value, source.Ref.Lexical.Present
	}
	return "", false
}

func schemaElementName(n *schemaNode) string {
	if source := n.semantic.Element; source != nil {
		return source.Global.Name.Value
	}
	return ""
}

func schemaElementForm(n *schemaNode) (string, bool) {
	if source := n.semantic.Element; source != nil {
		return source.Form.Value, source.Form.Present
	}
	return "", false
}

func schemaElementType(n *schemaNode) (string, bool) {
	if source := n.semantic.Element; source != nil {
		return source.Type.Lexical.Value, source.Type.Lexical.Present
	}
	return "", false
}

func schemaGroupRef(n *schemaNode) (string, bool) {
	if source := n.semantic.Group; source != nil {
		return source.Ref.Lexical.Value, source.Ref.Lexical.Present
	}
	return "", false
}

func schemaListItemType(n *schemaNode) (string, bool) {
	if source := n.semantic.Derivation; source != nil {
		return source.ItemType.Lexical.Value, source.ItemType.Lexical.Present
	}
	return "", false
}

func (n *schemaNode) resolvedQName(lexical string) (xml.Name, bool) {
	if n == nil {
		return xml.Name{}, false
	}
	if name, ok := resolvedQNameSimpleType(n.semantic.SimpleType, lexical); ok {
		return name, true
	}
	if name, ok := resolvedQNameElement(n.semantic.Element, lexical); ok {
		return name, true
	}
	if name, ok := resolvedQNameAttribute(n.semantic.Attribute, lexical); ok {
		return name, true
	}
	if name, ok := resolvedQNameDerivation(n.semantic.Derivation, lexical); ok {
		return name, true
	}
	if name, ok := resolvedQNameParticle(n.semantic.Particle, lexical); ok {
		return name, true
	}
	if name, ok := resolvedQNameIdentity(n.semantic.Identity, lexical); ok {
		return name, true
	}
	if name, ok := resolvedQNameGroup(n.semantic.Group, lexical); ok {
		return name, true
	}
	if name, ok := resolvedQNameAttributeGroup(n.semantic.AttributeGroup, lexical); ok {
		return name, true
	}
	return xml.Name{}, false
}

func resolvedQNameValue(value schemaQNameAttribute, lexical string) (xml.Name, bool) {
	if value.Resolved && value.Lexical.Present && value.Lexical.Value == lexical {
		return value.Name, true
	}
	return xml.Name{}, false
}

func resolvedQNameSimpleType(source *schemaSimpleTypeSource, lexical string) (xml.Name, bool) {
	if source == nil {
		return xml.Name{}, false
	}
	if name, ok := resolvedQNameValue(source.Base, lexical); ok {
		return name, true
	}
	if name, ok := resolvedQNameValue(source.ItemType, lexical); ok {
		return name, true
	}
	return resolvedQNameList(source.MemberTypes, lexical)
}

func resolvedQNameElement(source *schemaElementSource, lexical string) (xml.Name, bool) {
	if source == nil {
		return xml.Name{}, false
	}
	if name, ok := resolvedQNameValue(source.Ref, lexical); ok {
		return name, true
	}
	if name, ok := resolvedQNameValue(source.Type, lexical); ok {
		return name, true
	}
	return resolvedQNameValue(source.SubstitutionGroup, lexical)
}

func resolvedQNameAttribute(source *schemaAttributeSource, lexical string) (xml.Name, bool) {
	if source == nil {
		return xml.Name{}, false
	}
	if name, ok := resolvedQNameValue(source.Ref, lexical); ok {
		return name, true
	}
	return resolvedQNameValue(source.Type, lexical)
}

func resolvedQNameDerivation(source *schemaDerivationSource, lexical string) (xml.Name, bool) {
	if source == nil {
		return xml.Name{}, false
	}
	if name, ok := resolvedQNameValue(source.Base, lexical); ok {
		return name, true
	}
	if name, ok := resolvedQNameValue(source.ItemType, lexical); ok {
		return name, true
	}
	return resolvedQNameList(source.MemberTypes, lexical)
}

func resolvedQNameParticle(source *schemaParticleSource, lexical string) (xml.Name, bool) {
	if source == nil {
		return xml.Name{}, false
	}
	return resolvedQNameValue(source.Ref, lexical)
}

func resolvedQNameIdentity(source *schemaIdentitySource, lexical string) (xml.Name, bool) {
	if source == nil {
		return xml.Name{}, false
	}
	return resolvedQNameValue(source.Refer, lexical)
}

func resolvedQNameGroup(source *schemaGroupSource, lexical string) (xml.Name, bool) {
	if source == nil {
		return xml.Name{}, false
	}
	return resolvedQNameValue(source.Ref, lexical)
}

func resolvedQNameAttributeGroup(source *schemaAttributeGroupSource, lexical string) (xml.Name, bool) {
	if source == nil {
		return xml.Name{}, false
	}
	return resolvedQNameValue(source.Ref, lexical)
}

func resolvedQNameList(value schemaQNameListAttribute, lexical string) (xml.Name, bool) {
	if !value.Lexical.Present {
		return xml.Name{}, false
	}
	i := 0
	for part := range lex.XMLFieldsSeq(value.Lexical.Value) {
		if part == lexical && i < len(value.Names) {
			return value.Names[i], true
		}
		i++
	}
	return xml.Name{}, false
}

func schemaGlobalSourceFor(n *schemaSyntaxNode) schemaGlobalSource {
	return schemaGlobalSource{
		ID:   schemaLexicalAttributeSyntax(n, vocab.XSDAttrID),
		Name: schemaLexicalAttributeSyntax(n, vocab.XSDAttrName),
	}
}

func schemaQNameSourceFor(n *schemaSyntaxNode, local string) (schemaQNameAttribute, error) {
	lexical := schemaLexicalAttributeSyntax(n, local)
	if !lexical.Present {
		return schemaQNameAttribute{Lexical: lexical}, nil
	}
	name, err := n.resolveQName(lexical.Value)
	if err != nil {
		return schemaQNameAttribute{}, err
	}
	return schemaQNameAttribute{Lexical: lexical, Name: name, Resolved: true}, nil
}

func schemaQNameListSourceFor(n *schemaSyntaxNode, local string) (schemaQNameListAttribute, error) {
	lexical := schemaLexicalAttributeSyntax(n, local)
	if !lexical.Present {
		return schemaQNameListAttribute{Lexical: lexical}, nil
	}
	names := make([]xml.Name, 0, xmlFieldCount(lexical.Value))
	for start := 0; start < len(lexical.Value); {
		part, next, ok := nextXMLField(lexical.Value, start)
		if !ok {
			break
		}
		name, err := n.resolveQName(part)
		if err != nil {
			return schemaQNameListAttribute{}, err
		}
		names = append(names, name)
		start = next
	}
	return schemaQNameListAttribute{Lexical: lexical, Names: names}, nil
}

func nextXMLField(s string, start int) (field string, next int, ok bool) {
	for start < len(s) && lex.IsXMLWhitespaceByte(s[start]) {
		start++
	}
	if start == len(s) {
		return "", start, false
	}
	next = start + 1
	for next < len(s) && !lex.IsXMLWhitespaceByte(s[next]) {
		next++
	}
	return s[start:next], next, true
}

func xmlFieldCount(s string) int {
	count := 0
	inField := false
	for i := range s {
		if lex.IsXMLWhitespaceByte(s[i]) {
			inField = false
			continue
		}
		if !inField {
			count++
			inField = true
		}
	}
	return count
}

func (syntax *schemaSyntaxNode) buildSemanticSource(n *schemaNode) error {
	var s schemaSemanticSource
	for _, attr := range syntax.Attrs {
		if attr.Name.Space == vocab.XMLNamespaceURI && attr.Name.Local == vocab.XMLAttrBase {
			s.XMLBase = LexicalAttribute{Value: attr.Value, Present: true}
			continue
		}
	}
	s.ID = schemaLexicalAttributeSyntax(syntax, vocab.XSDAttrID)
	projection, err := resolveSemanticQNames(syntax)
	if err != nil {
		return err
	}
	buildSemanticKind(syntax, &s, &projection)
	n.semantic = s
	return nil
}

func resolveSemanticQNames(n *schemaSyntaxNode) (semanticQNameProjection, error) {
	projection, err := resolveSemanticQNameAttributes(n)
	if err != nil {
		return semanticQNameProjection{}, err
	}
	if !schemaNodeHasQNameAttr(n, vocab.XSDAttrMemberTypes) {
		return projection, nil
	}
	value, err := schemaQNameListSourceFor(n, vocab.XSDAttrMemberTypes)
	if err != nil {
		return semanticQNameProjection{}, err
	}
	if value.Lexical.Present {
		projection.memberTypes = value
	}
	return projection, nil
}

var semanticQNameAttributeNames = [...]string{
	vocab.XSDAttrBase,
	vocab.XSDAttrItemType,
	vocab.XSDAttrRef,
	vocab.XSDAttrRefer,
	vocab.XSDAttrSubstitutionGroup,
	vocab.XSDAttrType,
}

func resolveSemanticQNameAttributes(n *schemaSyntaxNode) (semanticQNameProjection, error) {
	var projection semanticQNameProjection
	for _, local := range semanticQNameAttributeNames {
		if !schemaNodeHasQNameAttr(n, local) {
			continue
		}
		value, err := schemaQNameSourceFor(n, local)
		if err != nil {
			return semanticQNameProjection{}, err
		}
		setSemanticQNameAttribute(&projection, local, value)
	}
	return projection, nil
}

func setSemanticQNameAttribute(projection *semanticQNameProjection, local string, value schemaQNameAttribute) {
	switch local {
	case vocab.XSDAttrBase:
		projection.base = value
	case vocab.XSDAttrItemType:
		projection.itemType = value
	case vocab.XSDAttrRef:
		projection.ref = value
	case vocab.XSDAttrRefer:
		projection.refer = value
	case vocab.XSDAttrSubstitutionGroup:
		projection.substitutionGroup = value
	case vocab.XSDAttrType:
		projection.typeName = value
	}
}

func buildSemanticKind(n *schemaSyntaxNode, s *schemaSemanticSource, qnames *semanticQNameProjection) {
	switch n.Name.Local {
	case vocab.XSDElemSchema:
		s.Document = &schemaDocumentSource{
			ID:                   schemaLexicalAttributeSyntax(n, vocab.XSDAttrID),
			TargetNamespace:      schemaLexicalAttributeSyntax(n, vocab.XSDAttrTargetNamespace),
			Version:              schemaLexicalAttributeSyntax(n, vocab.XSDAttrVersion),
			FinalDefault:         schemaLexicalAttributeSyntax(n, vocab.XSDAttrFinalDefault),
			BlockDefault:         schemaLexicalAttributeSyntax(n, vocab.XSDAttrBlockDefault),
			ElementFormDefault:   schemaLexicalAttributeSyntax(n, vocab.XSDAttrElementFormDefault),
			AttributeFormDefault: schemaLexicalAttributeSyntax(n, vocab.XSDAttrAttributeFormDefault),
		}
	case vocab.XSDElemInclude, vocab.XSDElemImport:
		s.Reference = &schemaReferenceSource{
			ID:             schemaLexicalAttributeSyntax(n, vocab.XSDAttrID),
			Namespace:      schemaLexicalAttributeSyntax(n, vocab.XSDAttrNamespace),
			SchemaLocation: schemaLexicalAttributeSyntax(n, vocab.XSDAttrSchemaLocation),
		}
	case vocab.XSDElemSimpleType:
		s.SimpleType = &schemaSimpleTypeSource{
			Global:      schemaGlobalSourceFor(n),
			Final:       schemaLexicalAttributeSyntax(n, vocab.XSDAttrFinal),
			Base:        qnames.base,
			ItemType:    qnames.itemType,
			MemberTypes: qnames.memberTypes,
		}
	case vocab.XSDElemElement:
		s.Element = &schemaElementSource{
			Global:            schemaGlobalSourceFor(n),
			Ref:               qnames.ref,
			Type:              qnames.typeName,
			SubstitutionGroup: qnames.substitutionGroup,
			Default:           schemaLexicalAttributeSyntax(n, vocab.XSDAttrDefault),
			Fixed:             schemaLexicalAttributeSyntax(n, vocab.XSDAttrFixed),
			Form:              schemaLexicalAttributeSyntax(n, vocab.XSDAttrForm),
			Nillable:          schemaLexicalAttributeSyntax(n, vocab.XSDAttrNillable),
			Abstract:          schemaLexicalAttributeSyntax(n, vocab.XSDAttrAbstract),
			Block:             schemaLexicalAttributeSyntax(n, vocab.XSDAttrBlock),
			Final:             schemaLexicalAttributeSyntax(n, vocab.XSDAttrFinal),
			MinOccurs:         schemaLexicalAttributeSyntax(n, vocab.XSDAttrMinOccurs),
			MaxOccurs:         schemaLexicalAttributeSyntax(n, vocab.XSDAttrMaxOccurs),
		}
		// Element declarations are also particles when they occur inside a
		// model group. Keep occurrence facts in the particle capability so the
		// content-model compiler has one typed source for every particle kind.
		s.Particle = &schemaParticleSource{
			MinOccurs: schemaLexicalAttributeSyntax(n, vocab.XSDAttrMinOccurs),
			MaxOccurs: schemaLexicalAttributeSyntax(n, vocab.XSDAttrMaxOccurs),
			Ref:       qnames.ref,
		}
	case vocab.XSDElemAttribute:
		s.Attribute = &schemaAttributeSource{
			Global:  schemaGlobalSourceFor(n),
			Ref:     qnames.ref,
			Type:    qnames.typeName,
			Default: schemaLexicalAttributeSyntax(n, vocab.XSDAttrDefault),
			Fixed:   schemaLexicalAttributeSyntax(n, vocab.XSDAttrFixed),
			Form:    schemaLexicalAttributeSyntax(n, vocab.XSDAttrForm),
			Use:     schemaLexicalAttributeSyntax(n, vocab.XSDAttrUse),
		}
	case vocab.XSDElemComplexType:
		s.ComplexType = &schemaComplexTypeSource{
			Global:   schemaGlobalSourceFor(n),
			Mixed:    schemaLexicalAttributeSyntax(n, vocab.XSDAttrMixed),
			Abstract: schemaLexicalAttributeSyntax(n, vocab.XSDAttrAbstract),
			Block:    schemaLexicalAttributeSyntax(n, vocab.XSDAttrBlock),
			Final:    schemaLexicalAttributeSyntax(n, vocab.XSDAttrFinal),
		}
	case vocab.XSDElemRestriction, vocab.XSDElemExtension:
		s.Derivation = &schemaDerivationSource{ID: schemaLexicalAttributeSyntax(n, vocab.XSDAttrID), Base: qnames.base}
	case vocab.XSDElemComplexContent:
		s.Derivation = &schemaDerivationSource{ID: schemaLexicalAttributeSyntax(n, vocab.XSDAttrID), Mixed: schemaLexicalAttributeSyntax(n, vocab.XSDAttrMixed)}
	case vocab.XSDElemList:
		s.Derivation = &schemaDerivationSource{ID: schemaLexicalAttributeSyntax(n, vocab.XSDAttrID), ItemType: qnames.itemType}
	case vocab.XSDElemUnion:
		s.Derivation = &schemaDerivationSource{ID: schemaLexicalAttributeSyntax(n, vocab.XSDAttrID), MemberTypes: qnames.memberTypes}
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
		s.Particle = &schemaParticleSource{MinOccurs: schemaLexicalAttributeSyntax(n, vocab.XSDAttrMinOccurs), MaxOccurs: schemaLexicalAttributeSyntax(n, vocab.XSDAttrMaxOccurs), Ref: qnames.ref}
		if n.Name.Local != vocab.XSDElemGroup {
			kind := mustModelKindForLocal(n.Name.Local)
			s.Model = &schemaModelSource{Kind: kind, ChildIDs: schemaXSDChildIDs(n)}
		}
		if n.Name.Local == vocab.XSDElemGroup {
			s.Group = &schemaGroupSource{Global: schemaGlobalSourceFor(n), Ref: qnames.ref}
		}
	case vocab.XSDElemAny, vocab.XSDElemAnyAttribute:
		s.Particle = &schemaParticleSource{MinOccurs: schemaLexicalAttributeSyntax(n, vocab.XSDAttrMinOccurs), MaxOccurs: schemaLexicalAttributeSyntax(n, vocab.XSDAttrMaxOccurs)}
		s.Wildcard = &schemaWildcardSource{
			Namespace:       schemaLexicalAttributeSyntax(n, vocab.XSDAttrNamespace),
			NotNamespace:    schemaLexicalAttributeSyntax(n, vocab.XSDAttrNotNamespace),
			NotQName:        schemaLexicalAttributeSyntax(n, vocab.XSDAttrNotQName),
			ProcessContents: schemaLexicalAttributeSyntax(n, vocab.XSDAttrProcessContents),
		}
	case vocab.XSDElemKey, vocab.XSDElemKeyref, vocab.XSDElemUnique:
		s.Identity = &schemaIdentitySource{
			Name:     schemaLexicalAttributeSyntax(n, vocab.XSDAttrName),
			Refer:    qnames.refer,
			Fields:   schemaChildIDs(n, vocab.XSDElemField),
			Selector: schemaLexicalAttributeSyntax(n, vocab.XSDAttrXPath),
		}
	case vocab.XSDElemSelector, vocab.XSDElemField:
		s.IdentityXPath = &schemaIdentityXPathSource{XPath: schemaLexicalAttributeSyntax(n, vocab.XSDAttrXPath)}
	case vocab.XSDElemAttributeGroup:
		s.AttributeGroup = &schemaAttributeGroupSource{Global: schemaGlobalSourceFor(n), Ref: qnames.ref}
	case vocab.XSDElemNotation:
		s.Notation = &schemaNotationSource{Global: schemaGlobalSourceFor(n), Public: schemaLexicalAttributeSyntax(n, vocab.XSDAttrPublic), System: schemaLexicalAttributeSyntax(n, vocab.XSDAttrSystem)}
	default:
		if _, ok := facetMaskForLocal(n.Name.Local); ok {
			s.Facet = &schemaFacetSource{Value: schemaLexicalAttributeSyntax(n, vocab.XSDAttrValue), Fixed: schemaLexicalAttributeSyntax(n, vocab.XSDAttrFixed)}
		}
	}
}

func mustModelKindForLocal(local string) ModelKind {
	kind, err := ModelKindForLocal(local)
	if err != nil {
		panic(err)
	}
	return kind
}

func schemaNodeHasQNameAttr(n *schemaSyntaxNode, local string) bool {
	if n == nil || n.Name.Space != vocab.XSDNamespaceURI {
		return false
	}
	switch local {
	case vocab.XSDAttrBase:
		switch n.Name.Local {
		case vocab.XSDElemRestriction, vocab.XSDElemExtension:
			return true
		}
	case vocab.XSDAttrItemType:
		return n.Name.Local == vocab.XSDElemList
	case vocab.XSDAttrRef:
		switch n.Name.Local {
		case vocab.XSDElemElement, vocab.XSDElemAttribute, vocab.XSDElemGroup, vocab.XSDElemAttributeGroup:
			return true
		}
	case vocab.XSDAttrRefer:
		return n.Name.Local == vocab.XSDElemKeyref
	case vocab.XSDAttrSubstitutionGroup, vocab.XSDAttrType:
		return n.Name.Local == vocab.XSDElemElement || n.Name.Local == vocab.XSDElemAttribute
	case vocab.XSDAttrMemberTypes:
		return n.Name.Local == vocab.XSDElemUnion
	}
	return false
}

func schemaChildIDs(n *schemaSyntaxNode, local string) []schemaNodeID {
	if node, doc, ok := schemaSourceNode(n); ok {
		return schemaChildIDsFromNode(node, doc, local)
	}
	var ids []schemaNodeID
	for _, child := range n.Children {
		if child.Name.Space == vocab.XSDNamespaceURI && child.Name.Local == local {
			ids = append(ids, child.id)
		}
	}
	return ids
}

func schemaSourceNode(n *schemaSyntaxNode) (*schemaNode, *schemaDocument, bool) {
	if n == nil || n.doc == nil {
		return nil, nil, false
	}
	node := n.doc.node(n.id)
	return node, n.doc, node != nil
}

func schemaChildIDsFromNode(node *schemaNode, doc *schemaDocument, local string) []schemaNodeID {
	var ids []schemaNodeID
	for _, id := range node.children {
		child := doc.node(id)
		if child != nil && child.kind != schemaKindForeign && child.local == local {
			ids = append(ids, child.id)
		}
	}
	return ids
}

func schemaXSDChildIDs(n *schemaSyntaxNode) []schemaNodeID {
	if n == nil {
		return nil
	}
	if node, doc, ok := schemaSourceNode(n); ok {
		return schemaXSDChildIDsFromNode(node, doc)
	}
	var ids []schemaNodeID
	for _, child := range n.Children {
		if child.Name.Space == vocab.XSDNamespaceURI {
			ids = append(ids, child.id)
		}
	}
	return ids
}

func schemaXSDChildIDsFromNode(node *schemaNode, doc *schemaDocument) []schemaNodeID {
	var ids []schemaNodeID
	for _, id := range node.children {
		child := doc.node(id)
		if child != nil && child.kind != schemaKindForeign {
			ids = append(ids, child.id)
		}
	}
	return ids
}
