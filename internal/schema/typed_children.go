package schema

// This file contains the compiler-facing grammar checks. Parser admission
// uses the *Syntax variants in compile_children.go; compilation runs against
// compact semantic nodes and does not reconstruct XML nodes.

import (
	"errors"
	"math/bits"

	"github.com/jacoelho/xsd/xsderrors"
)

type contentDerivationSource struct {
	node *schemaNode
	kind ContentDerivationKind
}

func checkOrderedXSDChildren(n *schemaNode, order ChildOrder) error {
	state := childOrderState{maxLevelSeen: -1}
	index := 0
	for child := range n.xsdChildren() {
		if err := state.accept(index, child.local, order); err != nil {
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				return err
			}
			target := child
			if issue.Index < 0 {
				target = n
			}
			return schemaCompileAt(target, xsderrors.CodeSchemaContentModel, issue.Message)
		}
		index++
	}
	return nil
}

func checkComplexTypeChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, complexTypeChildOrder)
}

func checkComplexContentSyntax(n *schemaNode) (contentDerivationSource, error) {
	return checkContentDerivation(n, complexContent, complexContentChildOrder)
}

func checkSimpleContentSyntax(n *schemaNode) (contentDerivationSource, error) {
	return checkContentDerivation(n, simpleContent, simpleContentChildOrder)
}

func checkContentDerivation(n *schemaNode, container string, order ChildOrder) (contentDerivationSource, error) {
	if err := checkOrderedXSDChildren(n, order); err != nil {
		return contentDerivationSource{}, err
	}
	for child := range n.xsdChildren() {
		switch child.local {
		case extensionChild:
			return contentDerivationSource{node: child, kind: ContentDerivationExtension}, nil
		case restrictionChild:
			return contentDerivationSource{node: child, kind: ContentDerivationRestriction}, nil
		}
	}
	return contentDerivationSource{}, schemaCompileAt(n, xsderrors.CodeSchemaContentModel, container+" missing extension or restriction")
}

func checkSimpleContentRestrictionChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, simpleContentRestrictionChildOrder)
}

func checkSimpleContentExtensionChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, simpleContentExtensionChildOrder)
}

func checkComplexContentRestrictionChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, complexContentRestrictionChildOrder)
}

func checkComplexContentExtensionChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, complexContentExtensionChildOrder)
}

func checkAnyParticleChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, anyParticleChildOrder)
}

func checkAnyAttributeChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, anyAttributeChildOrder)
}

func checkElementRefChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, elementRefChildOrder)
}

func checkAttributeRefChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, attributeRefChildOrder)
}

func checkAttributeGroupUseChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, attributeGroupUseChildOrder)
}

func checkElementRefAttributes(n *schemaNode) error {
	return checkAllowedTypedAttributes(n, "element ref", isElementRefAttribute)
}

func checkAttributeRefAttributes(n *schemaNode) error {
	return checkAllowedTypedAttributes(n, "attribute ref", isAttributeRefAttribute)
}

func checkGroupOccurrenceAttributes(n *schemaNode) error {
	return checkAllowedTypedAttributes(n, "group", isGroupOccurrenceAttribute)
}

func checkAllowedTypedAttributes(n *schemaNode, label string, allowed func(string) bool) error {
	mask := n.attributeMask
	for mask != 0 {
		index := bits.TrailingZeros64(mask)
		local := typedAttributeNames[index]
		if !allowed(local) {
			return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, label+" cannot have attribute "+local)
		}
		mask &^= uint64(1) << index
	}
	return nil
}

func checkLocalElementAttributes(n *schemaNode) error {
	source := semanticSource[*schemaElementSource](&n.semantic)
	if source.Abstract.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "local element cannot have abstract")
	}
	if source.Final.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "local element cannot have final")
	}
	if source.SubstitutionGroup.Lexical.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "local element cannot have substitutionGroup")
	}
	return nil
}

func checkLocalElementSource(n *schemaNode) error {
	source := semanticSource[*schemaElementSource](&n.semantic)
	if err := ValidateLocalElementSource(NameReferenceSource{
		Name: source.Global.Name, Reference: source.Ref.Lexical,
	}); err != nil {
		return withSchemaCompileLocation(n, err)
	}
	return nil
}

func checkLocalSimpleTypeAttributes(n *schemaNode) error {
	source, ok := n.semantic.kind.(*schemaSimpleTypeSource)
	if !ok || source == nil {
		return withSchemaCompileLocation(n, xsderrors.InternalInvariant("simpleType node has no typed simpleType source"))
	}
	if source.Global.Name.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "local simpleType cannot have name")
	}
	if source.Final.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "local simpleType cannot have final")
	}
	return nil
}

func checkLocalComplexTypeAttributes(n *schemaNode) error {
	source := semanticSource[*schemaComplexTypeSource](&n.semantic)
	if source.Global.Name.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "local complexType cannot have name")
	}
	if source.Abstract.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "local complexType cannot have abstract")
	}
	if source.Block.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "local complexType cannot have block")
	}
	if source.Final.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "local complexType cannot have final")
	}
	return nil
}

func checkAttributeGroupDeclarationChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, attributeGroupDeclarationChildOrder)
}

//nolint:gocognit // Child grammar must complete before occurrence checks, with each failure attached to its source node.
func checkTopLevelGroupChildren(n *schemaNode) (*schemaNode, error) {
	var model *schemaNode
	seenAnnotation := false
	for child := range n.xsdChildren() {
		switch child.local {
		case annotationChild:
			if seenAnnotation {
				return nil, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "top-level group can contain at most one annotation")
			}
			if model != nil {
				return nil, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "top-level group annotation must be first")
			}
			seenAnnotation = true
		case sequenceChild, choiceChild, allChild:
			if model != nil {
				return nil, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "top-level group must contain exactly one model group")
			}
			model = child
		case groupChild:
			return nil, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "top-level group cannot contain group ref")
		default:
			return nil, schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "invalid top-level group child "+child.local)
		}
	}
	if model == nil {
		return nil, schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "top-level group must contain exactly one model group")
	}
	if semanticSource[*schemaModelSource](&model.semantic).MinOccurs.Present {
		return nil, schemaCompileAt(model, xsderrors.CodeSchemaOccurrence, "top-level model group cannot have minOccurs")
	}
	if semanticSource[*schemaModelSource](&model.semantic).MaxOccurs.Present {
		return nil, schemaCompileAt(model, xsderrors.CodeSchemaOccurrence, "top-level model group cannot have maxOccurs")
	}
	return model, nil
}

func checkNotationDeclaration(n *schemaNode) error {
	if n.hasNonWhitespaceText {
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "notation can contain only annotation")
	}
	for _, id := range n.children {
		child := n.doc.node(id)
		if child == nil || child.local != annotationChild {
			return schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "notation can contain only annotation")
		}
	}
	source, ok := n.semantic.kind.(*schemaNotationSource)
	if !ok || source == nil {
		return withSchemaCompileLocation(n, xsderrors.InternalInvariant("notation node has no typed notation source"))
	}
	if !source.Global.Name.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "notation missing name")
	}
	if !source.Public.Present && !source.System.Present {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "notation requires public or system")
	}
	return nil
}

func checkAttributeDeclarationChildren(n *schemaNode) error {
	return checkOrderedXSDChildren(n, attributeDeclarationChildOrder)
}

func checkAttributeUseSource(n *schemaNode) error {
	source := semanticSource[*schemaAttributeSource](&n.semantic)
	if err := ValidateAttributeUseSource(NameReferenceSource{
		Name: source.Global.Name, Reference: source.Ref.Lexical,
	}); err != nil {
		return withSchemaCompileLocation(n, err)
	}
	return nil
}

func checkAttributeGroupUseSource(n *schemaNode) error {
	if err := ValidateAttributeGroupUseSource(semanticSource[*schemaAttributeGroupSource](&n.semantic).Ref.Lexical); err != nil {
		return withSchemaCompileLocation(n, err)
	}
	return nil
}

func checkElementDeclarationChildren(n *schemaNode) error {
	if err := checkOrderedXSDChildren(n, elementDeclarationChildOrder); err != nil {
		return err
	}
	if semanticSource[*schemaElementSource](&n.semantic).Type.Lexical.Present && hasAnonymousTypedElementType(n) {
		return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "element cannot have both type and anonymous type")
	}
	return nil
}

func hasAnonymousTypedElementType(n *schemaNode) bool {
	for child := range n.xsdChildren() {
		if child.local == simpleTypeChild || child.local == complexTypeChild {
			return true
		}
	}
	return false
}

func checkContentDerivationBase(n *schemaNode, base ContentDerivationBase) error {
	if err := ValidateContentDerivationBase(base); err != nil {
		return withSchemaCompileLocation(n, err)
	}
	return nil
}

func checkSchemaQNameParts(n *schemaNode, lexical string) (QNameParts, error) {
	parts, err := ParseQNameParts(lexical)
	if err != nil {
		return QNameParts{}, withSchemaCompileLocation(n, err)
	}
	return parts, nil
}
