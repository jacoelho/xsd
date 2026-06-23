package compile

import (
	"errors"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlns"
	"github.com/jacoelho/xsd/xsderrors"
)

func checkComplexTypeChildren(n *rawNode) error {
	return checkChildOrderRules(n, complexTypeChildOrder)
}

func checkComplexContentSyntax(n *rawNode) (contentDerivationSource, error) {
	return checkContentDerivationSyntaxRules(n, complexContent, complexContentChildOrder)
}

func checkSimpleContentSyntax(n *rawNode) (contentDerivationSource, error) {
	return checkContentDerivationSyntaxRules(n, simpleContent, simpleContentChildOrder)
}

func checkSimpleContentRestrictionChildren(n *rawNode) error {
	return checkChildOrderRules(n, simpleContentRestrictionChildOrder)
}

func checkSimpleContentExtensionChildren(n *rawNode) error {
	return checkChildOrderRules(n, simpleContentExtensionChildOrder)
}

func checkComplexContentRestrictionChildren(n *rawNode) error {
	return checkChildOrderRules(n, complexContentRestrictionChildOrder)
}

func checkComplexContentExtensionChildren(n *rawNode) error {
	return checkChildOrderRules(n, complexContentExtensionChildOrder)
}

func checkAnyParticleChildren(n *rawNode) error {
	return checkChildOrderRules(n, anyParticleChildOrder)
}

func checkAnyAttributeChildren(n *rawNode) error {
	return checkChildOrderRules(n, anyAttributeChildOrder)
}

func checkElementRefChildren(n *rawNode) error {
	return checkChildOrderRules(n, elementRefChildOrder)
}

func checkAttributeRefChildren(n *rawNode) error {
	return checkChildOrderRules(n, attributeRefChildOrder)
}

func checkAttributeGroupUseChildren(n *rawNode) error {
	return checkChildOrderRules(n, attributeGroupUseChildOrder)
}

func checkElementRefAttributes(n *rawNode) error {
	return checkAllowedRawAttributes(n, "element ref", isElementRefAttribute)
}

func checkAttributeRefAttributes(n *rawNode) error {
	return checkAllowedRawAttributes(n, "attribute ref", isAttributeRefAttribute)
}

func checkGroupOccurrenceAttributes(n *rawNode) error {
	return checkAllowedRawAttributes(n, "group", isGroupOccurrenceAttribute)
}

func checkRawSchemaAttributes(n *rawNode) error {
	for _, attr := range n.Attr {
		if xmlns.IsNamespaceAttr(attr) || attr.Name.Space != "" {
			continue
		}
		if !schemaElementAttributeAllowed(n.Name.Local, attr.Name.Local) {
			return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, n.Name.Local+" cannot have attribute "+attr.Name.Local)
		}
	}
	return nil
}

func checkUnsupportedSchemaNode(n, parent *rawNode) (bool, error) {
	var parentLocal string
	var parentXSD bool
	if parent != nil {
		parentLocal = parent.Name.Local
		parentXSD = parent.Name.Space == vocab.XSDNamespaceURI
	}
	if parentXSD && parentLocal == annotationChild &&
		n.Name.Space == vocab.XSDNamespaceURI &&
		(n.Name.Local == vocab.XSDElemAppinfo || n.Name.Local == vocab.XSDElemDocumentation) {
		return true, nil
	}
	for _, attr := range n.Attr {
		if attr.Name.Space == vocab.XSDNamespaceURI {
			return false, schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "schema namespace attribute "+attr.Name.Local+" is not allowed")
		}
	}
	if n.Name.Space != vocab.XSDNamespaceURI {
		return false, nil
	}
	switch n.Name.Local {
	case redefineChild:
		return false, xsderrors.Unsupported(xsderrors.CodeUnsupportedRedefine, "xs:redefine is not supported")
	case notationChild:
		if !parentXSD || parentLocal != vocab.XSDElemSchema {
			return false, schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "xs:notation must be a top-level schema child")
		}
	case assertChild, "alternative", "override", "openContent", "defaultOpenContent":
		return false, xsderrors.Unsupported(xsderrors.CodeUnsupportedXSD11, "XSD 1.1 feature "+n.Name.Local+" is not supported")
	case anyChild, anyAttribute:
		for _, attr := range []string{vocab.XSDAttrNotNamespace, vocab.XSDAttrNotQName} {
			if _, ok := n.attr(attr); ok {
				return false, xsderrors.Unsupported(xsderrors.CodeUnsupportedXSD11, "XSD 1.1 wildcard attribute "+attr+" is not supported")
			}
		}
	}
	return false, nil
}

type schemaIDNode struct {
	node *rawNode
	id   string
}

func checkSchemaIDs(root *rawNode) error {
	var nodes []schemaIDNode
	collectSchemaIDs(root, &nodes)
	ids := make([]SchemaID, len(nodes))
	for i, node := range nodes {
		ids[i] = SchemaID{Value: node.id}
	}
	if err := ValidateSchemaIDs(ids); err != nil {
		if issue, ok := errors.AsType[*SchemaIDError](err); ok {
			if issue.Index >= 0 && issue.Index < len(nodes) {
				return schemaCompileAt(nodes[issue.Index].node, issue.Code, issue.Message)
			}
		}
		return err
	}
	return nil
}

func collectSchemaIDs(n *rawNode, out *[]schemaIDNode) {
	if id, ok := n.attr(vocab.XSDAttrID); ok {
		*out = append(*out, schemaIDNode{node: n, id: id})
	}
	for _, child := range n.Children {
		collectSchemaIDs(child, out)
	}
}

func checkSchemaNodeNames(n, parent *rawNode) error {
	var parentLocal string
	var parentXSD bool
	if parent != nil {
		parentLocal = parent.Name.Local
		parentXSD = parent.Name.Space == vocab.XSDNamespaceURI
	}
	id, hasID := n.attr(vocab.XSDAttrID)
	name, hasName := n.attr(vocab.XSDAttrName)
	return withSchemaCompileLocation(n, ValidateSchemaNodeNames(SchemaNodeNames{
		Local:       n.Name.Local,
		ParentLocal: parentLocal,
		ID:          id,
		Name:        name,
		XSD:         n.Name.Space == vocab.XSDNamespaceURI,
		ParentXSD:   parentXSD,
		HasID:       hasID,
		HasName:     hasName,
	}))
}

func checkSchemaAnnotationNode(n *rawNode) (bool, error) {
	action, err := validateRawSchemaAnnotationNode(n)
	if err != nil {
		if issue, ok := errors.AsType[*SchemaAnnotationSyntaxError](err); ok {
			return false, schemaAnnotationSyntaxIssueAt(n, issue)
		}
		return false, withSchemaCompileLocation(n, err)
	}
	return action.SkipChildren, nil
}

func checkSchemaQNameParts(n *rawNode, lexical string) (string, string, bool, error) {
	parts, err := ParseQNameParts(lexical)
	if err != nil {
		return "", "", false, withSchemaCompileLocation(n, err)
	}
	return parts.Prefix, parts.Local, parts.Prefixed, nil
}

type contentDerivationSource struct {
	node *rawNode
	kind ContentDerivationKind
}

func checkContentDerivationBase(container string, derivation ContentDerivationKind, n *rawNode, hasBase bool) error {
	if err := ValidateContentDerivationBase(container, derivation.String(), hasBase); err != nil {
		return withSchemaCompileLocation(n, err)
	}
	return nil
}

func checkContentDerivationSyntaxRules(n *rawNode, container string, order ChildOrder) (contentDerivationSource, error) {
	syntax, err := validateDerivationContainerChildrenRaw(n, container, order)
	if err != nil {
		if issue, ok := errors.AsType[*ChildOrderError](err); ok {
			return contentDerivationSource{}, childOrderIssueAtRaw(n, issue)
		}
		return contentDerivationSource{}, err
	}
	child, ok := xsdChildAt(n, syntax.Index)
	if !ok {
		return contentDerivationSource{}, xsderrors.InternalInvariant("content derivation validator returned invalid child index")
	}
	switch syntax.Kind {
	case ContentDerivationExtension, ContentDerivationRestriction:
	default:
		return contentDerivationSource{}, xsderrors.InternalInvariant("content derivation validator returned missing kind")
	}
	return contentDerivationSource{node: child, kind: syntax.Kind}, nil
}

func validateDerivationContainerChildrenRaw(n *rawNode, label string, order ChildOrder) (ContentDerivationSyntax, error) {
	syntax := ContentDerivationSyntax{Index: -1}
	if err := checkOrderedRawXSDChildren(n, order); err != nil {
		return syntax, err
	}
	i := 0
	for child := range n.xsdChildren() {
		switch child.Name.Local {
		case extensionChild:
			return ContentDerivationSyntax{Index: i, Kind: ContentDerivationExtension}, nil
		case restrictionChild:
			return ContentDerivationSyntax{Index: i, Kind: ContentDerivationRestriction}, nil
		}
		i++
	}
	return syntax, childOrderError(-1, label+" missing extension or restriction")
}

func parseUnionMemberTypes(n *rawNode, memberTypes string, hasMemberTypes, hasSimpleTypeChild bool) ([]string, error) {
	members, err := ParseUnionMemberTypes(memberTypes, hasMemberTypes, hasSimpleTypeChild)
	if err != nil {
		return nil, withSchemaCompileLocation(n, err)
	}
	return members, nil
}

func schemaAnnotationSyntaxIssueAt(n *rawNode, issue *SchemaAnnotationSyntaxError) error {
	target := n
	if issue.Index >= 0 {
		if issue.Index >= len(n.Children) {
			return issue
		}
		target = n.Children[issue.Index]
	}
	return schemaCompileAt(target, issue.Code, issue.Message)
}

func validateRawSchemaAnnotationNode(n *rawNode) (schemaAnnotationAction, error) {
	if n.Name.Space != vocab.XSDNamespaceURI {
		return schemaAnnotationAction{}, nil
	}
	switch n.Name.Local {
	case vocab.XSDElemAppinfo:
		return schemaAnnotationAction{SkipChildren: true}, nil
	case vocab.XSDElemDocumentation:
		for _, attr := range n.Attr {
			if attr.Name.Space == vocab.XMLNamespaceURI && attr.Name.Local == vocab.XMLAttrLang && !lex.IsLanguage(attr.Value) {
				return schemaAnnotationAction{SkipChildren: true}, schemaAnnotationSyntaxError(-1, xsderrors.CodeSchemaInvalidAttribute, "invalid xml:lang on xs:documentation")
			}
		}
		return schemaAnnotationAction{SkipChildren: true}, nil
	case annotationChild:
		return schemaAnnotationAction{}, validateRawAnnotationElement(n)
	case vocab.XSDElemSchema:
		return schemaAnnotationAction{}, nil
	default:
		return schemaAnnotationAction{}, validateRawComponentAnnotationPlacement(n)
	}
}

func validateRawAnnotationElement(n *rawNode) error {
	for _, attr := range n.Attr {
		if attr.Name.Space == "" && attr.Name.Local != vocab.XSDAttrID {
			return schemaAnnotationSyntaxError(-1, xsderrors.CodeSchemaInvalidAttribute, "attribute "+attr.Name.Local+" cannot appear on xs:annotation")
		}
	}
	for i, child := range n.Children {
		if child.Name.Space == vocab.XSDNamespaceURI && child.Name.Local == annotationChild {
			return schemaAnnotationSyntaxError(i, xsderrors.CodeSchemaContentModel, "xs:annotation cannot contain xs:annotation")
		}
	}
	return nil
}

func validateRawComponentAnnotationPlacement(n *rawNode) error {
	annotations := 0
	seenNonAnnotation := false
	for i, child := range n.Children {
		if child.Name.Space != vocab.XSDNamespaceURI {
			continue
		}
		if child.Name.Local == annotationChild {
			annotations++
			if annotations > 1 {
				return schemaAnnotationSyntaxError(i, xsderrors.CodeSchemaContentModel, "schema component cannot contain multiple annotations")
			}
			if seenNonAnnotation {
				return schemaAnnotationSyntaxError(i, xsderrors.CodeSchemaContentModel, n.Name.Local+" annotation must be first")
			}
			continue
		}
		seenNonAnnotation = true
	}
	return nil
}

func checkLocalElementAttributes(n *rawNode) error {
	for _, attr := range []string{vocab.XSDAttrAbstract, vocab.XSDAttrFinal, vocab.XSDAttrSubstitutionGroup} {
		if _, ok := n.attr(attr); ok {
			return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "local element cannot have "+attr)
		}
	}
	return nil
}

func checkLocalElementSource(n *rawNode) error {
	_, hasName := n.attr(vocab.XSDAttrName)
	_, hasRef := n.attr(vocab.XSDAttrRef)
	if err := ValidateLocalElementSource(hasName, hasRef); err != nil {
		return withSchemaCompileLocation(n, err)
	}
	return nil
}

func checkLocalSimpleTypeAttributes(n *rawNode) error {
	if _, ok := n.attr(vocab.XSDAttrName); ok {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "local simpleType cannot have name")
	}
	return nil
}

func checkLocalComplexTypeAttributes(n *rawNode) error {
	if _, ok := n.attr(vocab.XSDAttrName); ok {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "local complexType cannot have name")
	}
	return nil
}

func checkAttributeGroupDeclarationChildren(n *rawNode) error {
	return checkChildOrderRules(n, attributeGroupDeclarationChildOrder)
}

func checkTopLevelGroupChildren(n *rawNode) (*rawNode, error) {
	children := xsdChildren(n)
	in := make([]TopLevelGroupChild, len(children))
	for i, child := range children {
		_, hasMinOccurs := child.attr(vocab.XSDAttrMinOccurs)
		_, hasMaxOccurs := child.attr(vocab.XSDAttrMaxOccurs)
		in[i] = TopLevelGroupChild{
			Local:        child.Name.Local,
			HasMinOccurs: hasMinOccurs,
			HasMaxOccurs: hasMaxOccurs,
		}
	}
	syntax, err := ValidateTopLevelGroupChildren(in)
	if err != nil {
		if issue, ok := errors.AsType[*TopLevelGroupSyntaxError](err); ok {
			return nil, topLevelGroupSyntaxIssueAt(n, children, issue)
		}
		return nil, err
	}
	if syntax.Model < 0 || syntax.Model >= len(children) {
		return nil, xsderrors.InternalInvariant("top-level group syntax returned invalid model index")
	}
	return children[syntax.Model], nil
}

func topLevelGroupSyntaxIssueAt(n *rawNode, children []*rawNode, issue *TopLevelGroupSyntaxError) error {
	target := n
	if issue.Index >= 0 {
		if issue.Index >= len(children) {
			return issue
		}
		target = children[issue.Index]
	}
	return schemaCompileAt(target, issue.Code, issue.Message)
}

func checkNotationDeclaration(n *rawNode) error {
	children := make([]NotationChild, len(n.Children))
	for i, child := range n.Children {
		children[i] = NotationChild{
			Local: child.Name.Local,
			XSD:   child.Name.Space == vocab.XSDNamespaceURI,
		}
	}
	_, hasName := n.attr(vocab.XSDAttrName)
	_, hasPublic := n.attr(vocab.XSDAttrPublic)
	_, hasSystem := n.attr(vocab.XSDAttrSystem)
	if err := ValidateNotationDeclaration(n.Text, children, hasName, hasPublic, hasSystem); err != nil {
		if issue, ok := errors.AsType[*NotationSyntaxError](err); ok {
			return notationSyntaxIssueAt(n, issue)
		}
		return err
	}
	return nil
}

func notationSyntaxIssueAt(n *rawNode, issue *NotationSyntaxError) error {
	target := n
	if issue.Index >= 0 {
		if issue.Index >= len(n.Children) {
			return issue
		}
		target = n.Children[issue.Index]
	}
	return schemaCompileAt(target, issue.Code, issue.Message)
}

func checkTopLevelSchemaChild(n *rawNode) error {
	child := TopLevelSchemaChild{Local: n.Name.Local}
	for _, attr := range n.Attr {
		if attr.Name.Space != "" {
			continue
		}
		switch attr.Name.Local {
		case vocab.XSDAttrName:
			child.HasName = true
		case vocab.XSDAttrRef:
			child.HasRef = true
		case vocab.XSDAttrForm:
			child.HasForm = true
		case vocab.XSDAttrUse:
			child.HasUse = true
		case vocab.XSDAttrMinOccurs:
			child.HasMinOccurs = true
		case vocab.XSDAttrMaxOccurs:
			child.HasMaxOccurs = true
		}
	}
	err := ValidateTopLevelSchemaChild(child)
	if err != nil {
		if issue, ok := errors.AsType[*TopLevelSchemaChildError](err); ok {
			return schemaCompileAt(n, issue.Code, issue.Message)
		}
		return err
	}
	return nil
}

func checkAttributeDeclarationChildren(n *rawNode) error {
	return checkChildOrder(n, ValidateAttributeDeclarationChildren)
}

func checkAttributeUseSource(n *rawNode) error {
	_, hasName := n.attr(vocab.XSDAttrName)
	_, hasRef := n.attr(vocab.XSDAttrRef)
	if err := ValidateAttributeUseSource(hasName, hasRef); err != nil {
		return withSchemaCompileLocation(n, err)
	}
	return nil
}

func checkAttributeGroupUseSource(n *rawNode) error {
	_, hasRef := n.attr(vocab.XSDAttrRef)
	if err := ValidateAttributeGroupUseSource(hasRef); err != nil {
		return withSchemaCompileLocation(n, err)
	}
	return nil
}

func checkElementDeclarationChildren(n *rawNode) error {
	_, hasTypeAttr := n.attr(vocab.XSDAttrType)
	if err := checkOrderedRawXSDChildren(n, elementDeclarationChildOrder); err != nil {
		if issue, ok := errors.AsType[*ChildOrderError](err); ok {
			return childOrderIssueAtRaw(n, issue)
		}
		return err
	}
	if hasTypeAttr && hasAnonymousRawElementType(n) {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "element cannot have both type and anonymous type")
	}
	return nil
}

func hasAnonymousRawElementType(n *rawNode) bool {
	for child := range n.xsdChildren() {
		if child.Name.Local == simpleTypeChild || child.Name.Local == complexTypeChild {
			return true
		}
	}
	return false
}

func checkIdentityConstraintChildren(n *rawNode) (identityConstraintSyntax, error) {
	var out identityConstraintSyntax
	children := xsdChildren(n)
	in := make([]IdentityConstraintChild, len(children))
	for i, child := range children {
		xpath, hasXPath := child.attr(vocab.XSDAttrXPath)
		in[i] = IdentityConstraintChild{
			Local:    child.Name.Local,
			XPath:    xpath,
			Children: childLocalNames(xsdChildren(child)),
			HasXPath: hasXPath,
		}
	}
	syntax, err := ValidateIdentityConstraintChildren(in)
	if err != nil {
		if issue, ok := errors.AsType[*IdentityConstraintSyntaxError](err); ok {
			return out, identityConstraintSyntaxIssueAt(n, children, issue)
		}
		return out, err
	}
	out.selector = children[syntax.Selector]
	out.fields = make([]*rawNode, 0, len(syntax.Fields))
	for _, index := range syntax.Fields {
		out.fields = append(out.fields, children[index])
	}
	return out, nil
}

func identityConstraintSyntaxIssueAt(n *rawNode, children []*rawNode, issue *IdentityConstraintSyntaxError) error {
	target := n
	if issue.ChildIndex >= 0 {
		if issue.ChildIndex >= len(children) {
			return issue
		}
		target = children[issue.ChildIndex]
		if issue.NestedChildIndex >= 0 {
			nested := xsdChildren(target)
			if issue.NestedChildIndex >= len(nested) {
				return issue
			}
			target = nested[issue.NestedChildIndex]
		}
	}
	return schemaCompileAt(target, issue.Code, issue.Message)
}

func checkChildOrder(n *rawNode, validate func([]string) error) error {
	return checkChildOrderWithParentCode(n, validate, xsderrors.CodeSchemaContentModel)
}

func checkChildOrderWithParentCode(n *rawNode, validate func([]string) error, parentCode xsderrors.Code) error {
	children := xsdChildren(n)
	locals := childLocalNames(children)
	if err := validate(locals); err != nil {
		if issue, ok := errors.AsType[*ChildOrderError](err); ok {
			return childOrderIssueAt(n, children, issue, parentCode)
		}
		return err
	}
	return nil
}

func checkChildOrderRules(n *rawNode, order ChildOrder) error {
	if err := checkOrderedRawXSDChildren(n, order); err != nil {
		if issue, ok := errors.AsType[*ChildOrderError](err); ok {
			return childOrderIssueAtRaw(n, issue)
		}
		return err
	}
	return nil
}

func checkOrderedRawXSDChildren(n *rawNode, order ChildOrder) error {
	var seen uint64
	annotationSeen := false
	nonAnnotationSeen := false
	terminalSeen := false
	maxLevelSeen := -1
	childIndex := 0
	for child := range n.xsdChildren() {
		local := child.Name.Local
		if terminalSeen {
			return childOrderError(childIndex, order.InvalidMsg(local))
		}
		if local == annotationChild {
			if nonAnnotationSeen || (order.SingleAnnotation && annotationSeen) {
				return childOrderError(childIndex, order.AnnotationFirstMsg)
			}
			annotationSeen = true
			childIndex++
			continue
		}
		idx := childRuleIndex(local, order.Rules)
		if idx < 0 {
			return childOrderError(childIndex, order.InvalidMsg(local))
		}
		rule := order.Rules[idx]
		if rule.ForbiddenMsg != "" {
			return childOrderError(childIndex, rule.ForbiddenMsg)
		}
		nonAnnotationSeen = true
		if maxLevelSeen > rule.Level {
			return childOrderError(childIndex, rule.OrderMsg)
		}
		bit, err := childRuleSeenBit(idx)
		if err != nil {
			return err
		}
		if seen&bit != 0 && rule.MaxOne {
			return childOrderError(childIndex, rule.DupMsg)
		}
		seen |= bit
		maxLevelSeen = max(maxLevelSeen, rule.Level)
		terminalSeen = rule.Terminal
		childIndex++
	}
	return nil
}

func childOrderIssueAt(n *rawNode, children []*rawNode, issue *ChildOrderError, parentCode xsderrors.Code) error {
	if issue.Index < 0 {
		return schemaCompileAt(n, parentCode, issue.Message)
	}
	if issue.Index < len(children) {
		return schemaCompileAt(children[issue.Index], xsderrors.CodeSchemaContentModel, issue.Message)
	}
	return issue
}

func childOrderIssueAtRaw(n *rawNode, issue *ChildOrderError) error {
	if issue.Index < 0 {
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, issue.Message)
	}
	if child, ok := xsdChildAt(n, issue.Index); ok {
		return schemaCompileAt(child, xsderrors.CodeSchemaContentModel, issue.Message)
	}
	return issue
}

func xsdChildren(n *rawNode) []*rawNode {
	children := make([]*rawNode, 0, len(n.Children))
	for child := range n.xsdChildren() {
		children = append(children, child)
	}
	return children
}

func xsdChildAt(n *rawNode, index int) (*rawNode, bool) {
	if index < 0 {
		return nil, false
	}
	i := 0
	for child := range n.xsdChildren() {
		if i == index {
			return child, true
		}
		i++
	}
	return nil, false
}

func childLocalNames(children []*rawNode) []string {
	locals := make([]string, len(children))
	for i, child := range children {
		locals[i] = child.Name.Local
	}
	return locals
}

func checkAllowedRawAttributes(n *rawNode, label string, allowed func(string) bool) error {
	for _, attr := range n.Attr {
		if xmlns.IsNamespaceAttr(attr) || attr.Name.Space != "" {
			continue
		}
		if !allowed(attr.Name.Local) {
			return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, label+" cannot have attribute "+attr.Name.Local)
		}
	}
	return nil
}
