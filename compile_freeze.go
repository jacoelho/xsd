package xsd

func (c *compiler) freezeRuntime() (*runtimeSchema, error) {
	rt := c.rt
	if err := validateRuntimeSchema(&rt); err != nil {
		return nil, err
	}
	return &rt, nil
}

func validateRuntimeSchema(rt *runtimeSchema) error {
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
	if err := validateSubstitutionLookup(rt); err != nil {
		return err
	}
	if err := validateBuiltinIDs(rt); err != nil {
		return err
	}
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
		if err := validateSimpleType(rt, rt.SimpleTypes[i]); err != nil {
			return err
		}
	}
	for i := range rt.ComplexTypes {
		if err := validateComplexType(rt, rt.ComplexTypes[i]); err != nil {
			return err
		}
	}
	for i := range rt.AttributeUseSets {
		if err := validateAttributeUseSetRuntime(rt, rt.AttributeUseSets[i]); err != nil {
			return err
		}
	}
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
	for i := range rt.Identities {
		if err := validateIdentityConstraint(rt, rt.Identities[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateSubstitutionLookup(rt *runtimeSchema) error {
	for head, members := range rt.Substitutions {
		for _, member := range members {
			if !runtimeSubstitutionAllowed(rt, head, member) {
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
			if !substitutionMemberExists(members, member) {
				return internalInvariant("substitution lookup contains non-member")
			}
			if !runtimeSubstitutionAllowed(rt, head, member) {
				return internalInvariant("substitution lookup contains blocked member")
			}
		}
	}
	return nil
}

func runtimeSubstitutionAllowed(rt *runtimeSchema, headID, memberID elementID) bool {
	head := rt.Elements[headID]
	member := rt.Elements[memberID]
	if head.Block&blockSubstitution != 0 {
		return false
	}
	return rt.substitutionDerivationAllowed(member.Type, head.Type, head.Block)
}

func substitutionMemberExists(members []elementID, member elementID) bool {
	for _, candidate := range members {
		if candidate == member {
			return true
		}
	}
	return false
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
	if !decl.HasDefault && decl.DefaultCanonical != "" {
		return internalInvariant("element declaration stores canonical default without default")
	}
	if !decl.HasFixed && decl.FixedCanonical != "" {
		return internalInvariant("element declaration stores canonical fixed without fixed")
	}
	if err := validateStoredSimpleValue(rt, decl.HasDefault, decl.DefaultCanonical, decl.DefaultValue, "element declaration default"); err != nil {
		return err
	}
	if err := validateStoredSimpleValue(rt, decl.HasFixed, decl.FixedCanonical, decl.FixedValue, "element declaration fixed"); err != nil {
		return err
	}
	return nil
}

func validateAttributeDecl(rt *runtimeSchema, decl attributeDecl) error {
	if !validQName(rt, decl.Name) || !validSimpleTypeID(rt, decl.Type) {
		return internalInvariant("attribute declaration references invalid name or type")
	}
	if !decl.HasDefault && decl.DefaultCanonical != "" {
		return internalInvariant("attribute declaration stores canonical default without default")
	}
	if !decl.HasFixed && decl.FixedCanonical != "" {
		return internalInvariant("attribute declaration stores canonical fixed without fixed")
	}
	if err := validateStoredSimpleValue(rt, decl.HasDefault, decl.DefaultCanonical, decl.DefaultValue, "attribute declaration default"); err != nil {
		return err
	}
	if err := validateStoredSimpleValue(rt, decl.HasFixed, decl.FixedCanonical, decl.FixedValue, "attribute declaration fixed"); err != nil {
		return err
	}
	return nil
}

func validateSimpleType(rt *runtimeSchema, st simpleType) error {
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
	return nil
}

func validateComplexType(rt *runtimeSchema, ct complexType) error {
	if !validQName(rt, ct.Name) {
		return internalInvariant("complex type references invalid name")
	}
	if ct.Base.ID != uint32(noComplexType) && !validTypeID(rt, ct.Base) {
		return internalInvariant("complex type references invalid base")
	}
	if ct.Content != noContentModel && !validContentModelID(rt, ct.Content) {
		return internalInvariant("complex type references invalid content model")
	}
	if ct.Attrs != noAttributeUseSet && !validAttributeUseSetID(rt, ct.Attrs) {
		return internalInvariant("complex type references invalid attribute use set")
	}
	if ct.SimpleValue && !validSimpleTypeID(rt, ct.TextType) {
		return internalInvariant("complex type references invalid text type")
	}
	return nil
}

func validateAttributeUseSetRuntime(rt *runtimeSchema, set attributeUseSet) error {
	if set.wildcard != noWildcard && !validWildcardID(rt, set.wildcard) {
		return internalInvariant("attribute use set references invalid wildcard")
	}
	for i, use := range set.Uses {
		if !validQName(rt, use.Name) || !validSimpleTypeID(rt, use.Type) {
			return internalInvariant("attribute use references invalid name or type")
		}
		if slot, ok := set.Index[use.Name]; !ok || slot != uint32(i) {
			return internalInvariant("attribute use index does not match use slice")
		}
		if !use.HasDefault && use.DefaultCanonical != "" {
			return internalInvariant("attribute use stores canonical default without default")
		}
		if !use.HasFixed && use.FixedCanonical != "" {
			return internalInvariant("attribute use stores canonical fixed without fixed")
		}
		if err := validateStoredSimpleValue(rt, use.HasDefault, use.DefaultCanonical, use.DefaultValue, "attribute use default"); err != nil {
			return err
		}
		if err := validateStoredSimpleValue(rt, use.HasFixed, use.FixedCanonical, use.FixedValue, "attribute use fixed"); err != nil {
			return err
		}
	}
	for _, slot := range set.Required {
		if !validUint32Index(slot, len(set.Uses)) || !set.Uses[slot].Required {
			return internalInvariant("attribute use set required slot is invalid")
		}
	}
	for _, slot := range set.ValueConstraints {
		if !validUint32Index(slot, len(set.Uses)) || (!set.Uses[slot].HasDefault && !set.Uses[slot].HasFixed) {
			return internalInvariant("attribute use set value constraint slot is invalid")
		}
	}
	return nil
}

func validateStoredSimpleValue(rt *runtimeSchema, has bool, canonical string, value simpleValue, label string) error {
	if !has {
		if value.Canonical != "" || value.IDs != "" || value.IDRefs != "" {
			return internalInvariant(label + " stores value without value constraint")
		}
		return nil
	}
	if value.Canonical != canonical {
		return internalInvariant(label + " canonical value mismatch")
	}
	if value.Type != noSimpleType && !validSimpleTypeID(rt, value.Type) {
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
			if !validWildcardID(rt, p.wildcard) {
				return internalInvariant("particle references invalid wildcard")
			}
		default:
			return internalInvariant("particle has invalid kind")
		}
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
		if !validUint32Index(model.Start, len(model.Rows)) {
			return internalInvariant("compiled content model start state is invalid")
		}
		for i, row := range model.Rows {
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
				if row.Counted && edge.To == uint32(i) {
					if !sameCompiledParticle(edge.Particle, row.CountParticle) {
						return internalInvariant("compiled content model counted state has non-counted self loop")
					}
					countedLoops++
				}
			}
			if row.Counted && countedLoops != 1 {
				return internalInvariant("compiled content model counted state must have one counted self loop")
			}
		}
	default:
		return internalInvariant("compiled content model has invalid kind")
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
		if !validWildcardID(rt, p.wildcard) {
			return internalInvariant("compiled particle references invalid wildcard")
		}
	default:
		return internalInvariant("compiled particle has invalid kind")
	}
	return nil
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
	if err := validateCompiledIdentityFields(rt, ic); err != nil {
		return err
	}
	return nil
}

func validateCompiledIdentityFields(rt *runtimeSchema, ic identityConstraint) error {
	for _, field := range ic.ElementFields {
		if err := validateCompiledIdentityField(rt, ic, field); err != nil {
			return err
		}
		for _, path := range field.Paths {
			if path.Attr {
				return internalInvariant("compiled identity element field contains attribute path")
			}
		}
	}
	for name, fields := range ic.AttributeFields {
		if !validQName(rt, name) {
			return internalInvariant("compiled identity attribute field references invalid attribute")
		}
		for _, field := range fields {
			if err := validateCompiledIdentityField(rt, ic, field); err != nil {
				return err
			}
			for _, path := range field.Paths {
				if path.AttrWildcard || path.Attribute != name {
					return internalInvariant("compiled identity attribute field is in wrong lookup bucket")
				}
			}
		}
	}
	for _, field := range ic.AttributeWildcardFields {
		if err := validateCompiledIdentityField(rt, ic, field); err != nil {
			return err
		}
		for _, path := range field.Paths {
			if !path.AttrWildcard {
				return internalInvariant("compiled identity wildcard field contains exact attribute path")
			}
		}
	}
	return nil
}

func validateCompiledIdentityField(rt *runtimeSchema, ic identityConstraint, field compiledIdentityField) error {
	if field.Field < 0 || field.Field >= len(ic.Fields) {
		return internalInvariant("compiled identity field references invalid field")
	}
	if len(field.Paths) == 0 {
		return internalInvariant("compiled identity field has no paths")
	}
	for _, path := range field.Paths {
		if !validIdentityFieldPath(rt, path) {
			return internalInvariant("compiled identity field references invalid path")
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
		if !step.wildcard && !validQName(rt, step.Name) {
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
