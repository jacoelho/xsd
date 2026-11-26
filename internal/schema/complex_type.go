package schema

import (
	"errors"
	"fmt"
)

// derivationKind identifies how a complex type derives from its base.
type derivationKind uint8

const (
	// derivationKindNone is used only for xs:anyType.
	derivationKindNone derivationKind = iota
	// derivationKindRestriction records derivation by restriction.
	derivationKindRestriction
	// derivationKindExtension records derivation by extension.
	derivationKindExtension
)

// validDerivationKind reports whether kind is a known complex-type derivation kind.
func validDerivationKind(kind derivationKind) bool {
	switch kind {
	case derivationKindNone, derivationKindRestriction, derivationKindExtension:
		return true
	default:
		return false
	}
}

// complexContentVariety identifies the complex type {content type} variety.
type complexContentVariety uint8

const (
	// contentElementOnly is element-only complex content.
	contentElementOnly complexContentVariety = iota
	// contentMixed is mixed complex content.
	contentMixed
	// contentSimple is simple content.
	contentSimple
	// contentEmpty is empty complex content. It permits neither character nor
	// element information items. The zero value remains contentElementOnly so
	// compiler and test fixtures that use an omitted kind retain their meaning.
	contentEmpty
)

// Mixed reports whether content permits character text mixed with elements.
func (k complexContentVariety) Mixed() bool {
	return k == contentMixed
}

// Simple reports whether content is simple content.
func (k complexContentVariety) Simple() bool {
	return k == contentSimple
}

// validContentKind reports whether kind is a known complex-type content kind.
func validContentKind(kind complexContentVariety) bool {
	switch kind {
	case contentElementOnly, contentMixed, contentSimple, contentEmpty:
		return true
	default:
		return false
	}
}

// complexTypeMetadata stores compiled complex-type runtime metadata.
type complexTypeMetadata struct {
	Name        QName
	Base        TypeID
	Content     contentModelID
	Attrs       attributeUseSetID
	TextType    SimpleTypeID
	ContentKind complexContentVariety
	Abstract    bool
	Derivation  derivationKind
	// ExplicitDerivation is true when Derivation came from an xs:extension or
	// xs:restriction element. Plain complex types are implicit restrictions of
	// xs:anyType and keep local attribute wildcard provenance.
	ExplicitDerivation bool
	Block              DerivationMask
	Final              DerivationMask
	Scope              declarationScope
	// Detached marks a type no instance element reaches
	// (detachUnreachableComponents). Publication keeps its ID but no content
	// model or attribute uses for it.
	Detached bool
}

// complexTypeByID resolves a complex type ID against a complex-type table.
func complexTypeByID(types []complexTypeMetadata, id ComplexTypeID) (*complexTypeMetadata, bool) {
	if !validComplexTypeID(id, len(types)) {
		return nil, false
	}
	return &types[id], true
}

// complexTypeRefLimits are table sizes used to validate complex-type
// references in a frozen runtime schema.
type complexTypeRefLimits struct {
	SimpleTypeCount      int
	ComplexTypeCount     int
	AttributeUseSetCount int
	AnyType              ComplexTypeID
}

// Mixed reports whether the complex type permits mixed character content.
func (ct complexTypeMetadata) Mixed() bool {
	return ct.ContentKind.Mixed()
}

// SimpleContent reports whether the complex type has simple content.
func (ct complexTypeMetadata) SimpleContent() bool {
	return ct.ContentKind.Simple()
}

// validateComplexTypeRuntime validates complex-type metadata that can be
// expressed in runtime vocabulary.
func validateComplexTypeRuntime(
	id ComplexTypeID,
	ct complexTypeMetadata,
	models []contentModelNode,
	limits complexTypeRefLimits,
) error {
	if err := validateComplexTypeShape(ct); err != nil {
		return err
	}
	if err := validateComplexTypeReferences(id, ct, models, limits); err != nil {
		return err
	}
	return validateComplexTypeContent(ct, models, limits.SimpleTypeCount)
}

func validateComplexTypeShape(ct complexTypeMetadata) error {
	if !validContentKind(ct.ContentKind) {
		return newBuildDefect("complex type has invalid content kind")
	}
	if !validDerivationKind(ct.Derivation) {
		return newBuildDefect("complex type has invalid derivation kind")
	}
	if !validComplexBlockMask(ct.Block) {
		return newBuildDefect("complex type block mask contains invalid derivation")
	}
	if !validComplexFinalMask(ct.Final) {
		return newBuildDefect("complex type final mask contains invalid derivation")
	}
	if ct.ExplicitDerivation && ct.Derivation == derivationKindNone {
		return newBuildDefect("complex type marks explicit derivation without derivation kind")
	}
	return nil
}

func validateComplexTypeReferences(id ComplexTypeID, ct complexTypeMetadata, models []contentModelNode, limits complexTypeRefLimits) error {
	if ct.Base == (TypeID{}) {
		if id != limits.AnyType {
			return newBuildDefect("complex type has no base type")
		}
	} else if !validTypeID(ct.Base, limits.SimpleTypeCount, limits.ComplexTypeCount) {
		return newBuildDefect("complex type references invalid base")
	}
	if !validContentModelID(ct.Content, len(models)) {
		return newBuildDefect("complex type references invalid content model")
	}
	if !validAttributeUseSetID(ct.Attrs, limits.AttributeUseSetCount) {
		return newBuildDefect("complex type references invalid attribute use set")
	}
	return nil
}

func validateComplexTypeContent(ct complexTypeMetadata, models []contentModelNode, simpleTypeCount int) error {
	if ct.SimpleContent() {
		return validateSimpleComplexTypeContent(ct, models, simpleTypeCount)
	}
	if ct.TextType != NoSimpleType {
		return newBuildDefect("complex type stores text type without simple content")
	}
	if ct.ContentKind == contentEmpty {
		model := models[ct.Content]
		if model.Kind != modelEmpty {
			return newBuildDefect("empty complex type must reference an empty content model")
		}
	}
	return nil
}

func validateSimpleComplexTypeContent(ct complexTypeMetadata, models []contentModelNode, simpleTypeCount int) error {
	if !validSimpleTypeID(ct.TextType, simpleTypeCount) {
		return newBuildDefect("complex type references invalid text type")
	}
	if models[ct.Content].Kind != modelEmpty {
		return newBuildDefect("complex type simple content must have empty content model")
	}
	return nil
}

type complexTypeGraphState uint8

const (
	complexTypeGraphUnchecked complexTypeGraphState = iota
	complexTypeGraphChecking
	complexTypeGraphChecked
)

func validateComplexTypeGraph(types []complexTypeMetadata) error {
	state := make([]complexTypeGraphState, len(types))
	stack := make([]ComplexTypeID, 0, min(len(types), 1_024))
	for root := range types {
		if state[root] != complexTypeGraphUnchecked {
			continue
		}
		state[root] = complexTypeGraphChecking
		stack = appendDFSFrame(stack, ComplexTypeID(root), len(types))
		if err := walkComplexTypeGraph(types, state, &stack); err != nil {
			return err
		}
	}
	return nil
}

func walkComplexTypeGraph(types []complexTypeMetadata, state []complexTypeGraphState, stack *[]ComplexTypeID) error {
	for len(*stack) != 0 {
		if err := advanceComplexTypeGraph(types, state, stack); err != nil {
			return err
		}
	}
	return nil
}

func advanceComplexTypeGraph(types []complexTypeMetadata, state []complexTypeGraphState, stack *[]ComplexTypeID) error {
	last := len(*stack) - 1
	id := (*stack)[last]
	base := types[id].Base
	switch {
	case base == (TypeID{}), base.IsSimple():
		state[id] = complexTypeGraphChecked
		*stack = (*stack)[:last]
		return nil
	case base.IsComplex():
		baseID, _ := base.Complex()
		return advanceComplexTypeBase(types, state, stack, id, baseID)
	default:
		return newBuildDefect("complex type graph references invalid base")
	}
}

func advanceComplexTypeBase(types []complexTypeMetadata, state []complexTypeGraphState, stack *[]ComplexTypeID, id, baseID ComplexTypeID) error {
	if !validComplexTypeID(baseID, len(types)) {
		return newBuildDefect("complex type graph references invalid base")
	}
	switch state[baseID] {
	case complexTypeGraphUnchecked:
		state[baseID] = complexTypeGraphChecking
		*stack = appendDFSFrame(*stack, baseID, len(types))
		return nil
	case complexTypeGraphChecking:
		return newBuildDefect("complex type graph contains cycle")
	case complexTypeGraphChecked:
		state[id] = complexTypeGraphChecked
		*stack = (*stack)[:len(*stack)-1]
		return nil
	default:
		return newBuildDefect("complex type graph references invalid base")
	}
}

// validateComplexTypeDerivationRuntime validates derivation-mode rules common
// to complex types derived from a complex-type base. Simple-base derivation
// rules are validated by validateComplexTypeSimpleBaseExtensionRuntime.
func validateComplexTypeDerivationRuntime(anyType, id ComplexTypeID, ct complexTypeMetadata) error {
	if id == anyType {
		return nil
	}
	if _, ok := ct.Base.Simple(); ok {
		return nil
	}
	switch ct.Derivation {
	case derivationKindExtension:
		if !ct.ExplicitDerivation {
			return newBuildDefect("complex extension is not marked explicit")
		}
	case derivationKindRestriction:
		if !ct.ExplicitDerivation && ct.Base != ComplexRef(anyType) {
			return newBuildDefect("complex restriction is not marked explicit")
		}
	case derivationKindNone:
		return newBuildDefect("non-anyType complex type has no derivation")
	default:
		return newBuildDefect("complex type has invalid derivation kind")
	}
	return nil
}

// complexTypeDerivationBaseID returns the complex base ID for a derivation
// whose base must be a complex type.
func complexTypeDerivationBaseID(base TypeID, complexTypeCount int) (ComplexTypeID, error) {
	baseID, ok := base.Complex()
	if !ok || !validComplexTypeID(baseID, complexTypeCount) {
		return noComplexType, newBuildDefect("complex type derivation references non-complex base")
	}
	return baseID, nil
}

// validateComplexTypeFinalAllows validates that a complex-type final mask
// admits one complex derivation step.
func validateComplexTypeFinalAllows(final, derivation DerivationMask) error {
	if !validComplexFinalMask(final) {
		return newBuildDefect("complex type final mask contains invalid derivation")
	}
	// DerivationMask is a bitmask; only single complex-derivation bits are valid here.
	//exhaustive:ignore
	switch derivation {
	case DerivationExtension:
		if final&DerivationExtension != 0 {
			return errors.New("complex type extension is blocked by base final")
		}
	case DerivationRestriction:
		if final&DerivationRestriction != 0 {
			return errors.New("complex type restriction is blocked by base final")
		}
	default:
		return newBuildDefect("complex type final derivation is invalid")
	}
	return nil
}

// validateComplexTypeExtensionRuntime validates complex-type extension rules
// that do not depend on attribute-use value-constraint equality.
func validateComplexTypeExtensionRuntime(rt *schemaBuild, base, derived complexTypeMetadata) error {
	if err := validateComplexTypeFinalAllows(base.Final, DerivationExtension); err != nil {
		return err
	}
	if base.SimpleContent() {
		if !derived.SimpleContent() || derived.TextType != base.TextType || !contentModelKind(rt, derived.Content, modelEmpty) {
			return errors.New("complex simple-content extension shape does not match base")
		}
		return nil
	}
	if derived.SimpleContent() {
		return errors.New("complex extension changes element content to simple content")
	}
	if base.Mixed() && !derived.Mixed() {
		return errors.New("complex extension drops mixed base content")
	}
	if !complexContentExtendsBase(rt, base, derived) {
		return errors.New("complex extension content does not preserve base content")
	}
	return nil
}

// Extension after restriction must retain the attribute obligations of the
// ancestor whose base is anyType (cos-ct-extends 1.5). Intermediate restrictions
// can be discarded by the required two-step reconstruction; their uses are not
// retained.
func validateComplexExtensionAttributeAncestry(rt *schemaBuild, derived complexTypeMetadata) error {
	if derived.Derivation != derivationKindExtension {
		return nil
	}
	retained, err := retainedAncestorAttributeUses(rt, derived.Base)
	if err != nil || len(retained) == 0 {
		return err
	}
	return validateRetainedAncestorAttributes(rt, retained, derived.Attrs)
}

// retainedAncestorAttributeUses returns the attribute uses of the ancestor of
// base whose base is anyType when a restriction lies on the path to it, and
// none otherwise.
func retainedAncestorAttributeUses(rt *schemaBuild, base TypeID) ([]attributeUse, error) {
	current := base
	restricted := false
	for range len(rt.ComplexTypes) {
		id, ok := derivedComplexType(rt, current)
		if !ok {
			return nil, nil
		}
		if err := rt.spendTypeDerivationWork(1); err != nil {
			return nil, err
		}
		// Base references are admitted before derivation checks.
		ancestor := &rt.ComplexTypes[id]
		if ancestor.Base != ComplexRef(rt.AnyType) {
			restricted = restricted || ancestor.Derivation == derivationKindRestriction
			current = ancestor.Base
			continue
		}
		if restricted {
			return rt.AttributeUseSets[ancestor.Attrs].Uses, nil
		}
		return nil, nil
	}
	return nil, newBuildDefect("extension ancestry contains a cycle")
}

// derivedComplexType returns the complex type t names unless t is a simple
// type or anyType.
func derivedComplexType(rt *schemaBuild, t TypeID) (ComplexTypeID, bool) {
	id, isComplex := t.Complex()
	return id, isComplex && id != rt.AnyType
}

func validateRetainedAncestorAttributes(rt *schemaBuild, retained []attributeUse, derivedID attributeUseSetID) error {
	derived := rt.AttributeUseSets[derivedID]
	for _, use := range retained {
		if err := rt.spendTypeDerivationWork(1); err != nil {
			return err
		}
		slot, present := derived.Index[use.Name]
		if !present {
			if use.Required {
				return errors.New("extension omits required ancestor attribute")
			}
			continue
		}
		if err := validateAttributeUseRestriction(rt, use, derived.Uses[slot]); err != nil {
			return wrapUnlessLimit(err, fmt.Sprintf("extension attribute %s violates original ancestor", rt.formatName(use.Name)))
		}
	}
	return nil
}

// validateComplexTypeSimpleBaseExtensionRuntime validates complex-type
// extension rules for complex types derived from a simple-type base.
func validateComplexTypeSimpleBaseExtensionRuntime(rt *schemaBuild, base SimpleTypeID, derived complexTypeMetadata) error {
	if derived.Derivation != derivationKindExtension {
		return errors.New("complex type with simple base is not an extension")
	}
	if !derived.ExplicitDerivation {
		return newBuildDefect("complex simple-base extension is not marked explicit")
	}
	baseFinal, ok := rt.SimpleTypeFinal(base)
	if !ok {
		return newBuildDefect("complex type references invalid simple base")
	}
	if err := validateSimpleBaseComplexExtensionFinalAllows(baseFinal); err != nil {
		return err
	}
	if !derived.SimpleContent() || derived.TextType != base || !contentModelKind(rt, derived.Content, modelEmpty) {
		return newBuildDefect("complex type simple-base extension shape is invalid")
	}
	return nil
}

// validateComplexTypeRestrictionRuntime validates complex-type restriction
// rules that do not depend on content-particle restriction traversal or
// attribute-use-set validation.
func validateComplexTypeRestrictionRuntime(
	rt *schemaBuild,
	analysis *contentModelAnalysis,
	base, derived complexTypeMetadata,
	work func(int) error,
) error {
	if err := validateComplexTypeFinalAllows(base.Final, DerivationRestriction); err != nil {
		return err
	}
	if derived.SimpleContent() {
		allowed, err := simpleContentDerivationBaseAllowed(analysis, base, derivationKindRestriction)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("complex simple-content restriction base is not simple or emptiable mixed")
		}
		return validateSimpleContentRestrictionTextType(rt, derived.TextType, base.TextType, work)
	}
	if base.SimpleContent() {
		return errors.New("complex content restriction drops simple base content")
	}
	return nil
}

// validateSimpleContentRestrictionTextType validates that a simple-content
// restriction text type is derived from its simple-content base text type.
func validateSimpleContentRestrictionTextType(
	rt *schemaBuild,
	derived, base SimpleTypeID,
	work func(int) error,
) error {
	if base == NoSimpleType {
		return nil
	}
	_, ok, err := deriveTypeMask(rt, SimpleRef(derived), SimpleRef(base), work)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("simpleContent restriction type is not derived from base")
	}
	return nil
}

// baseIsUrType reports whether the {base type definition} of ct is the
// ur-type itself (Derivation Valid (Restriction, Complex) clauses 5.1 and
// 4.3). A type that shares xs:anyType's content or wildcard, or derives from
// it, is not the ur-type.
func baseIsUrType(rt *schemaBuild, ct complexTypeMetadata) bool {
	return ct.Base == ComplexRef(rt.AnyType)
}

func validateComplexRestrictionAttributeWildcard(rt *schemaBuild, ct complexTypeMetadata) error {
	if ct.Derivation != derivationKindRestriction || baseIsUrType(rt, ct) {
		return nil
	}
	derivedID := rt.AttributeUseSets[ct.Attrs].Wildcard
	if derivedID == NoWildcard {
		return nil
	}
	baseID, err := complexTypeDerivationBaseID(ct.Base, len(rt.ComplexTypes))
	if err != nil {
		return err
	}
	base := rt.AttributeUseSets[rt.ComplexTypes[baseID].Attrs]
	if !validWildcardID(base.Wildcard, len(rt.Wildcards)) {
		return errors.New("attribute wildcard restriction requires base wildcard")
	}
	if !validWildcardID(derivedID, len(rt.Wildcards)) {
		return newBuildDefect("attribute wildcard restriction references invalid wildcard")
	}
	// Read only admitted scalars; the cloning lookup would copy namespaces.
	if rt.Wildcards[derivedID].Process > rt.Wildcards[base.Wildcard].Process {
		return errors.New("attribute wildcard restriction weakens processContents")
	}
	return nil
}

// validateSimpleBaseComplexExtensionFinalAllows validates the final-mask rule
// for a complex type extending a simple type through simpleContent.
func validateSimpleBaseComplexExtensionFinalAllows(final DerivationMask) error {
	if final&DerivationExtension != 0 {
		return errors.New("complex type extension is blocked by simple base final")
	}
	return nil
}

// simpleContentDerivationBaseAllowed reports whether a simpleContent derivation
// may use base. Restrictions may derive simple content from an emptiable mixed
// complex base; extensions require an existing simple-content base.
func simpleContentDerivationBaseAllowed(analysis *contentModelAnalysis, base complexTypeMetadata, derivation derivationKind) (bool, error) {
	if base.SimpleContent() {
		return true, nil
	}
	if derivation != derivationKindRestriction || !base.Mixed() {
		return false, nil
	}
	if analysis == nil {
		return false, newBuildDefect("content model analysis is nil")
	}
	return analysis.ModelEmptiable(base.Content)
}

// complexContentMixedDerivationBaseAllowed reports whether a complexContent
// derivation with mixed=true may derive from base.
func complexContentMixedDerivationBaseAllowed(base complexTypeMetadata, derivation derivationKind) bool {
	if base.Mixed() {
		return true
	}
	return derivation == derivationKindExtension && base.ContentKind == contentEmpty
}

// validateComplexContentMixedDerivationBase validates complexContent mixed
// derivation admission against the compiled base type.
func validateComplexContentMixedDerivationBase(base complexTypeMetadata, derivation derivationKind, content complexContentVariety) error {
	if content.Mixed() && !complexContentMixedDerivationBaseAllowed(base, derivation) {
		return errors.New("complexContent mixed derivation requires mixed base")
	}
	return nil
}

func contentModelKind(rt *schemaBuild, id contentModelID, kind modelKind) bool {
	model, ok := contentModelByID(rt.Models, id)
	return ok && model.Kind == kind
}
