package schema

import (
	"slices"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func newSchemaBuild(names nameTable) schemaBuild {
	return schemaBuild{
		Names:            names,
		GlobalElements:   make(map[QName]ElementID),
		GlobalAttributes: make(map[QName]AttributeID, len(builtinXSIAttributes)),
		GlobalTypes:      make(map[QName]TypeID, builtinGlobalTypeCount()),
		GlobalIdentities: make(map[QName]IdentityConstraintID),
		Notations:        make(map[QName]bool),
		SimpleTypes:      make([]simpleType, 0, builtinSimpleTypeCount()),
		Attributes:       make([]attributeDecl, 0, len(builtinXSIAttributes)),
		ComplexTypes:     make([]complexTypeMetadata, 0, builtinComplexTypeCount()),
		Wildcards:        make([]wildcardRuntime, 0, 1),
		AttributeUseSets: make([]attributeUseSet, 0, 1),
		Models:           make([]contentModelNode, 0, 1),
	}
}

func (rt *schemaBuild) formatName(q QName) string {
	return rt.Names.Format(q)
}

func (rt *schemaBuild) namespaceURI(id namespaceID) string {
	return rt.Names.Namespace(id)
}

func (rt *schemaBuild) lookupQName(ns, local string) (QName, bool) {
	return rt.Names.LookupQName(ns, local)
}

func (rt *schemaBuild) complexType(id ComplexTypeID) complexTypeMetadata {
	return rt.ComplexTypes[id]
}

func (rt *schemaBuild) notationDeclared(q QName) bool {
	return rt.Notations[q]
}

func sortedBuildQNames[T any](rt *schemaBuild, values map[QName]T) []QName {
	return sortedQNames(values, rt.Names)
}

func (rt *schemaBuild) internQName(ns, local string) (QName, error) {
	return newNameInterner(&rt.Names).InternQName(ns, local)
}

func (rt *schemaBuild) simpleTypeFinalMask(id SimpleTypeID) DerivationMask {
	return rt.SimpleTypes[id].Final
}

// appendFlattenedUnionMember keeps a nested union with effective facets as one
// member so evaluation cannot discard its constraints. See I-010 in
// docs/spec/INTERPRETATIONS.md.
func (rt *schemaBuild) appendFlattenedUnionMember(
	dst *[]SimpleTypeID,
	id SimpleTypeID,
	seen map[SimpleTypeID]struct{},
	remaining int,
) (int, bool) {
	typ := rt.SimpleTypes[id]
	members := []SimpleTypeID{id}
	if typ.ValueSpec.Variety == value.Union && !typ.unionHasFacets {
		members = typ.ValueSpec.Union
	}
	added := 0
	for _, member := range members {
		if _, ok := seen[member]; ok {
			continue
		}
		if added == remaining {
			return added, false
		}
		seen[member] = struct{}{}
		*dst = append(*dst, member)
		added++
	}
	return added, true
}

// derivedSimpleType copies base as the starting point of a restriction step.
// Completed-base union and facet storage is immutable and remains shared until
// the restriction replaces a facet value or appends a persistent pattern step.
func (rt *schemaBuild) derivedSimpleType(id SimpleTypeID, name QName) simpleType {
	st := rt.SimpleTypes[id]
	st.Name = name
	st.Final = 0
	spec := st.ValueSpec
	spec.Base = id
	spec.Facets = value.FacetSpec{}
	spec.WhitespacePresent = false
	st.ValueSpec = spec
	return st
}

func cloneValueConstraint(vc *valueConstraint) *valueConstraint {
	if vc == nil {
		return nil
	}
	cloned := *vc
	cloned.ResolvedNames = slices.Clone(vc.ResolvedNames)
	cloned.qnameContext = cloneValueConstraintQNameContext(vc.qnameContext)
	return &cloned
}

func (c *compiler) elementCopies() []elementDecl {
	return slices.Clone(c.rt.Elements)
}

func (rt *schemaBuild) buildSubstitutionTable(elements []elementDecl, maxEntries int) (substitutionTable, error) {
	return buildSubstitutionTable(
		rt,
		&rt.Names,
		elements,
		rt.GlobalElements,
		maxEntries,
		rt.spendTypeDerivationWork,
	)
}

func (rt *schemaBuild) attributeUse(id AttributeID) attributeUse {
	decl := rt.Attributes[id]
	return attributeUse{
		Name:    decl.Name,
		Type:    decl.Type,
		Default: cloneValueConstraint(decl.Default),
		Fixed:   cloneValueConstraint(decl.Fixed),
	}
}

// attributeUsesAndWildcardForCompilation copies only the use slice. Value
// constraints are immutable after admission, and compilation replaces whole
// attributeUse values when applying overrides, so cloning each pointed record
// here only adds work to the inherited-use path.
func (rt *schemaBuild) attributeUsesAndWildcardForCompilation(id attributeUseSetID) ([]attributeUse, WildcardID) {
	if id == noAttributeUseSet {
		return nil, NoWildcard
	}
	set := rt.AttributeUseSets[id]
	return slices.Clone(set.Uses), set.Wildcard
}

func (rt *schemaBuild) identityName(id IdentityConstraintID) QName {
	return rt.Identities[id].Name
}

func (c *compiler) checkIdentityConstraintNameAvailable(q QName) error {
	return checkIdentityConstraintNameAvailable(c.rt.GlobalIdentities, q, c.rt.Names.Format(q))
}

func (c *compiler) resolveIdentityConstraintRefer(q QName) (IdentityConstraintID, error) {
	return resolveIdentityConstraintRefer(c.rt.GlobalIdentities, q, c.rt.Names.Format(q))
}

func (c *compiler) validateIdentityReferencesBuild() error {
	return validateIdentityReferences(c.rt.Identities)
}

func (c *compiler) compileContentModelsBuild() ([]compiledModel, error) {
	return compileContentModels(
		&c.rt.Names,
		&c.rt,
		len(c.rt.Models),
		c.limits.MaxContentModelStates,
		&c.contentWork,
		c.contentAnalysis,
	)
}

func (c *compiler) checkContentModelsUPABuild() error {
	return checkContentModelsUPA(&c.rt.Names, &c.rt, len(c.rt.Models), &c.contentWork, c.contentAnalysis)
}

func (c *compiler) checkContentModelElementDeclarationsConsistentBuild() error {
	if len(c.modelSources) != len(c.rt.Models) {
		return xsderrors.InternalInvariant("content model provenance count does not match model count")
	}
	checker := newElementDeclarationConsistencyChecker(&c.rt, c.contentWork.spend)
	for id := range c.rt.Models {
		if err := checker.checkModel(contentModelID(id)); err != nil {
			if c.modelSources[id] != nil {
				return withSchemaCompileLocation(c.modelSources[id], err)
			}
			return err
		}
	}
	return nil
}

func (c *compiler) validateAttributeDeclNameBuild(q QName) error {
	return validateAttributeDeclName(&c.rt.Names, q)
}

func (c *compiler) validateAttributeUseSetBuild(set attributeUseSet) error {
	return validateAttributeUseSetRecord(&c.rt.Names, &c.rt, set)
}

func (c *compiler) simpleListItemReachesList(id SimpleTypeID) bool {
	if !validSimpleTypeID(id, len(c.rt.SimpleTypes)) {
		return false
	}
	typ := c.rt.SimpleTypes[id]
	return typ.ValueSpec.Variety == value.List || typ.unionContainsList
}

func (c *compiler) registerGlobalElement(q QName, decl elementDecl) (ElementID, error) {
	id, err := c.addElement(decl)
	if err != nil {
		return NoElement, err
	}
	c.rt.Elements[id].Scope = declarationScopeGlobal
	c.rt.GlobalElements[q] = id
	return id, nil
}

func (c *compiler) registerGlobalAttribute(q QName, decl attributeDecl) (AttributeID, error) {
	id, err := nextAttributeID(len(c.rt.Attributes))
	if err != nil {
		return 0, err
	}
	c.rt.Attributes = append(c.rt.Attributes, decl)
	c.rt.GlobalAttributes[q] = id
	return id, nil
}

func (c *compiler) registerGlobalComplexType(q QName, typ complexTypeMetadata) (ComplexTypeID, error) {
	id, err := c.addComplexType(typ)
	if err != nil {
		return noComplexType, err
	}
	c.rt.ComplexTypes[id].Scope = declarationScopeGlobal
	c.rt.GlobalTypes[q] = ComplexRef(id)
	return id, nil
}

func (c *compiler) registerGlobalSimpleType(q QName, typ simpleType) (SimpleTypeID, error) {
	id, err := c.addSimpleType(typ)
	if err != nil {
		return NoSimpleType, err
	}
	c.rt.SimpleTypes[id].Scope = declarationScopeGlobal
	c.rt.GlobalTypes[q] = SimpleRef(id)
	return id, nil
}

func (c *compiler) registerGlobalIdentity(q QName, identity identityConstraintSpec) (IdentityConstraintID, error) {
	id, err := nextIdentityConstraintID(len(c.rt.Identities))
	if err != nil {
		return NoIdentityConstraint, err
	}
	c.rt.Identities = append(c.rt.Identities, identity)
	c.rt.GlobalIdentities[q] = id
	return id, nil
}

func (c *compiler) addElement(decl elementDecl) (ElementID, error) {
	id, err := nextElementID(len(c.rt.Elements))
	if err != nil {
		return NoElement, err
	}
	decl.Scope = declarationScopeNonGlobal
	c.rt.Elements = append(c.rt.Elements, decl)
	return id, nil
}

func (c *compiler) completeElement(id ElementID, decl elementDecl) {
	decl.Scope = c.rt.Elements[id].Scope
	c.rt.Elements[id] = decl
}

func (c *compiler) addComplexType(typ complexTypeMetadata) (ComplexTypeID, error) {
	id, err := nextComplexTypeID(len(c.rt.ComplexTypes))
	if err != nil {
		return noComplexType, err
	}
	typ.Scope = declarationScopeNonGlobal
	c.rt.ComplexTypes = append(c.rt.ComplexTypes, typ)
	c.complexDefinitions = append(c.complexDefinitions, complexTypeDefinition{})
	return id, nil
}

func (c *compiler) completeComplexType(id ComplexTypeID, typ complexTypeMetadata) {
	typ.Scope = c.rt.ComplexTypes[id].Scope
	c.rt.ComplexTypes[id] = typ
}

func (c *compiler) addSimpleType(typ simpleType) (SimpleTypeID, error) {
	id, err := nextSimpleTypeID(len(c.rt.SimpleTypes))
	if err != nil {
		return NoSimpleType, err
	}
	if id >= value.BuiltinTypeCount {
		reserved, reserveErr := c.ensureValueBuilder().Reserve()
		if reserveErr != nil {
			return NoSimpleType, schemaValueError(c.limits, xsderrors.CodeSchemaFacet, "simple type", reserveErr)
		}
		if reserved != id {
			return NoSimpleType, xsderrors.InternalInvariant("schema and value type IDs diverged")
		}
	}
	typ.Scope = declarationScopeNonGlobal
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, typ)
	unavailable := c.simpleTypeHasMissingDependency(typ)
	c.rt.simpleTypeUnavailable = append(c.rt.simpleTypeUnavailable, unavailable)
	return id, nil
}

func (c *compiler) completeSimpleType(id SimpleTypeID, typ simpleType) error {
	typ.Scope = c.rt.SimpleTypes[id].Scope
	c.rt.SimpleTypes[id] = typ
	unavailable := c.simpleTypeHasMissingDependency(typ)
	c.rt.simpleTypeUnavailable[id] = unavailable
	return c.completeValueType(id, typ)
}

func (c *compiler) simpleTypeHasMissingDependency(typ simpleType) bool {
	if typ.Missing {
		return true
	}
	base := typ.ValueSpec.Base
	if base != NoSimpleType && validSimpleTypeID(base, len(c.rt.simpleTypeUnavailable)) && c.rt.simpleTypeUnavailable[base] {
		return true
	}
	if typ.ValueSpec.Variety == value.List && validSimpleTypeID(typ.ValueSpec.ListItem, len(c.rt.simpleTypeUnavailable)) && c.rt.simpleTypeUnavailable[typ.ValueSpec.ListItem] {
		return true
	}
	for _, member := range typ.ValueSpec.Union {
		if validSimpleTypeID(member, len(c.rt.simpleTypeUnavailable)) && c.rt.simpleTypeUnavailable[member] {
			return true
		}
	}
	return false
}

func (c *compiler) completeIdentity(id IdentityConstraintID, identity identityConstraintSpec) {
	c.rt.Identities[id] = identity
}

// Admission charges MaxSchemaDependencySteps for every retained record: each
// particle appended to a content model costs one step, charged before the
// append by appendModelParticles; admitting a model record costs one step,
// an attribute-use set len(Uses) and a wildcard len(Namespaces). An admitted
// model is immutable and reads share it. A new retained table must charge in
// its admission function.
// These functions return unlocated budget errors; enterComponent's leave locates
// them at the component being compiled.
func (c *compiler) appendWildcard(wildcard wildcardRuntime) (WildcardID, error) {
	id, err := nextWildcardID(len(c.rt.Wildcards))
	if err != nil {
		return NoWildcard, err
	}
	if err := c.dependencyWork.spend(len(wildcard.Namespaces)); err != nil {
		return NoWildcard, err
	}
	c.rt.Wildcards = append(c.rt.Wildcards, wildcard)
	return id, nil
}

func (c *compiler) addAttributeUseSet(set attributeUseSet) (attributeUseSetID, error) {
	id, err := nextAttributeUseSetID(len(c.rt.AttributeUseSets))
	if err != nil {
		return noAttributeUseSet, err
	}
	if err := c.dependencyWork.spend(len(set.Uses)); err != nil {
		return noAttributeUseSet, err
	}
	c.rt.AttributeUseSets = append(c.rt.AttributeUseSets, set)
	return id, nil
}

func (c *compiler) addModel(model contentModelNode) (contentModelID, error) {
	id, err := nextContentModelID(len(c.rt.Models))
	if err != nil {
		return noContentModel, err
	}
	if err := c.dependencyWork.spend(1); err != nil {
		return noContentModel, err
	}
	model.Particles = slices.Clip(model.Particles)
	c.rt.Models = append(c.rt.Models, model)
	c.modelSources = append(c.modelSources, nil)
	return id, nil
}

func (c *compiler) addModelAt(model contentModelNode, source *schemaNode) (contentModelID, error) {
	id, err := c.addModel(model)
	if err == nil {
		c.modelSources[id] = source
	}
	return id, err
}

// completeModel installs particles that appendModelParticles already charged;
// with addModel it is the only writer of the model table.
func (c *compiler) completeModel(id contentModelID, model contentModelNode) {
	model.Particles = slices.Clip(model.Particles)
	c.rt.Models[id] = model
}

func (c *compiler) installCompiledModels(models []compiledModel) error {
	if len(models) != len(c.rt.Models) {
		return xsderrors.InternalInvariant("compiled content model count does not match source models")
	}
	c.rt.CompiledModels = models
	return nil
}

func (c *compiler) installFinalizedElements(elements []elementDecl, substitutions substitutionTable) {
	c.rt.Elements = elements
	c.rt.Substitutions = substitutions
}

func (c *compiler) indexGlobalAttribute(q QName, component schemaComponent, label string) error {
	if err := c.validateAttributeDeclNameBuild(q); err != nil {
		return invalidAttributeError(err)
	}
	return addSchemaComponent(c.attributeComponents, q, component, label)
}

func (c *compiler) addNotation(q QName, label string) error {
	return addNotation(c.rt.Notations, q, label)
}

func (c *compiler) registerBuiltinAnyType(q QName, typ complexTypeMetadata) (ComplexTypeID, error) {
	id, err := c.registerGlobalComplexType(q, typ)
	if err != nil {
		return noComplexType, err
	}
	c.rt.AnyType = id
	c.complexDefinitions[id].state = complexTypeComplete
	return id, nil
}

func (c *compiler) publishSchema() (*Schema, error) {
	if len(c.deferredModelChildren) != 0 {
		return nil, xsderrors.InternalInvariant("omitted compositor descendants were not completed")
	}
	for _, definition := range c.complexDefinitions {
		if definition.state != complexTypeComplete {
			return nil, xsderrors.InternalInvariant("complex type definitions were not completed")
		}
	}
	if len(c.pendingElementConstraints) != 0 {
		return nil, xsderrors.InternalInvariant("element value constraints were not finalized")
	}
	published, err := publishSchema(&c.rt, c.contentWork.spend)
	if err != nil {
		return nil, err
	}
	return published, nil
}
