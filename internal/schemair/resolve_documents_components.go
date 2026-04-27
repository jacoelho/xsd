package schemair

import (
	"fmt"
	"slices"

	ast "github.com/jacoelho/xsd/internal/schemaast"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdpath"
)

func (r *docResolver) ensureElement(decl *ast.ElementDecl, global bool) (ElementID, error) {
	if decl == nil {
		return 0, nil
	}
	if !decl.Ref.IsZero() {
		return r.elementID(decl.Ref)
	}
	handle := r.elementDeclHandle(decl)
	id, ok := r.ids.elements[handle]
	if !ok {
		return 0, fmt.Errorf("schema ir: element %s missing ID", formatName(nameFromQName(decl.Name)))
	}
	if r.emittedElems[id] {
		return id, nil
	}
	if err := validateDefaultFixed("element", nameFromQName(decl.Name), decl.Default, decl.Fixed); err != nil {
		return 0, err
	}
	r.emittedElems[id] = true
	head, err := r.elementID(decl.SubstitutionGroup)
	if err != nil {
		return 0, err
	}
	if err := r.validateIdentityNames(nameFromQName(decl.Name), decl.Identity); err != nil {
		return 0, err
	}
	typeRef, err := r.typeUseRef(decl.Type, true)
	if err != nil {
		return 0, err
	}
	if typeUseIsZero(decl.Type) && head != 0 {
		if headElem, ok := r.emittedElement(head); ok {
			typeRef = headElem.TypeDecl
		}
	}
	if err := r.validateValueConstraintType("element", nameFromQName(decl.Name), typeRef, decl.Default, decl.Fixed); err != nil {
		return 0, err
	}
	if head != 0 {
		if err := r.validateSubstitutionFinal(nameFromQName(decl.Name), typeRef, head); err != nil {
			return 0, err
		}
	}
	elem := Element{
		ID:               id,
		Name:             nameFromQName(decl.Name),
		TypeDecl:         typeRef,
		SubstitutionHead: head,
		Default:          r.valueConstraint(ValueConstraintDefault, decl.Default),
		Fixed:            r.valueConstraint(ValueConstraintFixed, decl.Fixed),
		Final:            derivationSet(decl.Final),
		Block:            elementBlock(decl.Block),
		Nillable:         decl.Nillable,
		Abstract:         decl.Abstract,
		Global:           global,
		Origin:           decl.Origin,
	}
	r.out.Elements = append(r.out.Elements, elem)
	for _, identity := range decl.Identity {
		constraint, err := r.identity(id, identity)
		if err != nil {
			return 0, err
		}
		r.out.IdentityConstraints = append(r.out.IdentityConstraints, constraint)
	}
	return id, nil
}

func typeUseIsZero(use ast.TypeUse) bool {
	return use.Name.IsZero() && use.Simple == nil && use.Complex == nil
}

func (r *docResolver) validateSubstitutionFinal(memberName Name, memberType TypeRef, headID ElementID) error {
	if headID == 0 {
		return nil
	}
	head, ok := r.emittedElement(headID)
	if !ok {
		return nil
	}
	if sameTypeRef(memberType, head.TypeDecl) || r.headTypeAllowsSubstitution(head.TypeDecl, memberType) {
		return nil
	}
	if ok, err := r.typeRestrictsUnionMember(memberType, head.TypeDecl); err != nil {
		return err
	} else if ok {
		return nil
	}
	mask, ok, err := r.derivationMask(memberType, head.TypeDecl)
	if err != nil {
		return fmt.Errorf("schema ir: resolve substitution group derivation for %s: %w", formatName(memberName), err)
	}
	if !ok {
		return fmt.Errorf("schema ir: element %s: type '%s' is not derived from substitution group head type '%s'",
			formatName(memberName), formatName(memberType.TypeName()), formatName(head.TypeDecl.TypeName()))
	}
	if head.Final == 0 {
		return nil
	}
	for _, method := range []Derivation{DerivationExtension, DerivationRestriction, DerivationList, DerivationUnion} {
		if mask&method != 0 && head.Final&method != 0 {
			return fmt.Errorf("schema ir: element %s cannot substitute for %s: head element is final for %s",
				formatName(memberName), formatName(head.Name), derivationLabel(method))
		}
	}
	return nil
}

func (r *docResolver) headTypeAllowsSubstitution(head, member TypeRef) bool {
	if head.IsBuiltin() && head.TypeName().Local == "anyType" {
		return true
	}
	if !head.IsBuiltin() || head.TypeName().Local != "anySimpleType" {
		return false
	}
	info, ok, err := r.typeInfoForRef(member)
	if err != nil || !ok {
		return false
	}
	return info.Kind != TypeComplex
}

func (r *docResolver) validateValueConstraintType(
	kind string,
	name Name,
	typ TypeRef,
	def ast.ValueConstraintDecl,
	fixed ast.ValueConstraintDecl,
) error {
	if !def.Present && !fixed.Present {
		return nil
	}
	if kind == "element" {
		ok, err := r.elementTypeAllowsValueConstraint(typ)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("schema ir: element %s cannot have default or fixed value because its type has element-only content", formatName(name))
		}
	}
	if err := r.validateValueConstraintLexical(kind, name, typ, def, fixed); err != nil {
		return err
	}
	isID, err := r.isIDType(typ, make(map[TypeRef]bool))
	if err != nil {
		return err
	}
	if !isID {
		return nil
	}
	if fixed.Present {
		return fmt.Errorf("schema ir: %s %s invalid fixed value %q: ID types cannot have default or fixed values",
			kind, formatName(name), fixed.Lexical)
	}
	return fmt.Errorf("schema ir: %s %s invalid default value %q: ID types cannot have default or fixed values",
		kind, formatName(name), def.Lexical)
}

func (r *docResolver) elementTypeAllowsValueConstraint(ref TypeRef) (bool, error) {
	info, ok, err := r.typeInfoForRef(ref)
	if err != nil || !ok {
		return ok, err
	}
	if info.Kind == TypeSimple {
		return true, nil
	}
	if ref.IsBuiltin() {
		return ref.TypeName().Local != "anyType", nil
	}
	plan, ok, err := r.ensureBaseComplexPlan(ref)
	if err != nil || !ok {
		return ok, err
	}
	return plan.Mixed || plan.Content == ContentSimple || !plan.TextType.IsZero() || !isZeroSimpleTypeSpec(plan.TextSpec), nil
}

func (r *docResolver) validateValueConstraintLexical(
	kind string,
	name Name,
	ref TypeRef,
	def ast.ValueConstraintDecl,
	fixed ast.ValueConstraintDecl,
) error {
	spec, ok, err := r.valueConstraintSpecForType(ref)
	if err != nil || !ok {
		return err
	}
	if def.Present {
		if err := r.validateSpecLexicalValue(spec, def.Lexical, r.contextMap(def.NamespaceContextID), make(map[TypeRef]bool)); err != nil {
			return fmt.Errorf("schema ir: %s %s invalid default value %q: %w", kind, formatName(name), def.Lexical, err)
		}
	}
	if fixed.Present {
		if err := r.validateSpecLexicalValue(spec, fixed.Lexical, r.contextMap(fixed.NamespaceContextID), make(map[TypeRef]bool)); err != nil {
			return fmt.Errorf("schema ir: %s %s invalid fixed value %q: %w", kind, formatName(name), fixed.Lexical, err)
		}
	}
	return nil
}

func (r *docResolver) valueConstraintSpecForType(ref TypeRef) (SimpleTypeSpec, bool, error) {
	if spec, ok := r.specForRef(ref); ok {
		return spec, true, nil
	}
	if ref.IsBuiltin() {
		return SimpleTypeSpec{}, false, nil
	}
	info, ok, err := r.typeInfoForRef(ref)
	if err != nil || !ok || info.Kind != TypeComplex {
		return SimpleTypeSpec{}, ok, err
	}
	plan, ok, err := r.ensureBaseComplexPlan(ref)
	if err != nil || !ok {
		return SimpleTypeSpec{}, ok, err
	}
	if !isZeroSimpleTypeSpec(plan.TextSpec) {
		return plan.TextSpec, true, nil
	}
	return r.simpleContentBaseSpec(plan.TextType, SimpleTypeSpec{})
}

func (r *docResolver) validateIdentityNames(element Name, identities []ast.IdentityDecl) error {
	seen := make(map[Name]struct{}, len(identities))
	for _, identity := range identities {
		name := nameFromQName(identity.Name)
		if _, ok := seen[name]; ok {
			return fmt.Errorf("schema ir: element %s duplicate identity constraint name %q", formatName(element), name.Local)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func (r *docResolver) emittedElement(id ElementID) (Element, bool) {
	for _, elem := range r.out.Elements {
		if elem.ID == id {
			return elem, true
		}
	}
	return Element{}, false
}

func (r *docResolver) derivationMask(member, base TypeRef) (Derivation, bool, error) {
	var mask Derivation
	seen := make(map[TypeRef]bool)
	current := member
	for !current.IsZero() {
		if sameTypeRef(current, base) {
			return mask, true, nil
		}
		if seen[current] {
			return 0, false, fmt.Errorf("type derivation cycle at %s", formatName(current.TypeName()))
		}
		seen[current] = true
		info, ok, err := r.typeInfoForRef(current)
		if err != nil || !ok {
			return 0, false, err
		}
		if info.Base.IsZero() {
			return 0, false, nil
		}
		mask |= info.Derivation
		current = info.Base
	}
	return 0, false, nil
}

func (r *docResolver) typeInfoForRef(ref TypeRef) (TypeDecl, bool, error) {
	if ref.IsBuiltin() {
		for _, builtin := range r.out.BuiltinTypes {
			if builtin.Name == ref.TypeName() {
				return TypeDecl{ID: 0, Name: builtin.Name, Kind: TypeSimple, Base: builtin.Base, Derivation: DerivationRestriction}, true, nil
			}
		}
		return TypeDecl{}, false, nil
	}
	refID := ref.TypeID()
	if r.emittingTypes[refID] {
		for _, typ := range r.out.Types {
			if typ.ID == refID {
				return typ, true, nil
			}
		}
		return TypeDecl{}, false, nil
	}
	if decl := r.simpleDecls[r.ids.simpleByID[refID]]; decl != nil {
		if _, err := r.ensureSimpleType(decl, !decl.Name.IsZero()); err != nil {
			return TypeDecl{}, false, err
		}
	}
	if decl := r.complexDecls[r.ids.complexByID[refID]]; decl != nil {
		if _, err := r.ensureComplexType(decl, !decl.Name.IsZero()); err != nil {
			return TypeDecl{}, false, err
		}
	}
	for _, typ := range r.out.Types {
		if typ.ID == refID {
			return typ, true, nil
		}
	}
	return TypeDecl{}, false, nil
}

func sameTypeRef(a, b TypeRef) bool {
	if a.IsBuiltin() != b.IsBuiltin() {
		return false
	}
	if a.IsBuiltin() {
		return a.TypeName() == b.TypeName()
	}
	return a.TypeID() == b.TypeID() && a.TypeID() != 0
}

func derivationLabel(method Derivation) string {
	switch method {
	case DerivationExtension:
		return "extension"
	case DerivationRestriction:
		return "restriction"
	case DerivationList:
		return "list"
	case DerivationUnion:
		return "union"
	default:
		return "unknown"
	}
}

func (r *docResolver) ensureAttribute(decl *ast.AttributeDecl, global bool) (AttributeID, error) {
	if decl == nil {
		return 0, nil
	}
	if !decl.Ref.IsZero() {
		return r.attributeID(decl.Ref)
	}
	if err := validateDocumentAttributeName(decl.Name); err != nil {
		return 0, err
	}
	handle := r.attributeDeclHandle(decl)
	id, ok := r.ids.attributes[handle]
	if !ok {
		return 0, fmt.Errorf("schema ir: attribute %s missing ID", formatName(documentAttributeDeclName(decl, global)))
	}
	if r.emittedAttrs[id] {
		return id, nil
	}
	if err := validateDefaultFixed("attribute", nameFromQName(decl.Name), decl.Default, decl.Fixed); err != nil {
		return 0, err
	}
	r.emittedAttrs[id] = true
	typeRef, err := r.typeUseRef(decl.Type, false)
	if err != nil {
		return 0, err
	}
	if err := r.validateValueConstraintType("attribute", nameFromQName(decl.Name), typeRef, decl.Default, decl.Fixed); err != nil {
		return 0, err
	}
	attr := Attribute{
		ID:       id,
		Name:     documentAttributeDeclName(decl, global),
		TypeDecl: typeRef,
		Default:  r.valueConstraint(ValueConstraintDefault, decl.Default),
		Fixed:    r.valueConstraint(ValueConstraintFixed, decl.Fixed),
		Global:   global,
		Origin:   decl.Origin,
	}
	r.out.Attributes = append(r.out.Attributes, attr)
	return id, nil
}

func documentAttributeDeclName(decl *ast.AttributeDecl, global bool) Name {
	if decl == nil {
		return Name{}
	}
	if global {
		return nameFromQName(decl.Name)
	}
	return Name{Local: decl.Name.Local}
}

func validateDocumentAttributeName(name ast.QName) error {
	if !ast.IsValidNCName(name.Local) {
		return fmt.Errorf("invalid attribute name '%s': must be a valid NCName", name.Local)
	}
	if name.Local == "xmlns" {
		return fmt.Errorf("invalid attribute name '%s': reserved XMLNS name", name.Local)
	}
	if name.Namespace == value.XSINamespace {
		return fmt.Errorf("invalid attribute name '%s': attributes in the xsi namespace are not allowed", name.Local)
	}
	return nil
}

func (r *docResolver) typeUseRef(use ast.TypeUse, allowComplex bool) (TypeRef, error) {
	if !use.Name.IsZero() {
		ref, err := r.typeRef(use.Name)
		if err != nil {
			return NoTypeRef(), err
		}
		if !allowComplex {
			if err := r.requireSimpleTypeRef(ref, fmt.Sprintf("type %s", formatName(nameFromQName(use.Name)))); err != nil {
				return NoTypeRef(), err
			}
		}
		return ref, nil
	}
	if use.Simple != nil {
		id, err := r.ensureSimpleType(use.Simple, false)
		if err != nil {
			return NoTypeRef(), err
		}
		return UserTypeRef(id, nameFromQName(use.Simple.Name)), nil
	}
	if !allowComplex && use.Complex != nil {
		return NoTypeRef(), fmt.Errorf("schema ir: inline complex type is not allowed where a simple type is required")
	}
	if allowComplex && use.Complex != nil {
		id, err := r.ensureComplexType(use.Complex, false)
		if err != nil {
			return NoTypeRef(), err
		}
		return UserTypeRef(id, nameFromQName(use.Complex.Name)), nil
	}
	if allowComplex {
		return r.builtinRef("anyType"), nil
	}
	return r.builtinRef("anySimpleType"), nil
}

func (r *docResolver) requireSimpleTypeRef(ref TypeRef, context string) error {
	if ref.IsBuiltin() && ref.TypeName().Local == "anyType" {
		return fmt.Errorf("schema ir: %s must reference a simple type", context)
	}
	info, ok, err := r.typeInfoForRef(ref)
	if err != nil {
		return err
	}
	if ok && info.Kind != TypeSimple {
		return fmt.Errorf("schema ir: %s must reference a simple type", context)
	}
	return nil
}

func (r *docResolver) addParticle(decl *ast.ParticleDecl, stack []Name) (ParticleID, error) {
	if decl == nil {
		return 0, nil
	}
	if err := validateDocumentParticleOccurs(decl); err != nil {
		return 0, err
	}
	id := ParticleID(len(r.out.Particles) + 1)
	r.out.Particles = append(r.out.Particles, NoParticle(id))
	idx := len(r.out.Particles) - 1
	minOccurs := occurs(decl.Min)
	maxOccurs := occurs(decl.Max)

	switch decl.Kind {
	case ast.ParticleElement:
		elemID, err := r.ensureElement(decl.Element, false)
		if err != nil {
			return 0, err
		}
		r.out.Particles[idx] = ElementParticle(id, elemID, minOccurs, maxOccurs, decl.Element != nil && !decl.Element.Ref.IsZero())
	case ast.ParticleWildcard:
		r.out.Particles[idx] = WildcardParticle(id, r.addWildcard(decl.Wildcard), minOccurs, maxOccurs)
	case ast.ParticleGroup:
		groupName := nameFromQName(decl.GroupRef)
		group, ok := r.globals.groups[groupName]
		if !ok {
			return 0, fmt.Errorf("schema ir: group ref %s not found", formatName(groupName))
		}
		if slices.Contains(stack, groupName) {
			return 0, fmt.Errorf("schema ir: group ref cycle detected: %s", formatName(groupName))
		}
		if group.Particle == nil {
			return 0, nil
		}
		particle := *group.Particle
		particle.Min = decl.Min
		particle.Max = decl.Max
		return r.addParticle(&particle, append(stack, groupName))
	case ast.ParticleSequence, ast.ParticleChoice, ast.ParticleAll:
		if err := validateDocumentParticleGroup(decl); err != nil {
			return 0, err
		}
		var children []ParticleID
		for i := range decl.Children {
			if decl.Kind == ast.ParticleAll && !ast.IsAllGroupChildMaxValid(decl.Children[i].Max) {
				return 0, fmt.Errorf("schema ir: xs:all: all particles must have maxOccurs <= 1 (got %s)", decl.Children[i].Max)
			}
			if (decl.Kind == ast.ParticleSequence || decl.Kind == ast.ParticleChoice) && r.groupRefUsesAllParticle(&decl.Children[i]) {
				return 0, fmt.Errorf("schema ir: xs:all cannot be nested inside %s", astParticleKindLabel(decl.Kind))
			}
			childID, err := r.addParticle(&decl.Children[i], stack)
			if err != nil {
				return 0, err
			}
			if childID != 0 {
				children = append(children, childID)
			}
		}
		r.out.Particles[idx] = GroupParticle(id, groupKindFromAST(decl.Kind), children, minOccurs, maxOccurs)
		if decl.Kind == ast.ParticleAll {
			if err := r.validateAllGroupUniqueElements(children); err != nil {
				return 0, err
			}
		}
	default:
		return 0, fmt.Errorf("schema ir: unsupported particle kind %d", decl.Kind)
	}
	return id, nil
}

func (r *docResolver) validateAllGroupUniqueElements(children []ParticleID) error {
	seen := make(map[Name]struct{}, len(children))
	for _, childID := range children {
		child, ok, err := r.particle(childID)
		if err != nil || !ok {
			return err
		}
		elemID, ok := child.ElementID()
		if !ok {
			continue
		}
		elem, ok := r.emittedElement(elemID)
		if !ok {
			continue
		}
		if _, exists := seen[elem.Name]; exists {
			return fmt.Errorf("schema ir: xs:all: duplicate element declaration '%s'", elem.Name.Local)
		}
		seen[elem.Name] = struct{}{}
	}
	return nil
}

func (r *docResolver) groupRefUsesAllParticle(decl *ast.ParticleDecl) bool {
	if decl == nil || decl.Kind != ast.ParticleGroup {
		return false
	}
	group := r.globals.groups[nameFromQName(decl.GroupRef)]
	return group != nil && group.Particle != nil && group.Particle.Kind == ast.ParticleAll
}

func astParticleKindLabel(kind ast.ParticleKind) string {
	switch kind {
	case ast.ParticleSequence:
		return "sequence"
	case ast.ParticleChoice:
		return "choice"
	case ast.ParticleAll:
		return "all"
	default:
		return "particle"
	}
}

func validateDocumentParticleOccurs(decl *ast.ParticleDecl) error {
	switch ast.CheckBounds(decl.Min, decl.Max) {
	case ast.BoundsOverflow:
		return fmt.Errorf("%w: occurrence value exceeds uint32", ast.ErrOccursOverflow)
	case ast.BoundsMaxZeroWithMinNonZero:
		return fmt.Errorf("schema ir: maxOccurs cannot be 0 when minOccurs > 0")
	case ast.BoundsMinGreaterThanMax:
		return fmt.Errorf("schema ir: minOccurs (%s) cannot be greater than maxOccurs (%s)", decl.Min, decl.Max)
	default:
		return nil
	}
}

func validateDocumentParticleGroup(decl *ast.ParticleDecl) error {
	if decl.Kind != ast.ParticleAll {
		return nil
	}
	switch ast.CheckAllGroupBounds(decl.Min, decl.Max) {
	case ast.AllGroupMinNotZeroOrOne:
		return fmt.Errorf("schema ir: xs:all: minOccurs must be 0 or 1 (got %s)", decl.Min)
	case ast.AllGroupMaxNotOne:
		return fmt.Errorf("schema ir: xs:all: maxOccurs must be 1 (got %s)", decl.Max)
	default:
		return nil
	}
}

func (r *docResolver) attributeUses(use *ast.AttributeUseDecl, stack []Name, emitLocalDecl bool) ([]AttributeUseID, error) {
	if use == nil {
		return nil, nil
	}
	if !use.AttributeGroup.IsZero() {
		return r.attributeGroupUses(nameFromQName(use.AttributeGroup), stack)
	}
	if use.Attribute == nil {
		return nil, nil
	}
	if err := validateDefaultFixed("attribute", nameFromQName(use.Attribute.Name), use.Attribute.Default, use.Attribute.Fixed); err != nil {
		return nil, err
	}
	var (
		attrID  AttributeID
		name    Name
		typeRef TypeRef
		err     error
	)
	if !use.Attribute.Ref.IsZero() {
		attrID, err = r.ensureAttribute(use.Attribute, false)
		if err != nil {
			return nil, err
		}
		attr, ok := r.attributeByID(attrID)
		if !ok {
			return nil, fmt.Errorf("schema ir: attribute %d not emitted", attrID)
		}
		name = attr.Name
		typeRef = attr.TypeDecl
		if err := validateDocumentAttributeReferenceValueCompatibility(name, use.Attribute.Default, use.Attribute.Fixed, attr.Fixed); err != nil {
			return nil, err
		}
	} else if emitLocalDecl {
		attrID, err = r.ensureAttribute(use.Attribute, false)
		if err != nil {
			return nil, err
		}
		attr, ok := r.attributeByID(attrID)
		if !ok {
			return nil, fmt.Errorf("schema ir: attribute %d not emitted", attrID)
		}
		name = effectiveDocumentAttributeUseName(use.Attribute)
		typeRef = attr.TypeDecl
	} else {
		name = effectiveDocumentAttributeUseName(use.Attribute)
		typeRef, err = r.typeUseRef(use.Attribute.Type, false)
		if err != nil {
			return nil, err
		}
	}
	if err := r.validateValueConstraintType("attribute", name, typeRef, use.Attribute.Default, use.Attribute.Fixed); err != nil {
		return nil, err
	}
	id := AttributeUseID(len(r.out.AttributeUses) + 1)
	r.out.AttributeUses = append(r.out.AttributeUses, AttributeUse{
		ID:       id,
		Name:     name,
		TypeDecl: typeRef,
		Use:      attributeUseKind(use.Attribute.Use),
		Decl:     attrID,
		Default:  r.valueConstraint(ValueConstraintDefault, use.Attribute.Default),
		Fixed:    r.valueConstraint(ValueConstraintFixed, use.Attribute.Fixed),
	})
	return []AttributeUseID{id}, nil
}

func effectiveDocumentAttributeUseName(decl *ast.AttributeDecl) Name {
	if decl == nil {
		return Name{}
	}
	return nameFromQName(decl.Name)
}

func validateDocumentAttributeReferenceValueCompatibility(
	name Name,
	def ast.ValueConstraintDecl,
	fixed ast.ValueConstraintDecl,
	targetFixed ValueConstraint,
) error {
	if !targetFixed.IsPresent() {
		return nil
	}
	if def.Present {
		return fmt.Errorf("schema ir: attribute reference '%s' cannot specify a default when declaration is fixed", name.Local)
	}
	if fixed.Present && fixed.Lexical != targetFixed.LexicalValue() {
		return fmt.Errorf("schema ir: attribute reference '%s' fixed value '%s' conflicts with declaration fixed value '%s'",
			name.Local, fixed.Lexical, targetFixed.LexicalValue())
	}
	return nil
}

func validateDefaultFixed(kind string, name Name, def, fixed ast.ValueConstraintDecl) error {
	if !def.Present || !fixed.Present {
		return nil
	}
	return fmt.Errorf("schema ir: %s %s cannot have both default and fixed values", kind, formatName(name))
}

func (r *docResolver) attributeByID(id AttributeID) (Attribute, bool) {
	for _, attr := range r.out.Attributes {
		if attr.ID == id {
			return attr, true
		}
	}
	return Attribute{}, false
}

func (r *docResolver) attributeGroupUses(name Name, stack []Name) ([]AttributeUseID, error) {
	group, ok := r.globals.attributeGroups[name]
	if !ok {
		return nil, fmt.Errorf("schema ir: attributeGroup ref %s not found", formatName(name))
	}
	if slices.Contains(stack, name) {
		return nil, fmt.Errorf("schema ir: attributeGroup ref cycle detected: %s", formatName(name))
	}
	stack = append(stack, name)
	var ids []AttributeUseID
	for i := range group.Attributes {
		groupIDs, err := r.attributeUses(&group.Attributes[i], stack, true)
		if err != nil {
			return nil, err
		}
		ids = append(ids, r.nonProhibitedUses(groupIDs)...)
	}
	for _, ref := range group.AttributeGroups {
		groupIDs, err := r.attributeGroupUses(nameFromQName(ref), stack)
		if err != nil {
			return nil, err
		}
		ids = append(ids, groupIDs...)
	}
	return ids, nil
}

func (r *docResolver) attributeGroupWildcard(name Name, stack []Name) (WildcardID, bool, error) {
	group, ok := r.globals.attributeGroups[name]
	if !ok {
		return 0, false, fmt.Errorf("schema ir: attributeGroup ref %s not found", formatName(name))
	}
	if slices.Contains(stack, name) {
		return 0, false, fmt.Errorf("schema ir: attributeGroup ref cycle detected: %s", formatName(name))
	}
	stack = append(stack, name)
	var wildcard WildcardID
	var hasWildcard bool
	if group.AnyAttribute != nil {
		wildcard = r.addWildcard(group.AnyAttribute)
		hasWildcard = true
	}
	for _, ref := range group.AttributeGroups {
		nested, hasNested, err := r.attributeGroupWildcard(nameFromQName(ref), stack)
		if err != nil {
			return 0, false, err
		}
		if hasNested {
			wildcard, hasWildcard = r.intersectLocalWildcards(wildcard, hasWildcard, nested)
		}
	}
	return wildcard, hasWildcard, nil
}

func (r *docResolver) nonProhibitedUses(ids []AttributeUseID) []AttributeUseID {
	out := ids[:0]
	for _, id := range ids {
		if id == 0 || int(id) > len(r.out.AttributeUses) {
			continue
		}
		if r.out.AttributeUses[id-1].Use == AttributeProhibited {
			continue
		}
		out = append(out, id)
	}
	return out
}

func (r *docResolver) addWildcard(decl *ast.WildcardDecl) WildcardID {
	if decl == nil {
		return 0
	}
	id := WildcardID(len(r.out.Wildcards) + 1)
	wildcard := Wildcard{
		ID:              id,
		NamespaceKind:   namespaceKind(decl.Namespace),
		TargetNamespace: decl.TargetNamespace,
		ProcessContents: processContents(decl.ProcessContents),
	}
	wildcard.Namespaces = append(wildcard.Namespaces, decl.NamespaceList...)
	r.out.Wildcards = append(r.out.Wildcards, wildcard)
	return id
}

func (r *docResolver) typeRef(qname ast.QName) (TypeRef, error) {
	ref := r.typeRefZero(qname)
	if !ref.IsZero() {
		return ref, nil
	}
	return NoTypeRef(), fmt.Errorf("schema ir: type %s not found", qname)
}

func (r *docResolver) typeRefZero(qname ast.QName) TypeRef {
	name := nameFromQName(qname)
	if ref, ok := r.builtins[name]; ok {
		return ref
	}
	if id, ok := r.ids.globalTypes[name]; ok {
		return UserTypeRef(id, name)
	}
	return NoTypeRef()
}

func (r *docResolver) elementID(qname ast.QName) (ElementID, error) {
	if qname.IsZero() {
		return 0, nil
	}
	name := nameFromQName(qname)
	if id, ok := r.ids.globalElements[name]; ok {
		if !r.emittedElems[id] {
			if decl, ok := r.globals.elementDecls[name]; ok {
				return r.ensureElement(decl, true)
			}
		}
		return id, nil
	}
	decl, ok := r.globals.elementDecls[name]
	if !ok {
		return 0, fmt.Errorf("schema ir: element %s not found", qname)
	}
	return r.ensureElement(decl, true)
}

func (r *docResolver) attributeID(qname ast.QName) (AttributeID, error) {
	if qname.IsZero() {
		return 0, nil
	}
	name := nameFromQName(qname)
	if id, ok := r.ids.globalAttributes[name]; ok {
		if !r.emittedAttrs[id] {
			if decl, ok := r.globals.attributeDecls[name]; ok {
				return r.ensureAttribute(decl, true)
			}
		}
		return id, nil
	}
	decl, ok := r.globals.attributeDecls[name]
	if !ok {
		return 0, fmt.Errorf("schema ir: attribute ref %s not found", qname)
	}
	return r.ensureAttribute(decl, true)
}

func (r *docResolver) identity(element ElementID, decl ast.IdentityDecl) (IdentityConstraint, error) {
	id := IdentityID(len(r.out.IdentityConstraints) + 1)
	name := nameFromQName(decl.Name)
	if existing := r.identityNames[name]; existing != 0 {
		return IdentityConstraint{}, fmt.Errorf("schema ir: identity constraint name %q is not unique within target namespace %q",
			name.Local, name.Namespace)
	}
	nsContext := r.contextMap(decl.NamespaceContextID)
	selectorExpr, err := xsdpath.Parse(decl.Selector, nsContext, xsdpath.AttributesDisallowed)
	if err != nil {
		return IdentityConstraint{}, fmt.Errorf("schema ir: selector xpath %q is invalid: %w", decl.Selector, err)
	}
	identity := IdentityConstraint{
		ID:               id,
		Element:          element,
		Name:             name,
		Selector:         decl.Selector,
		NamespaceContext: nsContext,
		Refer:            nameFromQName(decl.Refer),
	}
	switch decl.Kind {
	case ast.IdentityKey:
		identity.Kind = IdentityKey
	case ast.IdentityKeyref:
		identity.Kind = IdentityKeyRef
	default:
		identity.Kind = IdentityUnique
	}
	for _, field := range decl.Fields {
		fieldExpr, err := xsdpath.Parse(field, nsContext, xsdpath.AttributesAllowed)
		if err != nil {
			return IdentityConstraint{}, fmt.Errorf("schema ir: field xpath %q is invalid: %w", field, err)
		}
		resolved, err := r.resolveIdentityField(element, identity.Kind, decl.Selector, selectorExpr, field, fieldExpr)
		if err != nil {
			return IdentityConstraint{}, err
		}
		identity.Fields = append(identity.Fields, IdentityField{XPath: field, TypeDecl: resolved.TypeDecl})
	}
	r.identityNames[name] = id
	return identity, nil
}

type identityReferencePhaseInput struct {
	Constraints []IdentityConstraint
	Elements    []Element
}

type identityReferencePhaseOutput struct {
	Constraints []IdentityConstraint
}

type identityTypeCompatibility func(TypeRef, TypeRef) (bool, error)

func resolveIdentityReferences(input identityReferencePhaseInput, compatible identityTypeCompatibility) (identityReferencePhaseOutput, error) {
	constraints := slices.Clone(input.Constraints)
	keys := make(map[Name]IdentityID, len(constraints))
	for _, constraint := range constraints {
		if constraint.Kind == IdentityKey || constraint.Kind == IdentityUnique {
			keys[constraint.Name] = constraint.ID
		}
	}

	for i := range constraints {
		constraint := &constraints[i]
		if constraint.Kind != IdentityKeyRef {
			continue
		}
		if constraint.Refer.Namespace != constraint.Name.Namespace {
			return identityReferencePhaseOutput{}, fmt.Errorf("schema ir: keyref constraint %q refers to %s in namespace %q, which does not match target namespace %q",
				constraint.Name.Local, constraint.Refer.Local, constraint.Refer.Namespace, constraint.Name.Namespace)
		}
		id, ok := keys[constraint.Refer]
		if !ok {
			return identityReferencePhaseOutput{}, fmt.Errorf("schema ir: keyref %s refers to missing key", formatName(constraint.Name))
		}
		if int(id) > len(constraints) {
			return identityReferencePhaseOutput{}, fmt.Errorf("schema ir: keyref %s refers to missing key", formatName(constraint.Name))
		}
		target := constraints[id-1]
		if len(constraint.Fields) != len(target.Fields) {
			return identityReferencePhaseOutput{}, fmt.Errorf("schema ir: keyref constraint %q has %d fields but referenced constraint %q has %d fields",
				constraint.Name.Local, len(constraint.Fields), target.Name.Local, len(target.Fields))
		}
		for field := range constraint.Fields {
			keyrefType := constraint.Fields[field].TypeDecl
			keyType := target.Fields[field].TypeDecl
			if keyrefType.IsZero() || keyType.IsZero() {
				continue
			}
			ok, err := compatible(keyrefType, keyType)
			if err != nil {
				return identityReferencePhaseOutput{}, err
			}
			if !ok {
				return identityReferencePhaseOutput{}, fmt.Errorf("schema ir: keyref constraint %q field %d type %s is not compatible with referenced constraint %q field %d type %s",
					constraint.Name.Local, field+1, formatName(keyrefType.TypeName()), target.Name.Local, field+1, formatName(keyType.TypeName()))
			}
		}
		constraint.ReferID = id
	}
	return identityReferencePhaseOutput{Constraints: constraints}, nil
}

type runtimeEmissionPhaseInput struct {
	BuiltinTypes        []BuiltinType
	Types               []TypeDecl
	Elements            []Element
	Attributes          []Attribute
	AttributeUses       []AttributeUse
	ComplexTypes        []ComplexTypePlan
	Particles           []Particle
	Wildcards           []Wildcard
	IdentityConstraints []IdentityConstraint
	Docs                []ast.SchemaDocument
	Globals             globalIndex
	IDs                 idPlan
}

type runtimeEmissionPhaseOutput struct {
	ElementRefs   []ElementReference
	AttributeRefs []AttributeReference
	GroupRefs     []GroupReference
	GlobalIndexes GlobalIndexes
	RuntimeNames  RuntimeNamePlan
	Names         Names
}

type runtimeEmissionBuilder struct {
	input runtimeEmissionPhaseInput
	out   runtimeEmissionPhaseOutput
	names map[Name]struct{}
}

func buildRuntimeEmission(input runtimeEmissionPhaseInput) runtimeEmissionPhaseOutput {
	builder := runtimeEmissionBuilder{
		input: input,
		names: make(map[Name]struct{}),
	}
	builder.emitReferences()
	builder.emitGlobalIndexes()
	builder.emitRuntimeNamePlan()
	builder.emitNames()
	return builder.out
}

func (b *runtimeEmissionBuilder) emitReferences() {
	for name, id := range b.input.IDs.globalElements {
		b.out.ElementRefs = append(b.out.ElementRefs, ElementReference{Name: name, Element: id})
	}
	slices.SortFunc(b.out.ElementRefs, func(a, b ElementReference) int {
		return compareName(a.Name, b.Name)
	})
	for name, id := range b.input.IDs.globalAttributes {
		b.out.AttributeRefs = append(b.out.AttributeRefs, AttributeReference{Name: name, Attribute: id})
	}
	slices.SortFunc(b.out.AttributeRefs, func(a, b AttributeReference) int {
		return compareName(a.Name, b.Name)
	})
	for name, group := range b.input.Globals.groups {
		b.out.GroupRefs = append(b.out.GroupRefs, GroupReference{Name: name, Target: nameFromQName(group.Name)})
	}
	slices.SortFunc(b.out.GroupRefs, func(a, b GroupReference) int {
		return compareName(a.Name, b.Name)
	})
}

func (b *runtimeEmissionBuilder) emitGlobalIndexes() {
	for _, builtin := range b.input.BuiltinTypes {
		b.out.GlobalIndexes.Types = append(b.out.GlobalIndexes.Types, GlobalTypeIndex{Name: builtin.Name, Builtin: true})
	}
	for _, typ := range b.input.Types {
		if typ.Global {
			b.out.GlobalIndexes.Types = append(b.out.GlobalIndexes.Types, GlobalTypeIndex{Name: typ.Name, TypeDecl: typ.ID})
		}
	}
	for _, elem := range b.input.Elements {
		if elem.Global {
			b.out.GlobalIndexes.Elements = append(b.out.GlobalIndexes.Elements, GlobalElementIndex{Name: elem.Name, Element: elem.ID})
		}
	}
	for _, attr := range b.input.Attributes {
		if attr.Global {
			b.out.GlobalIndexes.Attributes = append(b.out.GlobalIndexes.Attributes, GlobalAttributeIndex{Name: attr.Name, Attribute: attr.ID})
		}
	}
}

func (b *runtimeEmissionBuilder) emitRuntimeNamePlan() {
	for _, builtin := range b.input.BuiltinTypes {
		b.addRuntimeSymbol(builtin.Name)
	}
	for _, typ := range b.input.Types {
		if !docIsZeroName(typ.Name) {
			b.addRuntimeSymbol(typ.Name)
		}
	}
	for _, elem := range b.input.Elements {
		b.addRuntimeSymbol(elem.Name)
	}
	for _, attr := range b.input.Attributes {
		b.addRuntimeSymbol(attr.Name)
	}
	seenWildcards := make(map[WildcardID]bool)
	for _, plan := range b.input.ComplexTypes {
		for _, id := range plan.Attrs {
			if id == 0 || int(id) > len(b.input.AttributeUses) {
				continue
			}
			b.addRuntimeSymbol(b.input.AttributeUses[id-1].Name)
		}
		b.addRuntimeWildcardNamespaces(plan.AnyAttr, seenWildcards)
	}
	for _, particle := range b.input.Particles {
		if wildcard, ok := particle.WildcardID(); ok {
			b.addRuntimeWildcardNamespaces(wildcard, seenWildcards)
		}
	}
	for _, constraint := range b.input.IdentityConstraints {
		b.addRuntimeSymbol(constraint.Name)
	}
	var notations []Name
	for di := range b.input.Docs {
		for i := range b.input.Docs[di].Decls {
			decl := &b.input.Docs[di].Decls[i]
			if decl.Notation != nil {
				name := nameFromQName(decl.Notation.Name)
				notations = append(notations, name)
			}
		}
	}
	slices.SortFunc(notations, compareName)
	for _, name := range notations {
		b.out.RuntimeNames.Notations = append(b.out.RuntimeNames.Notations, name)
		b.addRuntimeSymbol(name)
	}
}

func (b *runtimeEmissionBuilder) addRuntimeSymbol(name Name) {
	if docIsZeroName(name) {
		return
	}
	b.name(name)
	b.out.RuntimeNames.Ops = append(b.out.RuntimeNames.Ops, RuntimeNameOp{Kind: RuntimeNameSymbol, Name: name})
}

func (b *runtimeEmissionBuilder) addRuntimeNamespace(ns string) {
	b.name(Name{Namespace: ns})
	b.out.RuntimeNames.Ops = append(b.out.RuntimeNames.Ops, RuntimeNameOp{Kind: RuntimeNameNamespace, Namespace: ns})
}

func (b *runtimeEmissionBuilder) addRuntimeWildcardNamespaces(id WildcardID, seen map[WildcardID]bool) {
	if id == 0 || seen[id] || int(id) > len(b.input.Wildcards) {
		return
	}
	seen[id] = true
	wildcard := b.input.Wildcards[id-1]
	switch wildcard.NamespaceKind {
	case NamespaceTarget, NamespaceOther:
		if wildcard.TargetNamespace != "" {
			b.addRuntimeNamespace(wildcard.TargetNamespace)
		}
	case NamespaceList:
		for _, ns := range wildcard.Namespaces {
			switch ns {
			case "":
				continue
			case ast.NamespaceTargetPlaceholder:
				if wildcard.TargetNamespace != "" {
					b.addRuntimeNamespace(wildcard.TargetNamespace)
				}
			default:
				b.addRuntimeNamespace(ns)
			}
		}
	}
}

func (b *runtimeEmissionBuilder) emitNames() {
	for name := range b.names {
		b.out.Names.Values = append(b.out.Names.Values, name)
	}
	slices.SortFunc(b.out.Names.Values, compareName)
}

func (b *runtimeEmissionBuilder) name(name Name) {
	b.names[name] = struct{}{}
}

func (r *docResolver) builtinRef(local string) TypeRef {
	name := Name{Namespace: ast.XSDNamespace, Local: local}
	if ref, ok := r.builtins[name]; ok {
		return ref
	}
	return BuiltinTypeRef(0, name)
}

func (r *docResolver) specForRef(ref TypeRef) (SimpleTypeSpec, bool) {
	if ref.IsBuiltin() {
		for _, builtin := range r.out.BuiltinTypes {
			if builtin.Name == ref.TypeName() {
				return builtin.Value, true
			}
		}
		return SimpleTypeSpec{}, false
	}
	refID := ref.TypeID()
	for _, spec := range r.out.SimpleTypes {
		if spec.TypeDecl == refID {
			return spec, true
		}
	}
	return SimpleTypeSpec{}, false
}

func (r *docResolver) valueConstraint(kind ValueConstraintKind, value ast.ValueConstraintDecl) ValueConstraint {
	if !value.Present {
		return NoValueConstraint()
	}
	switch kind {
	case ValueConstraintDefault:
		return DefaultValueConstraint(value.Lexical, r.contextMap(value.NamespaceContextID))
	case ValueConstraintFixed:
		return FixedValueConstraint(value.Lexical, r.contextMap(value.NamespaceContextID))
	default:
		return NoValueConstraint()
	}
}

func (r *docResolver) contextMap(id ast.NamespaceContextID) map[string]string {
	context, ok := r.contexts[id]
	if !ok {
		return nil
	}
	out := make(map[string]string, len(context.Bindings))
	for _, binding := range context.Bindings {
		out[binding.Prefix] = binding.URI
	}
	return out
}
