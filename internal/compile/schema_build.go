package compile

import (
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

type compilerSchemaBuild struct {
	build runtime.SchemaBuild
}

func newCompilerSchemaBuild(names runtime.NameTable) compilerSchemaBuild {
	return compilerSchemaBuild{build: runtime.SchemaBuild{
		Names:            names,
		GlobalElements:   make(map[runtime.QName]runtime.ElementID),
		GlobalAttributes: make(map[runtime.QName]runtime.AttributeID, runtime.BuiltinAttributeCount()),
		GlobalTypes:      make(map[runtime.QName]runtime.TypeID, runtime.BuiltinGlobalTypeCount()),
		GlobalIdentities: make(map[runtime.QName]runtime.IdentityConstraintID),
		Notations:        make(map[runtime.QName]bool),
		SimpleTypes:      make([]runtime.SimpleType, 0, runtime.BuiltinSimpleTypeCount()),
		Attributes:       make([]runtime.AttributeDecl, 0, runtime.BuiltinAttributeCount()),
		ComplexTypes:     make([]runtime.ComplexType, 0, runtime.BuiltinComplexTypeCount()),
		Wildcards:        make([]runtime.Wildcard, 0, 1),
		AttributeUseSets: make([]runtime.AttributeUseSet, 0, 1),
		Models:           make([]runtime.ContentModel, 0, 1),
	}}
}

func (rt *compilerSchemaBuild) TypeName(t runtime.TypeID) runtime.QName {
	return rt.build.TypeName(t)
}

func (rt *compilerSchemaBuild) AnyTypeID() runtime.ComplexTypeID {
	return rt.build.AnyTypeID()
}

func (rt *compilerSchemaBuild) ComplexTypeCount() int {
	return rt.build.ComplexTypeCount()
}

func (rt *compilerSchemaBuild) SimpleTypeCount() int {
	return rt.build.SimpleTypeCount()
}

func (rt *compilerSchemaBuild) SimpleTypeFinal(id runtime.SimpleTypeID) (runtime.DerivationMask, bool) {
	return rt.build.SimpleTypeFinal(id)
}

func (rt *compilerSchemaBuild) SimpleTypeDerivation(id runtime.SimpleTypeID) (runtime.SimpleTypeDerivation, bool) {
	return rt.build.SimpleTypeDerivation(id)
}

func (rt *compilerSchemaBuild) ComplexTypeDerivation(id runtime.ComplexTypeID) (runtime.ComplexTypeDerivation, bool) {
	return rt.build.ComplexTypeDerivation(id)
}

func (rt *compilerSchemaBuild) ContentModel(id runtime.ContentModelID) (runtime.ContentModel, bool) {
	return rt.build.ContentModel(id)
}

func (rt *compilerSchemaBuild) ElementName(id runtime.ElementID) (runtime.QName, bool) {
	return rt.build.ElementName(id)
}

func (rt *compilerSchemaBuild) ElementType(id runtime.ElementID) (runtime.TypeID, bool) {
	return rt.build.ElementType(id)
}

func (rt *compilerSchemaBuild) ElementRestriction(id runtime.ElementID) (runtime.ParticleRestrictionElement, bool) {
	return rt.build.ElementRestriction(id)
}

func (rt *compilerSchemaBuild) Wildcard(id runtime.WildcardID) (runtime.Wildcard, bool) {
	return rt.build.Wildcard(id)
}

func (rt *compilerSchemaBuild) ForEachSubstitutionMember(id runtime.ElementID, fn func(runtime.ElementID) bool) {
	rt.build.ForEachSubstitutionMember(id, fn)
}

func (rt *compilerSchemaBuild) ForEachSubstitutionEntry(id runtime.ElementID, fn func(runtime.QName, runtime.ElementID) bool) {
	rt.build.ForEachSubstitutionEntry(id, fn)
}

func (rt *compilerSchemaBuild) HasSubstitutionMembers(id runtime.ElementID) bool {
	return rt.build.HasSubstitutionMembers(id)
}

func (rt *compilerSchemaBuild) SubstitutionMemberByName(id runtime.ElementID, name runtime.QName) (runtime.ElementID, bool) {
	return rt.build.SubstitutionMemberByName(id, name)
}

func (rt *compilerSchemaBuild) TypeLabel(t runtime.TypeID) string {
	return rt.build.TypeLabel(t)
}

func (rt *compilerSchemaBuild) StringEnumerationContains(id runtime.SimpleTypeID, canonical string) (bool, bool) {
	return rt.build.StringEnumerationContains(id, canonical)
}

func (rt *compilerSchemaBuild) SimpleTypeIdentity(id runtime.SimpleTypeID) (runtime.SimpleIdentityKind, bool) {
	return rt.build.SimpleTypeIdentity(id)
}

func (rt *compilerSchemaBuild) DerivedSimpleIdentity(st runtime.SimpleType) runtime.SimpleIdentityKind {
	return rt.build.DerivedSimpleIdentity(st)
}

func (rt *compilerSchemaBuild) ValueConstraintSimpleType(id runtime.SimpleTypeID) (runtime.ValueConstraintSimpleType, bool) {
	return rt.build.ValueConstraintSimpleType(id)
}

func (rt *compilerSchemaBuild) ValueConstraintComplexType(id runtime.ComplexTypeID) (runtime.ValueConstraintComplexType, bool) {
	return rt.build.ValueConstraintComplexType(id)
}

func (rt *compilerSchemaBuild) builtinIDs() runtime.BuiltinIDs {
	return rt.build.Builtin
}

func (rt *compilerSchemaBuild) formatName(q runtime.QName) string {
	return rt.build.Names.Format(q)
}

func (rt *compilerSchemaBuild) namespaceURI(id runtime.NamespaceID) string {
	return rt.build.Names.Namespace(id)
}

func (rt *compilerSchemaBuild) lookupQName(ns, local string) (runtime.QName, bool) {
	return rt.build.Names.LookupQName(ns, local)
}

func (rt *compilerSchemaBuild) complexType(id runtime.ComplexTypeID) runtime.ComplexType {
	return rt.build.ComplexTypes[id]
}

func (rt *compilerSchemaBuild) notationDeclared(q runtime.QName) bool {
	return rt.build.Notations[q]
}

func sortedBuildQNames[T any](rt *compilerSchemaBuild, values map[runtime.QName]T) []runtime.QName {
	return SortedQNames(values, rt.build.Names)
}

func (rt *compilerSchemaBuild) InternNamespace(uri string) (runtime.NamespaceID, error) {
	return NewNameInterner(&rt.build.Names).InternNamespace(uri)
}

func (rt *compilerSchemaBuild) internQName(ns, local string) (runtime.QName, error) {
	return NewNameInterner(&rt.build.Names).InternQName(ns, local)
}

func (rt *compilerSchemaBuild) simpleTypeFinal(id runtime.SimpleTypeID) runtime.DerivationMask {
	return rt.build.SimpleTypes[id].Final
}

func (rt *compilerSchemaBuild) simpleTypeWhitespace(id runtime.SimpleTypeID) runtime.WhitespaceMode {
	return rt.build.SimpleTypes[id].Whitespace
}

func (rt *compilerSchemaBuild) appendFlattenedUnionMember(
	dst *[]runtime.SimpleTypeID,
	id runtime.SimpleTypeID,
	seen map[runtime.SimpleTypeID]struct{},
	remaining int,
) (int, bool) {
	typ := rt.build.SimpleTypes[id]
	members := []runtime.SimpleTypeID{id}
	if typ.Variety == runtime.SimpleVarietyUnion {
		members = typ.Union
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
func (rt *compilerSchemaBuild) derivedSimpleType(id runtime.SimpleTypeID, name runtime.QName) runtime.SimpleType {
	st := rt.build.SimpleTypes[id]
	st.Name = name
	st.Base = id
	st.UnionSources = nil
	st.Final = 0
	return st
}

func (rt *compilerSchemaBuild) simpleValueType(id runtime.SimpleTypeID) (runtime.SimpleValueType, bool) {
	if !runtime.ValidSimpleTypeID(id, len(rt.build.SimpleTypes)) || rt.build.SimpleTypes[id].Missing {
		return runtime.SimpleValueType{}, false
	}
	typ := runtime.SimpleValueTypeForSimpleType(rt.build.SimpleTypes[id])
	typ.UnionMembers = slices.Clone(typ.UnionMembers)
	return typ, true
}

func (c *compiler) validateCompiledFacetsBuild(st runtime.SimpleType, base runtime.SimpleTypeID, step runtime.OrderedFacetStep) error {
	return ValidateCompiledFacets(st, c.rt.build.SimpleTypes[base], step)
}

func (c *compiler) compiledLiteralForSimpleType(id runtime.SimpleTypeID, lexical, canonical string, resolvedNames []runtime.ResolvedValueName) runtime.CompiledLiteral {
	return runtime.NewCompiledLiteralForSimpleType(c.rt.build.SimpleTypes[id], id, lexical, canonical, resolvedNames)
}

func cloneValueConstraint(vc *runtime.ValueConstraint) *runtime.ValueConstraint {
	if vc == nil {
		return nil
	}
	cloned := *vc
	cloned.ResolvedNames = slices.Clone(vc.ResolvedNames)
	return &cloned
}

func (rt *compilerSchemaBuild) elementCopy(id runtime.ElementID) runtime.ElementDecl {
	decl := rt.build.Elements[id]
	decl.Identity = slices.Clone(decl.Identity)
	decl.Default = cloneValueConstraint(decl.Default)
	decl.Fixed = cloneValueConstraint(decl.Fixed)
	return decl
}

func (c *compiler) elementCopies() []runtime.ElementDecl {
	return slices.Clone(c.rt.build.Elements)
}

func (rt *compilerSchemaBuild) buildSubstitutionTable(elements []runtime.ElementDecl, maxEntries int) (runtime.SubstitutionTable, error) {
	return runtime.BuildSubstitutionTable(
		&rt.build,
		&rt.build.Names,
		elements,
		rt.build.GlobalElements,
		maxEntries,
	)
}

func (rt *compilerSchemaBuild) attributeUse(id runtime.AttributeID) runtime.AttributeUse {
	decl := rt.build.Attributes[id]
	return runtime.AttributeUse{
		Name:                 decl.Name,
		Type:                 decl.Type,
		Default:              cloneValueConstraint(decl.Default),
		Fixed:                cloneValueConstraint(decl.Fixed),
		FixedFromDeclaration: decl.Fixed != nil,
	}
}

func (rt *compilerSchemaBuild) attributeUsesAndWildcard(id runtime.AttributeUseSetID) ([]runtime.AttributeUse, runtime.WildcardID) {
	if id == runtime.NoAttributeUseSet {
		return nil, runtime.NoWildcard
	}
	set := rt.build.AttributeUseSets[id]
	uses := slices.Clone(set.Uses)
	for i := range uses {
		uses[i].Default = cloneValueConstraint(uses[i].Default)
		uses[i].Fixed = cloneValueConstraint(uses[i].Fixed)
	}
	return uses, set.Wildcard
}

func (rt *compilerSchemaBuild) identityName(id runtime.IdentityConstraintID) runtime.QName {
	return rt.build.Identities[id].Name
}

func (c *compiler) readSimpleFacets(id runtime.SimpleTypeID) (runtime.SimpleValueFacets, bool) {
	return c.simpleFacetCache.read(c.rt.build.SimpleTypes, id)
}

func (c *compiler) checkIdentityConstraintNameAvailable(q runtime.QName) error {
	return CheckIdentityConstraintNameAvailable(c.rt.build.GlobalIdentities, q, c.rt.build.Names.Format(q))
}

func (c *compiler) resolveIdentityConstraintRefer(q runtime.QName) (runtime.IdentityConstraintID, error) {
	return ResolveIdentityConstraintRefer(c.rt.build.GlobalIdentities, q, c.rt.build.Names.Format(q))
}

func (c *compiler) validateIdentityReferencesBuild() error {
	return ValidateIdentityReferences(c.rt.build.Identities)
}

func (c *compiler) compileContentModelsBuild() ([]runtime.CompiledModel, error) {
	return CompileContentModels(
		c.ctx,
		&c.rt.build.Names,
		&c.rt,
		len(c.rt.build.Models),
		c.limits.MaxContentModelStates,
	)
}

func (c *compiler) checkContentModelsUPABuild() error {
	return CheckContentModelsUPA(c.ctx, &c.rt.build.Names, &c.rt, len(c.rt.build.Models))
}

func (c *compiler) checkContentModelElementDeclarationsConsistentBuild() error {
	if len(c.modelSources) != len(c.rt.build.Models) {
		return xsderrors.InternalInvariant("content model provenance count does not match model count")
	}
	for id, model := range c.rt.build.Models {
		if err := compileContextError(c.ctx); err != nil {
			return err
		}
		if err := CheckElementDeclarationsConsistent(&c.rt, model); err != nil {
			if c.modelSources[id] != nil {
				return withSchemaCompileLocation(c.modelSources[id], err)
			}
			return err
		}
	}
	return nil
}

func (c *compiler) restrictionChoiceLimitUpdates() ([]runtime.RestrictionChoiceLimitUpdate, error) {
	return runtime.RestrictionChoiceLimitUpdates(
		&c.rt,
		c.rt.build.ComplexTypes,
		c.rt.build.Models,
		c.rt.build.Builtin.AnyType,
	)
}

func (c *compiler) validateAttributeDeclNameBuild(q runtime.QName) error {
	return runtime.ValidateAttributeDeclName(&c.rt.build.Names, q)
}

func (c *compiler) validateAttributeUseSetBuild(set runtime.AttributeUseSet) error {
	return runtime.ValidateAttributeUseSetRecord(&c.rt.build.Names, &c.rt, set)
}

func (c *compiler) simpleListItemReachesList(id runtime.SimpleTypeID) bool {
	return c.simpleListReach.reachesList(c.rt.build.SimpleTypes, id)
}

func (c *compiler) registerGlobalElement(q runtime.QName, decl runtime.ElementDecl) (runtime.ElementID, error) {
	id, err := c.addElement(decl)
	if err != nil {
		return runtime.NoElement, err
	}
	c.rt.build.Elements[id].Scope = runtime.DeclarationScopeGlobal
	c.rt.build.GlobalElements[q] = id
	return id, nil
}

func (c *compiler) registerGlobalAttribute(q runtime.QName, decl runtime.AttributeDecl) (runtime.AttributeID, error) {
	id, err := NextAttributeID(len(c.rt.build.Attributes))
	if err != nil {
		return 0, err
	}
	c.rt.build.Attributes = append(c.rt.build.Attributes, decl)
	c.rt.build.GlobalAttributes[q] = id
	return id, nil
}

func (c *compiler) registerGlobalComplexType(q runtime.QName, typ runtime.ComplexType) (runtime.ComplexTypeID, error) {
	id, err := c.addComplexType(typ)
	if err != nil {
		return runtime.NoComplexType, err
	}
	c.rt.build.ComplexTypes[id].Scope = runtime.DeclarationScopeGlobal
	c.rt.build.GlobalTypes[q] = runtime.ComplexRef(id)
	return id, nil
}

func (c *compiler) registerGlobalSimpleType(q runtime.QName, typ runtime.SimpleType) (runtime.SimpleTypeID, error) {
	id, err := c.addSimpleType(typ)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	c.rt.build.SimpleTypes[id].Scope = runtime.DeclarationScopeGlobal
	c.rt.build.GlobalTypes[q] = runtime.SimpleRef(id)
	return id, nil
}

func (c *compiler) registerGlobalIdentity(q runtime.QName, identity runtime.IdentityConstraint) (runtime.IdentityConstraintID, error) {
	id, err := NextIdentityConstraintID(len(c.rt.build.Identities))
	if err != nil {
		return runtime.NoIdentityConstraint, err
	}
	c.rt.build.Identities = append(c.rt.build.Identities, identity)
	c.rt.build.GlobalIdentities[q] = id
	return id, nil
}

func (c *compiler) addElement(decl runtime.ElementDecl) (runtime.ElementID, error) {
	id, err := NextElementID(len(c.rt.build.Elements))
	if err != nil {
		return runtime.NoElement, err
	}
	decl.Scope = runtime.DeclarationScopeNonGlobal
	c.rt.build.Elements = append(c.rt.build.Elements, decl)
	return id, nil
}

func (c *compiler) completeElement(id runtime.ElementID, decl runtime.ElementDecl) {
	decl.Scope = c.rt.build.Elements[id].Scope
	c.rt.build.Elements[id] = decl
}

func (c *compiler) addComplexType(typ runtime.ComplexType) (runtime.ComplexTypeID, error) {
	id, err := NextComplexTypeID(len(c.rt.build.ComplexTypes))
	if err != nil {
		return runtime.NoComplexType, err
	}
	typ.Scope = runtime.DeclarationScopeNonGlobal
	c.rt.build.ComplexTypes = append(c.rt.build.ComplexTypes, typ)
	return id, nil
}

func (c *compiler) completeComplexType(id runtime.ComplexTypeID, typ runtime.ComplexType) {
	typ.Scope = c.rt.build.ComplexTypes[id].Scope
	c.rt.build.ComplexTypes[id] = typ
}

func (c *compiler) addSimpleType(typ runtime.SimpleType) (runtime.SimpleTypeID, error) {
	id, err := NextSimpleTypeID(len(c.rt.build.SimpleTypes))
	if err != nil {
		return runtime.NoSimpleType, err
	}
	typ.Scope = runtime.DeclarationScopeNonGlobal
	c.rt.build.SimpleTypes = append(c.rt.build.SimpleTypes, typ)
	c.simpleTypeUnavailable = append(c.simpleTypeUnavailable, c.simpleTypeHasMissingDependency(typ))
	return id, nil
}

func (c *compiler) completeSimpleType(id runtime.SimpleTypeID, typ runtime.SimpleType) {
	typ.Scope = c.rt.build.SimpleTypes[id].Scope
	c.rt.build.SimpleTypes[id] = typ
	c.simpleTypeUnavailable[id] = c.simpleTypeHasMissingDependency(typ)
}

func (c *compiler) simpleTypeHasMissingDependency(typ runtime.SimpleType) bool {
	if typ.Missing {
		return true
	}
	if typ.Base != runtime.NoSimpleType && runtime.ValidSimpleTypeID(typ.Base, len(c.simpleTypeUnavailable)) && c.simpleTypeUnavailable[typ.Base] {
		return true
	}
	if typ.Variety == runtime.SimpleVarietyList && runtime.ValidSimpleTypeID(typ.ListItem, len(c.simpleTypeUnavailable)) && c.simpleTypeUnavailable[typ.ListItem] {
		return true
	}
	for _, member := range typ.Union {
		if runtime.ValidSimpleTypeID(member, len(c.simpleTypeUnavailable)) && c.simpleTypeUnavailable[member] {
			return true
		}
	}
	return false
}

func (c *compiler) completeIdentity(id runtime.IdentityConstraintID, identity runtime.IdentityConstraint) {
	c.rt.build.Identities[id] = identity
}

func (c *compiler) addWildcard(wildcard runtime.Wildcard) (runtime.WildcardID, error) {
	id, err := NextWildcardID(len(c.rt.build.Wildcards))
	if err != nil {
		return runtime.NoWildcard, err
	}
	c.rt.build.Wildcards = append(c.rt.build.Wildcards, wildcard)
	return id, nil
}

func (c *compiler) addAttributeUseSet(set runtime.AttributeUseSet) (runtime.AttributeUseSetID, error) {
	id, err := NextAttributeUseSetID(len(c.rt.build.AttributeUseSets))
	if err != nil {
		return runtime.NoAttributeUseSet, err
	}
	c.rt.build.AttributeUseSets = append(c.rt.build.AttributeUseSets, set)
	return id, nil
}

func (c *compiler) addModel(model runtime.ContentModel) (runtime.ContentModelID, error) {
	id, err := NextContentModelID(len(c.rt.build.Models))
	if err != nil {
		return runtime.NoContentModel, err
	}
	c.rt.build.Models = append(c.rt.build.Models, model)
	c.modelSources = append(c.modelSources, nil)
	return id, nil
}

func (c *compiler) addModelAt(model runtime.ContentModel, source *rawNode) (runtime.ContentModelID, error) {
	id, err := c.addModel(model)
	if err == nil {
		c.modelSources[id] = source
	}
	return id, err
}

func (c *compiler) completeModel(id runtime.ContentModelID, model runtime.ContentModel) {
	c.rt.build.Models[id] = model
}

func (c *compiler) installCompiledModels(models []runtime.CompiledModel) error {
	if len(models) != len(c.rt.build.Models) {
		return xsderrors.InternalInvariant("compiled content model count does not match source models")
	}
	c.rt.build.CompiledModels = models
	return nil
}

func (c *compiler) installFinalizedElements(elements []runtime.ElementDecl, substitutions runtime.SubstitutionTable) {
	c.rt.build.Elements = elements
	c.rt.build.Substitutions = substitutions
}

func (c *compiler) indexGlobalAttribute(q runtime.QName, component rawComponent, label string) error {
	return AddGlobalAttributeComponent(c.attributeRaw, c.rt.build.GlobalAttributes, q, component, label)
}

func (c *compiler) addNotation(q runtime.QName, label string) error {
	return AddNotation(c.rt.build.Notations, q, label)
}

func (c *compiler) registerBuiltinSimpleType(seed *runtime.BuiltinSimpleSeed, q runtime.QName, typ runtime.SimpleType) (runtime.SimpleTypeID, error) {
	id, err := c.registerGlobalSimpleType(q, typ)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	seed.RecordID(&c.rt.build.Builtin, id)
	return id, nil
}

func (c *compiler) registerBuiltinAnyType(q runtime.QName, typ runtime.ComplexType) (runtime.ComplexTypeID, error) {
	id, err := c.registerGlobalComplexType(q, typ)
	if err != nil {
		return runtime.NoComplexType, err
	}
	c.rt.build.Builtin.AnyType = id
	return id, nil
}

func (c *compiler) publishSchema() (*runtime.Schema, error) {
	if len(c.pendingElementConstraints) != 0 {
		return nil, xsderrors.InternalInvariant("element value constraints were not finalized")
	}
	published, err := runtime.PublishSchema(c.ctx, &c.rt.build)
	if err != nil {
		return nil, err
	}
	return published, nil
}
