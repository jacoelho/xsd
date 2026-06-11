package xsd

import "slices"

func (c *compiler) freezeRuntime() (*runtimeSchema, error) {
	rt := c.rt
	if err := validateRuntimeSchema(&rt); err != nil {
		return nil, err
	}
	return &rt, nil
}

func validateRuntimeSchema(rt *runtimeSchema) error {
	if err := validateRuntimeGlobals(rt); err != nil {
		return err
	}
	if err := validateRuntimeSubstitutions(rt); err != nil {
		return err
	}
	if err := validateBuiltinIDs(rt); err != nil {
		return err
	}
	if err := validateRuntimeComponents(rt); err != nil {
		return err
	}
	return validateRuntimeCompiledModels(rt)
}

func validateRuntimeGlobals(rt *runtimeSchema) error {
	for q, id := range rt.GlobalAttributes {
		if !validQName(rt, q) || !validAttributeID(rt, id) {
			return internalInvariant("global attribute references invalid declaration")
		}
	}
	for q, id := range rt.GlobalElements {
		if !validQName(rt, q) || !validElementID(rt, id) {
			return internalInvariant("global element references invalid declaration")
		}
	}
	for q, typ := range rt.GlobalTypes {
		if !validQName(rt, q) || !validTypeID(rt, typ) {
			return internalInvariant("global type references invalid declaration")
		}
	}
	for q, id := range rt.GlobalIdentities {
		if !validQName(rt, q) || !validIdentityID(rt, id) {
			return internalInvariant("global identity references invalid declaration")
		}
	}
	for q := range rt.Notations {
		if !validQName(rt, q) {
			return internalInvariant("notation references invalid name")
		}
	}
	return nil
}

func validateRuntimeSubstitutions(rt *runtimeSchema) error {
	for head, members := range rt.Substitutions {
		if !validElementID(rt, head) {
			return internalInvariant("substitution head references invalid element")
		}
		for _, member := range members {
			if !validElementID(rt, member) {
				return internalInvariant("substitution member references invalid element")
			}
		}
	}
	for head, members := range rt.SubstitutionLookup {
		if !validElementID(rt, head) {
			return internalInvariant("substitution lookup head references invalid element")
		}
		for name, member := range members {
			if !validQName(rt, name) || !validElementID(rt, member) {
				return internalInvariant("substitution lookup references invalid element")
			}
			if rt.Elements[member].Name != name {
				return internalInvariant("substitution lookup name does not match element")
			}
		}
	}
	return validateSubstitutionLookup(rt)
}

func validateRuntimeComponents(rt *runtimeSchema) error {
	for i := range rt.Elements {
		if err := validateElementDecl(rt, rt.Elements[i]); err != nil {
			return err
		}
	}
	for i := range rt.Attributes {
		if err := validateAttributeDecl(rt, rt.Attributes[i]); err != nil {
			return err
		}
	}
	for i := range rt.SimpleTypes {
		if err := validateSimpleType(rt, simpleTypeID(i), rt.SimpleTypes[i]); err != nil {
			return err
		}
	}
	for i := range rt.ComplexTypes {
		if err := validateComplexType(rt, complexTypeID(i), rt.ComplexTypes[i]); err != nil {
			return err
		}
	}
	for i := range rt.AttributeUseSets {
		if err := validateAttributeUseSetRuntime(rt, rt.AttributeUseSets[i]); err != nil {
			return err
		}
	}
	for i := range rt.Identities {
		if err := validateIdentityConstraint(rt, rt.Identities[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeCompiledModels(rt *runtimeSchema) error {
	for i := range rt.Models {
		if err := validateContentModelRuntime(rt, rt.Models[i]); err != nil {
			return err
		}
	}
	if len(rt.CompiledModels) != len(rt.Models) {
		return internalInvariant("compiled content model count does not match model count")
	}
	for i := range rt.CompiledModels {
		if err := validateCompiledModelRuntime(rt, rt.CompiledModels[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateSubstitutionLookup(rt *runtimeSchema) error {
	for head, members := range rt.Substitutions {
		for _, member := range members {
			if !rt.substitutionAllowed(head, member) {
				continue
			}
			byName := rt.SubstitutionLookup[head]
			if byName == nil || byName[rt.Elements[member].Name] != member {
				return internalInvariant("substitution lookup is missing allowed member")
			}
		}
	}
	for head, lookup := range rt.SubstitutionLookup {
		members := rt.Substitutions[head]
		for _, member := range lookup {
			if !slices.Contains(members, member) {
				return internalInvariant("substitution lookup contains non-member")
			}
			if !rt.substitutionAllowed(head, member) {
				return internalInvariant("substitution lookup contains blocked member")
			}
		}
	}
	return nil
}

func internalInvariant(msg string) error {
	return &Error{Category: InternalErrorCategory, Code: ErrInternalInvariant, Message: msg}
}

func validateBuiltinIDs(rt *runtimeSchema) error {
	ids := []simpleTypeID{
		rt.Builtin.AnySimpleType,
		rt.Builtin.String,
		rt.Builtin.Boolean,
		rt.Builtin.Decimal,
		rt.Builtin.Integer,
		rt.Builtin.Int,
		rt.Builtin.Date,
		rt.Builtin.DateTime,
		rt.Builtin.Time,
		rt.Builtin.AnyURI,
		rt.Builtin.qName,
		rt.Builtin.ID,
		rt.Builtin.IDREF,
		rt.Builtin.IDREFS,
		rt.Builtin.NMTOKEN,
		rt.Builtin.NMTOKENS,
		rt.Builtin.ENTITY,
		rt.Builtin.ENTITIES,
	}
	for _, id := range ids {
		if !validSimpleTypeID(rt, id) {
			return internalInvariant("builtin simple type references invalid declaration")
		}
	}
	if !validComplexTypeID(rt, rt.Builtin.AnyType) {
		return internalInvariant("builtin anyType references invalid declaration")
	}
	return nil
}

func validateElementDecl(rt *runtimeSchema, decl elementDecl) error {
	if !validQName(rt, decl.Name) || !validTypeID(rt, decl.Type) {
		return internalInvariant("element declaration references invalid name or type")
	}
	if decl.SubstHead != noElement && !validElementID(rt, decl.SubstHead) {
		return internalInvariant("element declaration references invalid substitution head")
	}
	for _, id := range decl.Identity {
		if !validIdentityID(rt, id) {
			return internalInvariant("element declaration references invalid identity constraint")
		}
	}
	if err := validateValueConstraintRuntime(rt, decl.Default, "element declaration default"); err != nil {
		return err
	}
	return validateValueConstraintRuntime(rt, decl.Fixed, "element declaration fixed")
}

func validateAttributeDecl(rt *runtimeSchema, decl attributeDecl) error {
	if !validQName(rt, decl.Name) || !validSimpleTypeID(rt, decl.Type) {
		return internalInvariant("attribute declaration references invalid name or type")
	}
	if err := validateValueConstraintRuntime(rt, decl.Default, "attribute declaration default"); err != nil {
		return err
	}
	return validateValueConstraintRuntime(rt, decl.Fixed, "attribute declaration fixed")
}

func validateSimpleType(rt *runtimeSchema, id simpleTypeID, st simpleType) error {
	if !validQName(rt, st.Name) {
		return internalInvariant("simple type references invalid name")
	}
	if st.Base != noSimpleType && !validSimpleTypeID(rt, st.Base) {
		return internalInvariant("simple type references invalid base")
	}
	if st.ListItem != noSimpleType && !validSimpleTypeID(rt, st.ListItem) {
		return internalInvariant("simple type references invalid list item")
	}
	for _, member := range st.Union {
		if !validSimpleTypeID(rt, member) {
			return internalInvariant("simple type references invalid union member")
		}
	}
	if st.Identity != expectedSimpleIdentity(rt, id, st) {
		return internalInvariant("simple type identity does not match derivation")
	}
	return validateFacetPresence(st.Facets)
}

func expectedSimpleIdentity(rt *runtimeSchema, id simpleTypeID, st simpleType) simpleIdentityKind {
	switch id {
	case rt.Builtin.ID:
		return simpleIdentityID
	case rt.Builtin.IDREF:
		return simpleIdentityIDREF
	}
	return rt.derivedSimpleIdentity(st)
}

func validateFacetPresence(f facetSet) error {
	facets := []struct {
		name    string
		flag    facetFlag
		present bool
	}{
		{xsdFacetLength, facetFlagLength, f.Length != nil},
		{xsdFacetMinLength, facetFlagMinLength, f.MinLength != nil},
		{xsdFacetMaxLength, facetFlagMaxLength, f.MaxLength != nil},
		{xsdFacetTotalDigits, facetFlagTotalDigits, f.TotalDigits != nil},
		{xsdFacetFractionDigits, facetFlagFractionDigits, f.FractionDigits != nil},
		{xsdFacetMinInclusive, facetFlagMinInclusive, f.MinInclusive != nil},
		{xsdFacetMaxInclusive, facetFlagMaxInclusive, f.MaxInclusive != nil},
		{xsdFacetMinExclusive, facetFlagMinExclusive, f.MinExclusive != nil},
		{xsdFacetMaxExclusive, facetFlagMaxExclusive, f.MaxExclusive != nil},
		{xsdFacetEnumeration, facetFlagEnumeration, len(f.Enumeration) != 0},
		{xsdFacetPattern, facetFlagPattern, len(f.Patterns) != 0},
	}
	for _, facet := range facets {
		if (f.Present&facet.flag != 0) != facet.present {
			return internalInvariant("simple type facet presence mask does not match " + facet.name + " facet")
		}
	}
	if f.Present&facetFlagWhiteSpace != 0 {
		return internalInvariant("simple type facet presence mask cannot set whiteSpace")
	}
	return nil
}

func validateComplexType(rt *runtimeSchema, id complexTypeID, ct complexType) error {
	if !validQName(rt, ct.Name) {
		return internalInvariant("complex type references invalid name")
	}
	if ct.Base == (typeID{}) {
		if id != rt.Builtin.AnyType {
			return internalInvariant("complex type has no base type")
		}
	} else if !validTypeID(rt, ct.Base) {
		return internalInvariant("complex type references invalid base")
	}
	if ct.Content != noContentModel && !validContentModelID(rt, ct.Content) {
		return internalInvariant("complex type references invalid content model")
	}
	if ct.Attrs != noAttributeUseSet && !validAttributeUseSetID(rt, ct.Attrs) {
		return internalInvariant("complex type references invalid attribute use set")
	}
	if !ct.simpleContent() {
		if ct.TextType != noSimpleType {
			return internalInvariant("complex type stores text type without simple content")
		}
		return nil
	}
	if !validSimpleTypeID(rt, ct.TextType) {
		return internalInvariant("complex type references invalid text type")
	}
	if !validContentModelID(rt, ct.Content) || rt.Models[ct.Content].Kind != modelEmpty {
		return internalInvariant("complex type simple content must have empty content model")
	}
	return nil
}

func validateAttributeUseSetRuntime(rt *runtimeSchema, set attributeUseSet) error {
	if set.Wildcard != noWildcard && !validWildcardID(rt, set.Wildcard) {
		return internalInvariant("attribute use set references invalid wildcard")
	}
	for i, use := range set.Uses {
		if !validQName(rt, use.Name) || !validSimpleTypeID(rt, use.Type) {
			return internalInvariant("attribute use references invalid name or type")
		}
		if slot, ok := set.Index[use.Name]; !ok || slot != uint32(i) {
			return internalInvariant("attribute use index does not match use slice")
		}
		if err := validateValueConstraintRuntime(rt, use.Default, "attribute use default"); err != nil {
			return err
		}
		if err := validateValueConstraintRuntime(rt, use.Fixed, "attribute use fixed"); err != nil {
			return err
		}
	}
	for _, slot := range set.Required {
		if !validUint32Index(slot, len(set.Uses)) || !set.Uses[slot].Required {
			return internalInvariant("attribute use set required slot is invalid")
		}
	}
	for _, slot := range set.ValueConstraints {
		if !validUint32Index(slot, len(set.Uses)) || (!set.Uses[slot].Default.Present && !set.Uses[slot].Fixed.Present) {
			return internalInvariant("attribute use set value constraint slot is invalid")
		}
	}
	return nil
}

func validateValueConstraintRuntime(rt *runtimeSchema, vc valueConstraint, label string) error {
	if !vc.Present {
		if vc.Canonical != "" {
			return internalInvariant(label + " stores canonical without value constraint")
		}
		if vc.Value.Canonical != "" || vc.Value.IDs != "" || vc.Value.IDRefs != "" {
			return internalInvariant(label + " stores value without value constraint")
		}
		return nil
	}
	if vc.Value.Canonical != vc.Canonical {
		return internalInvariant(label + " canonical value mismatch")
	}
	if vc.Value.Type != noSimpleType && !validSimpleTypeID(rt, vc.Value.Type) {
		return internalInvariant(label + " references invalid simple type")
	}
	return nil
}

func validateContentModelRuntime(rt *runtimeSchema, model contentModel) error {
	for _, p := range model.Particles {
		switch p.Kind {
		case particleElement:
			if !validElementID(rt, p.Element) {
				return internalInvariant("particle references invalid element")
			}
		case particleModel:
			if !validContentModelID(rt, p.Model) {
				return internalInvariant("particle references invalid content model")
			}
		case particleWildcard:
			if !validWildcardID(rt, p.Wildcard) {
				return internalInvariant("particle references invalid wildcard")
			}
		default:
			return internalInvariant("particle has invalid kind")
		}
		if err := validateParticleInactiveFields(p); err != nil {
			return err
		}
	}
	return nil
}

// validateParticleInactiveFields enforces the constructor invariant that a
// particle's inactive ID fields hold their no* sentinels; kind-blind particle
// comparisons rely on it.
func validateParticleInactiveFields(p particle) error {
	if p.Kind != particleElement && p.Element != noElement {
		return internalInvariant("particle stores element ID for non-element kind")
	}
	if p.Kind != particleModel && p.Model != noContentModel {
		return internalInvariant("particle stores content model ID for non-model kind")
	}
	if p.Kind != particleWildcard && p.Wildcard != noWildcard {
		return internalInvariant("particle stores wildcard ID for non-wildcard kind")
	}
	return nil
}

func validateCompiledModelRuntime(rt *runtimeSchema, model compiledModel) error {
	switch model.Kind {
	case compiledModelEmpty, compiledModelAny:
		return nil
	case compiledModelAll:
		for _, term := range model.All {
			if err := validateCompiledParticle(rt, term.Particle); err != nil {
				return err
			}
		}
	case compiledModelDFA:
		return validateCompiledDFARuntime(rt, model)
	default:
		return internalInvariant("compiled content model has invalid kind")
	}
	return nil
}

func validateCompiledDFARuntime(rt *runtimeSchema, model compiledModel) error {
	if !validUint32Index(model.Start, len(model.Rows)) {
		return internalInvariant("compiled content model start state is invalid")
	}
	for i, row := range model.Rows {
		index, err := checkedUint32(i, "compiled content model row index limit exceeded")
		if err != nil {
			return internalInvariant("compiled content model row index is invalid")
		}
		if err := validateCompiledDFARow(rt, model, row, index); err != nil {
			return err
		}
	}
	return nil
}

func validateCompiledDFARow(rt *runtimeSchema, model compiledModel, row compiledModelRow, index uint32) error {
	if row.Counted && !row.Unbounded && row.Max < row.Min {
		return internalInvariant("compiled content model counted state has invalid range")
	}
	if row.Counted {
		if err := validateCompiledParticle(rt, row.CountParticle); err != nil {
			return err
		}
	}
	countedLoops := 0
	for _, edge := range row.Edges {
		if !validUint32Index(edge.To, len(model.Rows)) {
			return internalInvariant("compiled content model edge target is invalid")
		}
		if err := validateCompiledParticle(rt, edge.Particle); err != nil {
			return err
		}
		if row.Counted && edge.To == index {
			if !sameCompiledParticle(edge.Particle, row.CountParticle) {
				return internalInvariant("compiled content model counted state has non-counted self loop")
			}
			countedLoops++
		}
	}
	if row.Counted && countedLoops != 1 {
		return internalInvariant("compiled content model counted state must have one counted self loop")
	}
	return nil
}

func validateCompiledParticle(rt *runtimeSchema, p particle) error {
	switch p.Kind {
	case particleElement:
		if !validElementID(rt, p.Element) {
			return internalInvariant("compiled particle references invalid element")
		}
	case particleWildcard:
		if !validWildcardID(rt, p.Wildcard) {
			return internalInvariant("compiled particle references invalid wildcard")
		}
	default:
		return internalInvariant("compiled particle has invalid kind")
	}
	return validateParticleInactiveFields(p)
}

func validateIdentityConstraint(rt *runtimeSchema, ic identityConstraint) error {
	if !validQName(rt, ic.Name) {
		return internalInvariant("identity constraint references invalid name")
	}
	if ic.Refer != noIdentityConstraint && !validIdentityID(rt, ic.Refer) {
		return internalInvariant("identity constraint references invalid key")
	}
	for _, path := range ic.Selector {
		if !validIdentitySteps(rt, path.Steps) {
			return internalInvariant("identity selector references invalid name")
		}
	}
	for _, field := range ic.Fields {
		for _, path := range field.Paths {
			if !validIdentityFieldPath(rt, path) {
				return internalInvariant("identity field references invalid attribute")
			}
		}
	}
	return nil
}

func validIdentityFieldPath(rt *runtimeSchema, path identityFieldPath) bool {
	if path.Attr && !path.AttrWildcard && !validQName(rt, path.Attribute) {
		return false
	}
	return validIdentitySteps(rt, path.Steps)
}

func validIdentitySteps(rt *runtimeSchema, steps []identityStep) bool {
	for _, step := range steps {
		if !step.Wildcard && !validQName(rt, step.Name) {
			return false
		}
	}
	return true
}

func validTypeID(rt *runtimeSchema, typ typeID) bool {
	switch typ.Kind {
	case typeSimple:
		return validSimpleTypeID(rt, simpleTypeID(typ.ID))
	case typeComplex:
		return validComplexTypeID(rt, complexTypeID(typ.ID))
	case typeNone:
		return false
	default:
		return false
	}
}

func validQName(rt *runtimeSchema, q qName) bool {
	return validUint32Index(uint32(q.Namespace), len(rt.Names.namespaces)) &&
		validUint32Index(uint32(q.Local), len(rt.Names.locals))
}

func validSimpleTypeID(rt *runtimeSchema, id simpleTypeID) bool {
	return validUint32Index(uint32(id), len(rt.SimpleTypes))
}

func validComplexTypeID(rt *runtimeSchema, id complexTypeID) bool {
	return validUint32Index(uint32(id), len(rt.ComplexTypes))
}

func validElementID(rt *runtimeSchema, id elementID) bool {
	return validUint32Index(uint32(id), len(rt.Elements))
}

func validAttributeID(rt *runtimeSchema, id attributeID) bool {
	return validUint32Index(uint32(id), len(rt.Attributes))
}

func validContentModelID(rt *runtimeSchema, id contentModelID) bool {
	return validUint32Index(uint32(id), len(rt.Models))
}

func validAttributeUseSetID(rt *runtimeSchema, id attributeUseSetID) bool {
	return validUint32Index(uint32(id), len(rt.AttributeUseSets))
}

func validWildcardID(rt *runtimeSchema, id wildcardID) bool {
	return validUint32Index(uint32(id), len(rt.Wildcards))
}

func validIdentityID(rt *runtimeSchema, id identityConstraintID) bool {
	return validUint32Index(uint32(id), len(rt.Identities))
}
