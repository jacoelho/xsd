package semantics

import (
	"errors"
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/value"
)

// collectAllAttributesForValidation collects all attributes from a complex type.
// This includes attributes from extensions, restrictions, and attribute groups.
func collectAllAttributesForValidation(schema *parser.Schema, ct *model.ComplexType) []*model.AttributeDecl {
	var allAttrs []*model.AttributeDecl
	_ = walkComplexTypeLocalAttributes(schema, ct, AttributeGroupWalkOptions{
		Missing: MissingIgnore,
		Cycles:  CyclePolicyIgnore,
	}, func(attr *model.AttributeDecl, _ bool) error {
		allAttrs = append(allAttrs, attr)
		return nil
	})
	return allAttrs
}

func collectEffectiveAttributeUses(schema *parser.Schema, ct *model.ComplexType) map[model.QName]*model.AttributeDecl {
	if ct == nil {
		return nil
	}
	attrMap := make(map[model.QName]*model.AttributeDecl)
	_ = walkComplexTypeAttributeChain(schema, ct, AttributeGroupWalkOptions{
		Missing: MissingIgnore,
		Cycles:  CyclePolicyIgnore,
	}, func(_ *model.ComplexType, attr *model.AttributeDecl, _ bool) error {
		mergeValidationAttribute(schema, attr, attrMap)
		return nil
	})
	return attrMap
}

func mergeAttributesFromTypeForValidation(schema *parser.Schema, ct *model.ComplexType, attrMap map[model.QName]*model.AttributeDecl) {
	_ = walkComplexTypeLocalAttributes(schema, ct, AttributeGroupWalkOptions{
		Missing: MissingIgnore,
		Cycles:  CyclePolicyIgnore,
	}, func(attr *model.AttributeDecl, _ bool) error {
		mergeValidationAttribute(schema, attr, attrMap)
		return nil
	})
}

func mergeAttributesFromGroupsForValidation(schema *parser.Schema, agRefs []model.QName, attrMap map[model.QName]*model.AttributeDecl) {
	_ = walkAttributeGroupAttributes(schema, agRefs, AttributeGroupWalkOptions{
		Missing: MissingIgnore,
		Cycles:  CyclePolicyIgnore,
	}, func(attr *model.AttributeDecl, _ bool) error {
		mergeValidationAttribute(schema, attr, attrMap)
		return nil
	})
}

func mergeValidationAttribute(schema *parser.Schema, attr *model.AttributeDecl, attrMap map[model.QName]*model.AttributeDecl) {
	if attr == nil {
		return
	}
	key := parser.EffectiveAttributeQName(schema, attr)
	if attr.Use == model.Prohibited && !attr.HasFixed {
		delete(attrMap, key)
		return
	}
	attrMap[key] = attr
}

// collectAttributesFromGroups collects attributes from attribute group references.
func collectAttributesFromGroups(schema *parser.Schema, agRefs []model.QName) []*model.AttributeDecl {
	var result []*model.AttributeDecl
	_ = walkAttributeGroupAttributes(schema, agRefs, AttributeGroupWalkOptions{
		Missing: MissingIgnore,
		Cycles:  CyclePolicyIgnore,
	}, func(attr *model.AttributeDecl, _ bool) error {
		result = append(result, attr)
		return nil
	})
	return result
}

// CollectAttributeUses resolves effective attribute uses and wildcard.
// It follows complex-type derivation from base to leaf in deterministic order.
func CollectAttributeUses(schema *parser.Schema, ct *model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, error) {
	if schema == nil || ct == nil {
		return nil, nil, nil
	}
	attrMap := make(map[model.QName]*model.AttributeDecl)
	chain := CollectComplexTypeChain(schema, ct, ComplexTypeChainExplicitBaseOnly)
	var wildcard *model.AnyAttribute
	for i := len(chain) - 1; i >= 0; i-- {
		current := chain[i]
		if err := mergeEffectiveAttributeUsesFromType(schema, current, attrMap); err != nil {
			return nil, nil, err
		}
		localWildcard, err := localAttributeWildcard(schema, current)
		if err != nil {
			return nil, nil, err
		}
		if i == len(chain)-1 {
			wildcard = localWildcard
		} else {
			wildcard, err = applyDerivedWildcard(wildcard, localWildcard, current)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	out := make([]*model.AttributeDecl, 0, len(attrMap))
	for _, decl := range attrMap {
		out = append(out, decl)
	}
	slices.SortFunc(out, func(a, b *model.AttributeDecl) int {
		left := parser.EffectiveAttributeQName(schema, a)
		right := parser.EffectiveAttributeQName(schema, b)
		return model.Compare(left, right)
	})
	return out, wildcard, nil
}

func mergeEffectiveAttributeUsesFromType(schema *parser.Schema, ct *model.ComplexType, attrMap map[model.QName]*model.AttributeDecl) error {
	return walkComplexTypeLocalAttributes(schema, ct, AttributeGroupWalkOptions{
		Missing: MissingError,
		Cycles:  CyclePolicyIgnore,
	}, func(attr *model.AttributeDecl, fromGroup bool) error {
		if fromGroup && attr.Use == model.Prohibited {
			return nil
		}
		key := parser.EffectiveAttributeQName(schema, attr)
		attrMap[key] = attr
		return nil
	})
}

func localAttributeWildcard(schema *parser.Schema, ct *model.ComplexType) (*model.AnyAttribute, error) {
	wildcard, err := CollectComplexTypeWildcard(schema, ct, AttributeGroupCollectOptions{
		Missing:      MissingError,
		Cycles:       CyclePolicyIgnore,
		EmptyIsError: true,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrAttributeWildcardIntersectionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard intersection not expressible")
		case errors.Is(err, ErrAttributeWildcardIntersectionEmpty):
			return nil, nil
		default:
			return nil, err
		}
	}
	return wildcard, nil
}

func applyDerivedWildcard(base, local *model.AnyAttribute, ct *model.ComplexType) (*model.AnyAttribute, error) {
	method := model.DerivationRestriction
	if ct != nil {
		if ct.DerivationMethod != 0 {
			method = ct.DerivationMethod
		} else if content := ct.Content(); content != nil {
			switch {
			case content.ExtensionDef() != nil:
				method = model.DerivationExtension
			case content.RestrictionDef() != nil:
				method = model.DerivationRestriction
			}
		}
	}
	out, err := ApplyAttributeWildcardDerivation(base, local, method)
	if err != nil {
		switch {
		case errors.Is(err, ErrAttributeWildcardUnionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard union not expressible")
		case errors.Is(err, ErrAttributeWildcardRestrictionAddsWildcard):
			return nil, fmt.Errorf("attribute wildcard restriction adds wildcard")
		case errors.Is(err, ErrAttributeWildcardRestrictionNotExpressible):
			return nil, fmt.Errorf("attribute wildcard restriction not expressible")
		case errors.Is(err, ErrAttributeWildcardRestrictionEmpty):
			return nil, fmt.Errorf("attribute wildcard restriction empty")
		default:
			return nil, err
		}
	}
	return out, nil
}

type attributeVisitFunc func(attr *model.AttributeDecl, fromGroup bool) error

func walkComplexTypeLocalAttributes(
	schema *parser.Schema,
	ct *model.ComplexType,
	opts AttributeGroupWalkOptions,
	visit attributeVisitFunc,
) error {
	if ct == nil || visit == nil {
		return nil
	}
	if err := walkAttributeDecls(ct.Attributes(), false, visit); err != nil {
		return err
	}
	if err := walkAttributeGroupAttributes(schema, ct.AttrGroups, opts, visit); err != nil {
		return err
	}

	content := ct.Content()
	if content == nil {
		return nil
	}
	if ext := content.ExtensionDef(); ext != nil {
		if err := walkAttributeDecls(ext.Attributes, false, visit); err != nil {
			return err
		}
		if err := walkAttributeGroupAttributes(schema, ext.AttrGroups, opts, visit); err != nil {
			return err
		}
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := walkAttributeDecls(restr.Attributes, false, visit); err != nil {
			return err
		}
		if err := walkAttributeGroupAttributes(schema, restr.AttrGroups, opts, visit); err != nil {
			return err
		}
	}
	return nil
}

func walkComplexTypeAttributeChain(
	schema *parser.Schema,
	ct *model.ComplexType,
	opts AttributeGroupWalkOptions,
	visit func(current *model.ComplexType, attr *model.AttributeDecl, fromGroup bool) error,
) error {
	if ct == nil || visit == nil {
		return nil
	}
	chain := CollectComplexTypeChain(schema, ct, ComplexTypeChainExplicitBaseOnly)
	for i := len(chain) - 1; i >= 0; i-- {
		current := chain[i]
		if err := walkComplexTypeLocalAttributes(schema, current, opts, func(attr *model.AttributeDecl, fromGroup bool) error {
			return visit(current, attr, fromGroup)
		}); err != nil {
			return err
		}
	}
	return nil
}

func walkAttributeGroupAttributes(
	schema *parser.Schema,
	refs []model.QName,
	opts AttributeGroupWalkOptions,
	visit attributeVisitFunc,
) error {
	if visit == nil {
		return nil
	}
	ctx := NewAttributeGroupContext(schema, opts)
	return ctx.Walk(refs, func(_ model.QName, group *model.AttributeGroup) error {
		if group == nil {
			return nil
		}
		return walkAttributeDecls(group.Attributes, true, visit)
	})
}

func walkAttributeDecls(attrs []*model.AttributeDecl, fromGroup bool, visit attributeVisitFunc) error {
	for _, attr := range attrs {
		if attr == nil {
			continue
		}
		if err := visit(attr, fromGroup); err != nil {
			return err
		}
	}
	return nil
}

// validateAttributeDeclStructure validates structural constraints of an attribute declaration.
// Does not validate references, which might be forward references or imports.
func validateAttributeDeclStructure(schemaDef *parser.Schema, attrQName model.QName, decl *model.AttributeDecl) error {
	if !model.IsValidNCName(attrQName.Local) {
		return fmt.Errorf("invalid attribute name '%s': must be a valid NCName", attrQName.Local)
	}
	if attrQName.Local == "xmlns" {
		return fmt.Errorf("invalid attribute name '%s': reserved XMLNS name", attrQName.Local)
	}
	effectiveNamespace := attrQName.Namespace
	if !decl.IsReference {
		switch decl.Form {
		case model.FormQualified:
			effectiveNamespace = schemaDef.TargetNamespace
		case model.FormUnqualified:
			effectiveNamespace = ""
		default:
			if schemaDef.AttributeFormDefault == parser.Qualified {
				effectiveNamespace = schemaDef.TargetNamespace
			}
		}
	}
	if effectiveNamespace == value.XSINamespace {
		return fmt.Errorf("invalid attribute name '%s': attributes in the xsi namespace are not allowed", attrQName.Local)
	}

	if st, ok := decl.Type.(*model.SimpleType); ok {
		if err := validateSimpleTypeStructure(schemaDef, st); err != nil {
			return fmt.Errorf("inline simpleType: %w", err)
		}
	}

	if decl.HasDefault && decl.HasFixed {
		return fmt.Errorf("attribute cannot have both 'default' and 'fixed' values")
	}
	if decl.Use == model.Required && decl.HasDefault {
		return fmt.Errorf("attribute with use='required' cannot have a default value")
	}
	if decl.Use == model.Prohibited && decl.HasDefault {
		return fmt.Errorf("attribute with use='prohibited' cannot have a default value")
	}
	if decl.HasDefault {
		if err := validateDefaultOrFixedValueWithContext(schemaDef, decl.Default, decl.Type, decl.DefaultContext); err != nil {
			return fmt.Errorf("invalid default value '%s': %w", decl.Default, err)
		}
	}
	if decl.HasFixed {
		if err := validateDefaultOrFixedValueWithContext(schemaDef, decl.Fixed, decl.Type, decl.FixedContext); err != nil {
			return fmt.Errorf("invalid fixed value '%s': %w", decl.Fixed, err)
		}
	}

	return nil
}

// validateAttributeGroupStructure validates structural constraints of an attribute group.
func validateAttributeGroupStructure(schema *parser.Schema, groupQName model.QName, ag *model.AttributeGroup) error {
	if !model.IsValidNCName(groupQName.Local) {
		return fmt.Errorf("invalid attributeGroup name '%s': must be a valid NCName", groupQName.Local)
	}
	for _, attr := range ag.Attributes {
		if err := validateAttributeDeclStructure(schema, attr.Name, attr); err != nil {
			return fmt.Errorf("attribute: %w", err)
		}
	}
	return validateAttributeGroupUniqueness(schema, ag)
}

// validateAttributeUniqueness validates that no two attributes in a complex type
// share the same name and namespace.
func validateAttributeUniqueness(schema *parser.Schema, ct *model.ComplexType) error {
	allAttributes := collectAllAttributesForValidation(schema, ct)

	seen := make(map[model.QName]bool)
	for _, attr := range allAttributes {
		key := parser.EffectiveAttributeQName(schema, attr)
		if seen[key] {
			return fmt.Errorf("duplicate attribute '%s' in namespace '%s'", attr.Name.Local, attr.Name.Namespace)
		}
		seen[key] = true
	}

	return nil
}

func validateExtensionAttributeUniqueness(schema *parser.Schema, ct *model.ComplexType) error {
	if ct == nil {
		return nil
	}
	content := ct.Content()
	if content == nil {
		return nil
	}
	ext := content.ExtensionDef()
	if ext == nil || ext.Base.IsZero() {
		return nil
	}
	baseCT, ok := LookupComplexType(schema, ext.Base)
	if !ok || baseCT == nil {
		return nil
	}

	baseAttrs := collectEffectiveAttributeUses(schema, baseCT)
	if len(baseAttrs) == 0 {
		return nil
	}

	attrs := slices.Clone(ext.Attributes)
	attrs = append(attrs, collectAttributesFromGroups(schema, ext.AttrGroups)...)
	for _, attr := range attrs {
		key := parser.EffectiveAttributeQName(schema, attr)
		if _, exists := baseAttrs[key]; exists {
			return fmt.Errorf("extension attribute '%s' in namespace '%s' duplicates base attribute", attr.Name.Local, attr.Name.Namespace)
		}
	}
	return nil
}

// validateAttributeGroupUniqueness validates that no two attributes in the group
// share the same name and namespace.
func validateAttributeGroupUniqueness(schema *parser.Schema, ag *model.AttributeGroup) error {
	seen := make(map[model.QName]bool)
	for _, attr := range ag.Attributes {
		key := parser.EffectiveAttributeQName(schema, attr)
		if seen[key] {
			return fmt.Errorf("duplicate attribute '%s' in namespace '%s'", attr.Name.Local, attr.Name.Namespace)
		}
		seen[key] = true
	}
	return nil
}

// validateNoCyclicAttributeGroups detects cycles between attribute group definitions.
func validateNoCyclicAttributeGroups(sch *parser.Schema) error {
	ctx := NewAttributeGroupContext(sch, AttributeGroupWalkOptions{
		Missing: MissingIgnore,
		Cycles:  CyclePolicyError,
	})

	for _, qname := range model.SortedMapKeys(sch.AttributeGroups) {
		if err := ctx.Walk([]model.QName{qname}, nil); err != nil {
			var cycleErr AttributeGroupCycleError
			if errors.As(err, &cycleErr) {
				return CycleError[model.QName]{Key: cycleErr.QName}
			}
			return err
		}
	}
	return nil
}

// validateAttributeReference validates that an attribute reference exists.
// An attribute with Type=nil could be:
// 1. An attribute reference from ref="..." and must exist in schema.AttributeDecls.
// 2. A local attribute declaration from name="..." and does not need to exist.
func validateAttributeReference(sch *parser.Schema, contextQName model.QName, attr *model.AttributeDecl, contextType string) error {
	if !shouldValidateAttributeReference(attr) {
		return nil
	}

	target, err := lookupReferencedAttributeDecl(sch, attr, contextQName, contextType)
	if err != nil {
		return err
	}
	return validateAttributeReferenceValueCompatibility(sch, attr, target, contextQName, contextType)
}

func shouldValidateAttributeReference(attr *model.AttributeDecl) bool {
	if !attr.IsReference {
		return false
	}
	return !isBuiltinXMLAttribute(attr)
}

func lookupReferencedAttributeDecl(sch *parser.Schema, attr *model.AttributeDecl, contextQName model.QName, contextType string) (*model.AttributeDecl, error) {
	target, exists := sch.AttributeDecls[attr.Name]
	if !exists {
		return nil, fmt.Errorf("%s %s: attribute reference %s does not exist", contextType, contextQName, attr.Name)
	}
	return target, nil
}

func validateAttributeReferenceValueCompatibility(
	sch *parser.Schema,
	attr *model.AttributeDecl,
	target *model.AttributeDecl,
	contextQName model.QName,
	contextType string,
) error {
	if attr.HasDefault && target.HasFixed {
		return fmt.Errorf("%s %s: attribute reference '%s' cannot specify a default when declaration is fixed",
			contextType, contextQName, attr.Name)
	}

	if !attr.HasFixed || !target.HasFixed {
		return nil
	}
	match, err := fixedValuesEqual(sch, attr, target)
	if err != nil {
		return fmt.Errorf("%s %s: attribute reference '%s' fixed value comparison failed: %w",
			contextType, contextQName, attr.Name, err)
	}
	if !match {
		return fmt.Errorf("%s %s: attribute reference '%s' fixed value '%s' conflicts with declaration fixed value '%s'",
			contextType, contextQName, attr.Name, attr.Fixed, target.Fixed)
	}
	return nil
}

// isBuiltinXMLAttribute checks if an attribute is a built-in XML namespace attribute.
func isBuiltinXMLAttribute(attr *model.AttributeDecl) bool {
	return attr.Name.Namespace == value.XMLNamespace
}

// validateAttributeGroupReference validates that an attribute group reference exists.
func validateAttributeGroupReference(sch *parser.Schema, agRef, contextQName model.QName) error {
	if attributeGroupReferenceExists(sch, agRef) {
		return nil
	}
	return fmt.Errorf("type %s: attributeGroup reference %s does not exist", contextQName, agRef)
}

func attributeGroupReferenceExists(sch *parser.Schema, agRef model.QName) bool {
	if _, exists := sch.AttributeGroups[agRef]; exists {
		return true
	}
	if agRef.Namespace != sch.TargetNamespace || sch.TargetNamespace == "" {
		return false
	}

	noNSRef := model.QName{
		Namespace: "",
		Local:     agRef.Local,
	}
	_, exists := sch.AttributeGroups[noNSRef]
	return exists
}

func validateAttributeValueConstraintsForType(sch *parser.Schema, typ model.Type) error {
	ct, ok := typ.(*model.ComplexType)
	if !ok {
		return nil
	}
	validateAttrs := func(attrs []*model.AttributeDecl) error {
		for _, attr := range attrs {
			if err := validateAttributeValueConstraints(sch, attr); err != nil {
				return fmt.Errorf("attribute %s: %w", attr.Name, err)
			}
		}
		return nil
	}
	if err := validateAttrs(ct.Attributes()); err != nil {
		return err
	}
	if ext := ct.Content().ExtensionDef(); ext != nil {
		if err := validateAttrs(ext.Attributes); err != nil {
			return err
		}
	}
	if restr := ct.Content().RestrictionDef(); restr != nil {
		if err := validateAttrs(restr.Attributes); err != nil {
			return err
		}
	}
	return nil
}

func validateAttributeValueConstraints(sch *parser.Schema, decl *model.AttributeDecl) error {
	resolvedType := parser.ResolveTypeReferenceAllowMissing(sch, decl.Type)
	if _, ok := resolvedType.(*model.ComplexType); ok {
		return fmt.Errorf("type must be a simple type")
	}
	if isDirectNotationType(resolvedType) {
		return fmt.Errorf("attribute cannot use NOTATION type")
	}
	if decl.HasDefault {
		if err := validateDefaultOrFixedValueResolved(sch, decl.Default, resolvedType, decl.DefaultContext, idValuesDisallowed); err != nil {
			return fmt.Errorf("invalid default value '%s': %w", decl.Default, err)
		}
	}
	if decl.HasFixed {
		if err := validateDefaultOrFixedValueResolved(sch, decl.Fixed, resolvedType, decl.FixedContext, idValuesDisallowed); err != nil {
			return fmt.Errorf("invalid fixed value '%s': %w", decl.Fixed, err)
		}
	}
	return nil
}
