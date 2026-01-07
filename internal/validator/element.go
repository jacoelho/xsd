package validator

import (
	"strings"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/grammar/contentmodel"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

func (r *validationRun) checkElement(elem xml.Element, path string) []errors.Validation {
	return r.checkElementWithProcessContents(elem, types.Strict, errors.ErrElementNotDeclared, path)
}

// checkElementWithProcessContents validates an element using processContents rules.
// missingCode controls the error code used when strict processing finds no declaration.
func (r *validationRun) checkElementWithProcessContents(elem xml.Element, processContents types.ProcessContents, missingCode errors.ErrorCode, path string) []errors.Validation {
	if processContents == types.Skip {
		return nil
	}

	qname := r.resolveElementQName(elem)
	decl := r.findElementDeclaration(qname)

	if decl == nil {
		// check for xsi:type - per XSD spec, if no element declaration exists but
		// xsi:type is specified, we can validate using just the type definition.
		// this is especially common for "schema-less" root elements.
		xsiTypeAttr := elem.GetAttributeNS(xml.XSINamespace, "type")
		if xsiTypeAttr != "" {
			xsiType, err := r.resolveXsiTypeOnly(elem, xsiTypeAttr)
			if err == nil && xsiType != nil {
				return r.checkElementWithType(elem, xsiType, path)
			}
		}

		switch processContents {
		case types.Strict:
			if missingCode == errors.ErrWildcardNotDeclared {
				return []errors.Validation{errors.NewValidationf(missingCode, path,
					"Element '%s' is not declared (strict wildcard requires declaration)", elem.LocalName())}
			}
			return []errors.Validation{errors.NewValidationf(missingCode, path,
				"Cannot find declaration for element '%s'", elem.LocalName())}
		case types.Lax:
			return nil
		}
		return nil
	}

	return r.checkElementWithDecl(elem, decl, path)
}

func (r *validationRun) checkElementWithDecl(elem xml.Element, decl *grammar.CompiledElement, path string) []errors.Validation {
	var violations []errors.Validation

	actualQName := r.resolveElementQName(elem)
	if decl != nil && actualQName != decl.QName {
		decl = r.resolveSubstitutionDecl(actualQName, decl)
	}

	if decl.Abstract {
		return []errors.Validation{errors.NewValidationf(errors.ErrElementAbstract, path,
			"Element '%s' is abstract and cannot be used directly in instance documents", elem.LocalName())}
	}

	nilAttr := elem.GetAttributeNS(xml.XSINamespace, "nil")
	hasNilAttr := nilAttr != ""
	isNil := nilAttr == "true" || nilAttr == "1"

	// per XSD spec, xsi:nil can only appear on elements declared as nillable
	// this is true even if the value is "false"
	if hasNilAttr && !decl.Nillable {
		violations = append(violations, errors.NewValidationf(errors.ErrElementNotNillable, path,
			"Element '%s' is not nillable but has xsi:nil attribute", elem.LocalName()))
		return violations
	}

	if decl.Type == nil {
		if isNil {
			// no type, nillable element with xsi:nil=true - just check for empty content
			textContent := strings.TrimSpace(elem.TextContent())
			if textContent != "" || len(elem.Children()) > 0 {
				violations = append(violations, errors.NewValidation(errors.ErrNilElementNotEmpty,
					"Element with xsi:nil='true' must be empty", path))
			}
		}
		return violations
	}

	effectiveType := decl.Type
	xsiTypeAttr := elem.GetAttributeNS(xml.XSINamespace, "type")
	if xsiTypeAttr != "" {
		xsiType, err := r.resolveXsiType(elem, xsiTypeAttr, decl.Type, decl.Block)
		if err != nil {
			violations = append(violations, errors.NewValidation(errors.ErrXsiTypeInvalid, err.Error(), path))
			return violations
		}
		if xsiType != nil {
			effectiveType = xsiType
		}
	}

	if effectiveType.Abstract {
		violations = append(violations, errors.NewValidationf(errors.ErrElementTypeAbstract, path,
			"Type '%s' is abstract and cannot be used for element '%s'", effectiveType.QName.String(), elem.LocalName()))
		return violations
	}

	// when xsi:nil is true, validate that element is empty but still check attributes
	// per XSD spec - the type's attribute declarations still apply
	if isNil {
		// per XSD spec: if element has a fixed value, xsi:nil="true" is invalid
		// because fixed values require element content, but xsi:nil means no content
		if decl.HasFixed {
			violations = append(violations, errors.NewValidationf(errors.ErrElementFixedValue, path,
				"Element '%s' has a fixed value constraint and cannot be nil", elem.LocalName()))
			return violations
		}
		textContent := strings.TrimSpace(elem.TextContent())
		if textContent != "" || len(elem.Children()) > 0 {
			violations = append(violations, errors.NewValidation(errors.ErrNilElementNotEmpty,
				"Element with xsi:nil='true' must be empty", path))
		}
		// still validate attributes against the effective type
		violations = append(violations, r.checkAttributes(elem, effectiveType.AllAttributes, effectiveType.AnyAttribute, path)...)
		return violations
	}

	return r.checkElementContent(elem, effectiveType, decl, path)
}

// checkElementContent validates element content using aspect-based validation.
// This unified approach handles simple types, complex types with simpleContent,
// and complex types with element content.
func (r *validationRun) checkElementContent(elem xml.Element, ct *grammar.CompiledType, decl *grammar.CompiledElement, path string) []errors.Validation {
	if isAnyType(ct) {
		return r.validateAnyTypeContent(elem, decl, path)
	}

	var violations []errors.Validation

	// always check attributes, even if type has none declared, to reject undeclared attributes
	// when AnyAttribute is nil (empty intersection) or absent.
	violations = append(violations, r.checkAttributes(elem, ct.AllAttributes, ct.AnyAttribute, path)...)

	textViolations, stop := r.validateTextContent(elem, ct, decl, path)
	violations = append(violations, textViolations...)
	if stop {
		return violations
	}

	emptyViolations, stop := r.validateEmptyContentModel(elem, ct, path)
	violations = append(violations, emptyViolations...)
	if stop {
		return violations
	}

	violations = append(violations, r.validateContentModel(elem, ct, decl, path)...)

	return violations
}

func (r *validationRun) validateAnyTypeContent(elem xml.Element, decl *grammar.CompiledElement, path string) []errors.Validation {
	var violations []errors.Validation

	// for anyType, we still need to check fixed values before returning
	// per XSD spec 3.3.4: If there is a fixed {value constraint}, the element
	// information item must have no element information item children.
	if decl != nil && decl.HasFixed {
		if len(elem.Children()) > 0 {
			violations = append(violations, errors.NewValidationf(errors.ErrElementFixedValue, path,
				"Element '%s' has a fixed value constraint and cannot have element children", decl.QName.Local))
		} else {
			textContent := elem.TextContent()
			// per XSD spec 9.1.1: If element is empty, the fixed value is supplied.
			// only validate non-empty content (anyType has whiteSpace=preserve).
			if textContent != "" && textContent != decl.Fixed {
				violations = append(violations, errors.NewValidationf(errors.ErrElementFixedValue, path,
					"Element has fixed value '%s' but actual value is '%s'", decl.Fixed, textContent))
			}
		}
	}

	violations = append(violations, r.checkAnyTypeContent(elem, path)...)
	return violations
}

func (r *validationRun) validateTextContent(elem xml.Element, ct *grammar.CompiledType, decl *grammar.CompiledElement, path string) ([]errors.Validation, bool) {
	var violations []errors.Validation

	textType := ct.TextType()
	if textType != nil {
		text := elem.TextContent()
		hadContent := text != ""
		// if element is empty and has a default or fixed value, use it
		if text == "" && decl != nil {
			if decl.Default != "" {
				text = decl.Default
			} else if decl.HasFixed {
				text = decl.Fixed
			}
		}
		violations = append(violations, r.checkSimpleValue(text, textType, path, elem)...)
		violations = append(violations, r.collectIDRefs(text, textType, path)...)

		// also validate additional facets on complex types with simpleContent
		if ct.Kind == grammar.TypeKindComplex && len(ct.Facets) > 0 {
			violations = append(violations, r.checkComplexTypeFacets(text, ct, path)...)
		}

		// check fixed value constraint only when element had actual content.
		// if element was empty, fixed value was applied above and is valid by definition.
		if decl != nil && decl.HasFixed && hadContent {
			violations = append(violations, r.checkFixedValue(text, decl.Fixed, textType, path)...)
		}

		// simple type content or complex type with simple content cannot have element children
		// (only text content is allowed)
		// exception: If the type has a content model, it can have element children
		if len(elem.Children()) > 0 && !ct.HasContentModel() {
			for _, child := range elem.Children() {
				childPath := appendPath(path, child.LocalName())
				violations = append(violations, errors.NewValidationf(errors.ErrTextInElementOnly, childPath,
					"Element '%s' is not allowed in simple content", child.LocalName()))
			}
			return violations, true
		}
		return violations, false
	}

	if ct.Kind == grammar.TypeKindComplex && ct.Mixed && !ct.HasContentModel() {
		// mixed content type with no content model (empty mixed content)
		// per XSD spec 3.3.4: If there is a fixed {value constraint}, the element
		// information item must have no element information item children.
		if decl != nil && decl.HasFixed {
			if len(elem.Children()) > 0 {
				violations = append(violations, errors.NewValidationf(errors.ErrElementFixedValue, path,
					"Element '%s' has a fixed value constraint and cannot have element children", decl.QName.Local))
			} else {
				textContent := elem.TextContent()
				violations = append(violations, r.checkFixedValue(textContent, decl.Fixed, textTypeForFixedValue(decl), path)...)
			}
		}
		return violations, false
	}

	if ct.Kind == grammar.TypeKindComplex && !ct.AllowsText() {
		// complex type that doesn't allow text (empty content or element-only content)
		// reject non-whitespace DIRECT text content (text nodes that are direct children,
		// not text inside nested child elements)
		if text := elem.DirectTextContent(); !isWhitespaceOnly(text) {
			violations = append(violations, errors.NewValidation(errors.ErrTextInElementOnly,
				"Element content cannot have character children (non-whitespace text found)", path))
		}
	}

	return violations, false
}

func (r *validationRun) validateEmptyContentModel(elem xml.Element, ct *grammar.CompiledType, path string) ([]errors.Validation, bool) {
	if ct.ContentModel == nil || !ct.ContentModel.Empty || ct.ContentModel.Mixed {
		return nil, false
	}

	var violations []errors.Validation
	if children := elem.Children(); len(children) > 0 {
		for _, child := range children {
			childPath := appendPath(path, child.LocalName())
			violations = append(violations, errors.NewValidationf(errors.ErrUnexpectedElement, childPath,
				"Element '%s' is not allowed. No element declaration found for it in the empty content model.", child.LocalName()))
		}
		return violations, true
	}

	return nil, false
}

func (r *validationRun) validateContentModel(elem xml.Element, ct *grammar.CompiledType, decl *grammar.CompiledElement, path string) []errors.Validation {
	if !ct.HasContentModel() {
		return nil
	}

	var violations []errors.Validation

	// check element-only constraint (redundant if we already checked above, but
	// this is more specific for element-only content models)
	if !ct.Mixed {
		if directText := elem.DirectTextContent(); !isWhitespaceOnly(directText) {
			violations = append(violations, errors.NewValidation(errors.ErrTextInElementOnly,
				"Element-only content cannot have character children (non-whitespace text found)", path))
		}
	}

	// per XSD spec 3.3.4: If there is a fixed {value constraint}, the element
	// information item must have no element information item children.
	// this applies even to mixed content types.
	if decl != nil && decl.HasFixed && len(elem.Children()) > 0 {
		violations = append(violations, errors.NewValidationf(errors.ErrElementFixedValue, path,
			"Element '%s' has a fixed value constraint and cannot have element children", decl.QName.Local))
	} else if ct.Mixed && decl != nil && decl.HasFixed {
		// for mixed content types with fixed value (and no element children),
		// if element is empty, use the fixed value as the default (like simple types)
		textContent := elem.TextContent()
		if textContent == "" {
			textContent = decl.Fixed
		}
		violations = append(violations, r.checkFixedValue(textContent, decl.Fixed, textTypeForFixedValue(decl), path)...)
	}

	matches, contentViolations := r.checkContentModel(elem, ct.ContentModel, path)
	violations = append(violations, contentViolations...)

	// recurse into children based on matches
	violations = append(violations, r.checkMatchedChildren(elem, ct.ContentModel, matches, path)...)

	return violations
}

// checkAnyTypeContent handles validation for xs:anyType which allows any content.
func (r *validationRun) checkAnyTypeContent(elem xml.Element, path string) []errors.Validation {
	var violations []errors.Validation

	// anyType allows any attributes - use a wildcard that allows all
	anyAttr := &types.AnyAttribute{
		Namespace:       types.NSCAny,
		ProcessContents: types.Lax,
		TargetNamespace: types.NamespaceEmpty,
	}
	violations = append(violations, r.checkAttributes(elem, nil, anyAttr, path)...)

	// anyType allows any child elements - validate in lax mode
	for _, child := range elem.Children() {
		childPath := appendPath(path, child.LocalName())
		violations = append(violations, r.checkElementWithProcessContents(child, types.Lax, errors.ErrWildcardNotDeclared, childPath)...)
	}

	return violations
}

// checkMatchedChildren validates child elements based on content model match results.
func (r *validationRun) checkMatchedChildren(elem xml.Element, cm *grammar.CompiledContentModel, matches []contentmodel.MatchResult, path string) []errors.Validation {
	var violations []errors.Validation
	children := elem.Children()
	var decls []*grammar.CompiledElement
	if cm != nil {
		if mappedDecls, ok := r.matchChildrenInSimpleSequence(children, cm); ok {
			decls = mappedDecls
		}
	}
	contentModelDecls := r.indexContentModelElements(cm)

	for i, child := range children {
		childPath := appendPath(path, child.LocalName())

		// if content model validation failed, don't validate unmatched children
		if i >= len(matches) {
			continue
		}

		// check if this child was matched by a wildcard
		match := matches[i]
		if match.IsWildcard {
			switch matches[i].ProcessContents {
			case types.Skip:
				continue
			case types.Lax:
				violations = append(violations, r.checkElementWithProcessContents(child, types.Lax, errors.ErrWildcardNotDeclared, childPath)...)
				continue
			case types.Strict:
				violations = append(violations, r.checkElementWithProcessContents(child, types.Strict, errors.ErrWildcardNotDeclared, childPath)...)
				continue
			}
		}

		// regular element validation
		if !match.MatchedQName.IsZero() {
			if contentModelDecls != nil {
				if decl := contentModelDecls[match.MatchedQName]; decl != nil {
					violations = append(violations, r.checkElementWithDecl(child, decl, childPath)...)
					continue
				}
			}
			if decl := r.schema.Element(match.MatchedQName); decl != nil {
				violations = append(violations, r.checkElementWithDecl(child, decl, childPath)...)
				continue
			}
			if decl := r.findBySubstitution(match.MatchedQName); decl != nil {
				violations = append(violations, r.checkElementWithDecl(child, decl, childPath)...)
				continue
			}
		}
		if i < len(decls) && decls[i] != nil {
			violations = append(violations, r.checkElementWithDecl(child, decls[i], childPath)...)
		} else {
			violations = append(violations, r.checkElement(child, childPath)...)
		}
	}

	return violations
}

func (r *validationRun) indexContentModelElements(cm *grammar.CompiledContentModel) map[types.QName]*grammar.CompiledElement {
	if cm == nil {
		return nil
	}
	decls := make(map[types.QName]*grammar.CompiledElement)
	var walk func(particles []*grammar.CompiledParticle)
	walk = func(particles []*grammar.CompiledParticle) {
		for _, particle := range particles {
			if particle == nil {
				continue
			}
			switch particle.Kind {
			case grammar.ParticleElement:
				if particle.Element == nil {
					continue
				}
				qname := r.effectiveElementQName(particle.Element)
				if existing, ok := decls[qname]; ok && existing != particle.Element {
					decls[qname] = nil
					continue
				}
				decls[qname] = particle.Element
			case grammar.ParticleGroup:
				walk(particle.Children)
			}
		}
	}
	walk(cm.Particles)
	return decls
}

func (r *validationRun) matchChildrenInSimpleSequence(children []xml.Element, cm *grammar.CompiledContentModel) ([]*grammar.CompiledElement, bool) {
	if cm == nil || len(children) == 0 {
		return nil, false
	}
	if cm.AllElements != nil {
		return nil, false
	}

	sequenceParticles, ok := r.flattenSequenceParticles(cm.Particles, nil)
	if !ok {
		return nil, false
	}

	decls := make([]*grammar.CompiledElement, len(children))
	childIdx := 0
	for _, particle := range sequenceParticles {
		if particle == nil || particle.Element == nil {
			return nil, false
		}
		expectedQName := r.effectiveElementQName(particle.Element)
		min := particle.MinOccurs
		max := particle.MaxOccurs
		count := 0

		for childIdx < len(children) && (max == types.UnboundedOccurs || count < max) {
			childQName := r.resolveElementQName(children[childIdx])
			if childQName == expectedQName || r.isSubstitutableQName(childQName, expectedQName) {
				decls[childIdx] = r.substitutionDeclForQName(expectedQName, childQName, particle.Element)
				childIdx++
				count++
			} else {
				break
			}
		}

		if count < min {
			return nil, false
		}
	}

	if childIdx != len(children) {
		return nil, false
	}

	return decls, true
}

// flattenSequenceParticles flattens nested sequence particles into a single slice.
// Returns (particles, true) on success, or (nil, false) if the structure is not a simple sequence.
func (r *validationRun) flattenSequenceParticles(particles []*grammar.CompiledParticle, out []*grammar.CompiledParticle) ([]*grammar.CompiledParticle, bool) {
	for _, particle := range particles {
		if particle == nil {
			return nil, false
		}
		switch particle.Kind {
		case grammar.ParticleElement:
			out = append(out, particle)
		case grammar.ParticleGroup:
			if particle.GroupKind != types.Sequence {
				return nil, false
			}
			var ok bool
			out, ok = r.flattenSequenceParticles(particle.Children, out)
			if !ok {
				return nil, false
			}
		case grammar.ParticleWildcard:
			return nil, false
		default:
			return nil, false
		}
	}
	return out, true
}