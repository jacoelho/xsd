package runtime

import (
	"errors"
	"maps"
	"reflect"
	"slices"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// schemaAudit joins compiler-owned source records with a candidate published
// runtime only while publication invariants are checked. Schema never retains
// this source after publication.
type schemaAudit struct {
	contentModelWork ContentModelWork
	contentAnalysis  *ContentModelAnalysis
	Schema
	build SchemaBuild
}

func validateSchema(rt *schemaAudit) error {
	ctx := schemaValidationContext{rt: rt}
	if err := validateNameTable(rt); err != nil {
		return err
	}
	if err := validateRuntimeGlobals(rt); err != nil {
		return err
	}
	if err := validateRuntimeSubstitutions(rt); err != nil {
		return err
	}
	if err := validateBuiltinIDs(&ctx); err != nil {
		return err
	}
	if err := validateRuntimeComponents(&ctx); err != nil {
		return err
	}
	if err := validateRuntimeChoiceLimits(rt); err != nil {
		return err
	}
	return validateRuntimeCompiledModels(rt)
}

type schemaValidationContext struct {
	rt                     *schemaAudit
	validatedBoundLiterals map[*CompiledLiteral]struct{}
}

func validateSchemaBuildOwnership(build *SchemaBuild) error {
	if err := validateAttributeBuildOwnership(build); err != nil {
		return err
	}
	if err := validateElementBuildOwnership(build); err != nil {
		return err
	}
	if err := validateSimpleTypeBuildOwnership(build); err != nil {
		return err
	}
	if err := validateComplexTypeBuildOwnership(build); err != nil {
		return err
	}
	if err := validateIdentityBuildOwnership(build); err != nil {
		return err
	}
	if err := validateIdentityConstraintOwnership(build.Elements, len(build.Identities)); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateAttributeBuildOwnership(build *SchemaBuild) error {
	for i, decl := range build.Attributes {
		id, ok := build.GlobalAttributes[decl.Name]
		if !ok || id != AttributeID(i) {
			return xsderrors.InternalInvariant("global attribute declaration is missing its exact name binding")
		}
	}
	return nil
}

func validateElementBuildOwnership(build *SchemaBuild) error {
	for i, decl := range build.Elements {
		id, bound := build.GlobalElements[decl.Name]
		exact := bound && id == ElementID(i)
		switch decl.Scope {
		case DeclarationScopeGlobal:
			if !exact {
				return xsderrors.InternalInvariant("global element declaration is missing its exact name binding")
			}
		case DeclarationScopeNonGlobal:
			if exact {
				return xsderrors.InternalInvariant("non-global element declaration has a global name binding")
			}
		default:
			return xsderrors.InternalInvariant("element declaration scope is invalid")
		}
	}
	return nil
}

func validateSimpleTypeBuildOwnership(build *SchemaBuild) error {
	for i, typ := range build.SimpleTypes {
		id, bound := build.GlobalTypes[typ.Name]
		exact := bound && id == SimpleRef(SimpleTypeID(i))
		switch typ.Scope {
		case DeclarationScopeGlobal:
			if !exact {
				return xsderrors.InternalInvariant("global simple type is missing its exact name binding")
			}
		case DeclarationScopeNonGlobal:
			if exact {
				return xsderrors.InternalInvariant("non-global simple type has a global name binding")
			}
		default:
			return xsderrors.InternalInvariant("simple type declaration scope is invalid")
		}
	}
	return nil
}

func validateComplexTypeBuildOwnership(build *SchemaBuild) error {
	for i, typ := range build.ComplexTypes {
		id, bound := build.GlobalTypes[typ.Name]
		exact := bound && id == ComplexRef(ComplexTypeID(i))
		switch typ.Scope {
		case DeclarationScopeGlobal:
			if !exact {
				return xsderrors.InternalInvariant("global complex type is missing its exact name binding")
			}
		case DeclarationScopeNonGlobal:
			if exact {
				return xsderrors.InternalInvariant("non-global complex type has a global name binding")
			}
		default:
			return xsderrors.InternalInvariant("complex type declaration scope is invalid")
		}
	}
	return nil
}

func validateIdentityBuildOwnership(build *SchemaBuild) error {
	for i, identity := range build.Identities {
		id, ok := build.GlobalIdentities[identity.Name]
		if !ok || id != IdentityConstraintID(i) {
			return xsderrors.InternalInvariant("identity constraint is missing its exact name binding")
		}
	}
	return nil
}

func validateNameTable(rt *schemaAudit) error {
	if err := ValidateRuntimeNameTable(&rt.build.Names); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeGlobals(rt *schemaAudit) error {
	if rt == nil {
		return xsderrors.InternalInvariant("runtime globals require schema")
	}
	if err := validateGlobalAttributes(rt); err != nil {
		return err
	}
	if err := validateGlobalElements(rt); err != nil {
		return err
	}
	if err := validateGlobalTypes(rt); err != nil {
		return err
	}
	if err := validateGlobalIdentities(rt); err != nil {
		return err
	}
	return validateGlobalNotations(rt)
}

func validateGlobalAttributes(rt *schemaAudit) error {
	for q, id := range rt.build.GlobalAttributes {
		if !rt.build.Names.ValidQName(q) || !ValidAttributeID(id, len(rt.build.Attributes)) {
			return xsderrors.InternalInvariant("global attribute references invalid declaration")
		}
		if rt.build.Attributes[id].Name != q {
			return xsderrors.InternalInvariant("global attribute name does not match declaration")
		}
	}
	return nil
}

func validateGlobalElements(rt *schemaAudit) error {
	for q, id := range rt.build.GlobalElements {
		if !rt.build.Names.ValidQName(q) || !ValidElementID(id, len(rt.build.Elements)) {
			return xsderrors.InternalInvariant("global element references invalid declaration")
		}
		if rt.build.Elements[id].Name != q {
			return xsderrors.InternalInvariant("global element name does not match declaration")
		}
	}
	return nil
}

func validateGlobalTypes(rt *schemaAudit) error {
	for q, typ := range rt.build.GlobalTypes {
		name, ok := TypeNameByID(rt.build.SimpleTypes, rt.build.ComplexTypes, typ)
		if !rt.build.Names.ValidQName(q) || !ok {
			return xsderrors.InternalInvariant("global type references invalid declaration")
		}
		if name != q {
			return xsderrors.InternalInvariant("global type name does not match declaration")
		}
	}
	return nil
}

func validateGlobalIdentities(rt *schemaAudit) error {
	for q, id := range rt.build.GlobalIdentities {
		if !rt.build.Names.ValidQName(q) || !ValidIdentityConstraintID(id, len(rt.build.Identities)) {
			return xsderrors.InternalInvariant("global identity references invalid declaration")
		}
		if rt.build.Identities[id].Name != q {
			return xsderrors.InternalInvariant("global identity name does not match declaration")
		}
	}
	return nil
}

func validateGlobalNotations(rt *schemaAudit) error {
	for q := range rt.build.Notations {
		if !rt.build.Names.ValidQName(q) {
			return xsderrors.InternalInvariant("notation references invalid name")
		}
	}
	return nil
}

func validateRuntimeSubstitutions(rt *schemaAudit) error {
	if err := ValidateSubstitutionTable(
		&rt.build,
		&rt.build.Names,
		rt.build.Elements,
		rt.build.GlobalElements,
		rt.build.Substitutions,
	); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeComponents(ctx *schemaValidationContext) error {
	if err := validateRuntimeSimpleComponents(ctx); err != nil {
		return err
	}
	if err := validateRuntimeDeclarationComponents(ctx); err != nil {
		return err
	}
	return validateRuntimeComplexComponents(ctx)
}

func validateRuntimeSimpleComponents(ctx *schemaValidationContext) error {
	rt := ctx.rt
	if err := validateMissingSimpleType(rt); err != nil {
		return err
	}
	for i := range rt.build.SimpleTypes {
		if err := validateSimpleType(ctx, SimpleTypeID(i), rt.build.SimpleTypes[i]); err != nil {
			return err
		}
	}
	if err := validateSimpleTypeGraph(rt); err != nil {
		return err
	}
	return validateCompiledFacetLiterals(ctx)
}

func validateRuntimeDeclarationComponents(ctx *schemaValidationContext) error {
	if err := validateRuntimeElementAndAttributeComponents(ctx); err != nil {
		return err
	}
	return validateRuntimeContentModelComponents(ctx.rt)
}

func validateRuntimeElementAndAttributeComponents(ctx *schemaValidationContext) error {
	if err := validateRuntimeElementAndAttributeRecords(ctx); err != nil {
		return err
	}
	if err := validateRuntimeWildcards(ctx.rt); err != nil {
		return err
	}
	return validateRuntimeAttributeUseSets(ctx)
}

func validateRuntimeElementAndAttributeRecords(ctx *schemaValidationContext) error {
	rt := ctx.rt
	for i := range rt.build.Elements {
		if err := validateElementDeclShape(rt, rt.build.Elements[i]); err != nil {
			return err
		}
	}
	for i := range rt.build.Attributes {
		if err := validateAttributeDecl(ctx, rt.build.Attributes[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeWildcards(rt *schemaAudit) error {
	for i := range rt.build.Wildcards {
		if err := ValidateWildcard(&rt.build.Names, rt.build.Wildcards[i]); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
	}
	return nil
}

func validateRuntimeAttributeUseSets(ctx *schemaValidationContext) error {
	rt := ctx.rt
	for i := range rt.build.AttributeUseSets {
		if err := validateAttributeUseSetRuntime(ctx, AttributeUseSetID(i), rt.build.AttributeUseSets[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeContentModelComponents(rt *schemaAudit) error {
	contentModelLimits := ContentModelRefLimits{
		ElementCount:      len(rt.build.Elements),
		ContentModelCount: len(rt.build.Models),
		WildcardCount:     len(rt.build.Wildcards),
	}
	for i := range rt.build.Models {
		if err := validateRuntimeContentModel(rt, rt.build.Models[i], contentModelLimits); err != nil {
			return err
		}
	}
	if err := validateContentModelGraph(rt.build.Models); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeContentModel(rt *schemaAudit, model ContentModel, limits ContentModelRefLimits) error {
	if err := spendContentModelWork(rt.contentModelWork); err != nil {
		return err
	}
	for range model.Particles {
		if err := spendContentModelWork(rt.contentModelWork); err != nil {
			return err
		}
	}
	if err := ValidateContentModelRuntime(model, limits); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeComplexComponents(ctx *schemaValidationContext) error {
	rt := ctx.rt
	if err := validateRuntimeComplexTypeRecords(rt); err != nil {
		return err
	}
	if err := validateRuntimeComplexTypeDerivations(rt); err != nil {
		return err
	}
	if err := ValidateIdentityConstraints(&rt.build.Names, rt.build.Identities); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return validateRuntimeElementValueConstraints(ctx)
}

func validateRuntimeComplexTypeRecords(rt *schemaAudit) error {
	for i := range rt.build.ComplexTypes {
		if err := validateComplexTypeRecord(&rt.build, ComplexTypeID(i), rt.build.ComplexTypes[i]); err != nil {
			return err
		}
	}
	if err := validateComplexTypeGraph(rt.build.ComplexTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeComplexTypeDerivations(rt *schemaAudit) error {
	for i := range rt.build.ComplexTypes {
		if err := validateComplexTypeDerivation(rt, ComplexTypeID(i), rt.build.ComplexTypes[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeElementValueConstraints(ctx *schemaValidationContext) error {
	rt := ctx.rt
	for i := range rt.build.Elements {
		if err := validateElementDeclValueConstraints(ctx, rt.build.Elements[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateMissingSimpleType(rt *schemaAudit) error {
	missingID, err := missingSimpleTypeID(rt)
	if err != nil || missingID == NoSimpleType {
		return err
	}
	if err := validateMissingSimpleTypeReferences(rt, missingID); err != nil {
		return err
	}
	return validateMissingSimpleTypeGlobals(rt, missingID)
}

func missingSimpleTypeID(rt *schemaAudit) (SimpleTypeID, error) {
	missingName, hasMissingName := rt.build.Names.LookupQName(vocab.EmptyNamespaceURI, MissingSimpleTypeLocalName())
	missingID := NoSimpleType
	for i := range rt.build.SimpleTypes {
		st := rt.build.SimpleTypes[i]
		if !st.Missing {
			continue
		}
		expected := MissingSimpleType(missingName, rt.build.Builtin.AnySimpleType)
		expected.Scope = DeclarationScopeNonGlobal
		if missingID != NoSimpleType || !hasMissingName || st.Name != missingName || !reflect.DeepEqual(st, expected) {
			return NoSimpleType, xsderrors.InternalInvariant("missing simple type sentinel is invalid")
		}
		missingID = SimpleTypeID(i)
	}
	return missingID, nil
}

func validateMissingSimpleTypeReferences(rt *schemaAudit, missingID SimpleTypeID) error {
	for _, st := range rt.build.SimpleTypes {
		if !st.Missing && st.Base == missingID {
			return xsderrors.InternalInvariant("missing simple type sentinel has invalid type reference")
		}
	}
	return validateMissingSimpleTypeComplexReferences(rt, missingID)
}

func validateMissingSimpleTypeComplexReferences(rt *schemaAudit, missingID SimpleTypeID) error {
	for _, ct := range rt.build.ComplexTypes {
		base, simpleBase := ct.Base.Simple()
		if ct.TextType == missingID || (simpleBase && base == missingID) {
			return xsderrors.InternalInvariant("missing simple type sentinel has invalid complex type reference")
		}
	}
	return nil
}

func validateMissingSimpleTypeGlobals(rt *schemaAudit, missingID SimpleTypeID) error {
	for _, typ := range rt.build.GlobalTypes {
		if simple, ok := typ.Simple(); ok && simple == missingID {
			return xsderrors.InternalInvariant("missing simple type sentinel is globally registered")
		}
	}
	return nil
}

func validateRuntimeReadProjections(rt *schemaAudit) error {
	if err := validatePrimaryRuntimeReadProjections(rt); err != nil {
		return err
	}
	return validateSecondaryRuntimeReadProjections(rt)
}

func validatePrimaryRuntimeReadProjections(rt *schemaAudit) error {
	if err := validateGlobalReadProjections(rt); err != nil {
		return err
	}
	if err := validateAttributeDeclReads(rt); err != nil {
		return err
	}
	if err := validateNameReads(rt); err != nil {
		return err
	}
	if err := validateNotationReads(rt); err != nil {
		return err
	}
	if err := validateElementReads(rt); err != nil {
		return err
	}
	if err := validateTypeDerivations(rt); err != nil {
		return err
	}
	return validateSimpleValueReads(rt)
}

func validateSecondaryRuntimeReadProjections(rt *schemaAudit) error {
	if err := validateAttributeUseSetReads(rt); err != nil {
		return err
	}
	if err := validateIdentityConstraintReads(rt); err != nil {
		return err
	}
	if err := validateComplexTypeReads(rt); err != nil {
		return err
	}
	if err := validateWildcardReads(rt); err != nil {
		return err
	}
	return validateCompiledModelReads(rt)
}

func validateGlobalReadProjections(rt *schemaAudit) error {
	if !maps.Equal(rt.runtime.GlobalAttributes, rt.build.GlobalAttributes) {
		return xsderrors.InternalInvariant("global attribute read projection does not match build")
	}
	if !maps.Equal(rt.runtime.GlobalElements, rt.build.GlobalElements) {
		return xsderrors.InternalInvariant("global element read projection does not match build")
	}
	if !maps.Equal(rt.runtime.GlobalTypes, rt.build.GlobalTypes) {
		return xsderrors.InternalInvariant("global type read projection does not match build")
	}
	return nil
}

func validateAttributeDeclReads(rt *schemaAudit) error {
	if err := ValidateAttributeDeclReadProjectionForDecls(rt.runtime.Attributes, rt.build.Attributes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateNameReads(rt *schemaAudit) error {
	if err := ValidateNameReadProjection(rt.runtime.Names, &rt.build.Names); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateNotationReads(rt *schemaAudit) error {
	if err := ValidateNotationReadMap(rt.runtime.Notations, &rt.build.Names, rt.build.Notations); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateElementReads(rt *schemaAudit) error {
	if err := validateElementReadTableProjection(rt.runtime.Elements, rt.build.Elements, rt.build.ComplexTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateSimpleValueReads(rt *schemaAudit) error {
	if err := validateSimpleValueRouteReadProjectionForTypes(rt.runtime.SimpleValueRoutes, rt.build.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := validateSimpleTypeColdReadProjectionForTypes(rt.runtime.SimpleTypeCold, rt.build.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := validateSimpleValueQNameResolverNeedsForSimpleTypes(rt.runtime.SimpleValueQNameNeeds, rt.build.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateAttributeUseSetReads(rt *schemaAudit) error {
	if err := ValidateAttributeUseSetReadProjectionForSetsWithSimpleTypes(rt.runtime.AttributeUseSets, &rt.build.Names, rt.build.AttributeUseSets, rt.build.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateIdentityConstraintReads(rt *schemaAudit) error {
	if err := ValidateIdentityConstraintReadProjection(rt.runtime.Identities, rt.build.Identities); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateComplexTypeReads(rt *schemaAudit) error {
	if len(rt.runtime.ComplexTypes) != len(rt.build.ComplexTypes) {
		return xsderrors.InternalInvariant("complex type read projection count does not match types")
	}
	for i := range rt.build.ComplexTypes {
		ct := rt.build.ComplexTypes[i]
		if rt.runtime.ComplexTypes[i] != newComplexTypeRead(ct) {
			return xsderrors.InternalInvariant("complex type read projection does not match type")
		}
	}
	return nil
}

func validateWildcardReads(rt *schemaAudit) error {
	if err := ValidateWildcardViewProjectionTable(rt.runtime.Wildcards, &rt.build.Names, rt.build.Wildcards); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateCompiledModelReads(rt *schemaAudit) error {
	if err := validateCompiledModelReadProjectionTable(
		rt.runtime.CompiledModels,
		rt.build.CompiledModels,
		rt.contentModelWork,
	); err != nil {
		return contentModelAuditError(err)
	}
	return nil
}

func validateTypeDerivations(rt *schemaAudit) error {
	if rt.runtime.TypeDerivations.simpleTypeTable() != rt.runtime.SimpleTypeCold {
		return xsderrors.InternalInvariant("type derivation and simple value reads do not share the simple type table")
	}
	if err := ValidateTypeDerivationReadProjection(rt.runtime.TypeDerivations, rt.build.Builtin.AnyType, rt.build.SimpleTypes, rt.build.ComplexTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeCompiledModels(rt *schemaAudit) error {
	if err := validateCompiledModelsRuntime(
		&rt.build.Names,
		&rt.build,
		rt.build.Models,
		rt.build.CompiledModels,
		true,
		rt.contentModelWork,
		rt.contentAnalysis,
	); err != nil {
		return contentModelAuditError(err)
	}
	return nil
}

func validateRuntimeChoiceLimits(rt *schemaAudit) error {
	if err := ValidateChoiceLimitDerivations(
		&rt.build,
		rt.build.ComplexTypes,
		rt.build.Models,
		rt.build.Builtin.AnyType,
		rt.contentModelWork,
		rt.contentAnalysis,
	); err != nil {
		return contentModelAuditError(err)
	}
	return nil
}

func validateBuiltinIDs(ctx *schemaValidationContext) error {
	rt := ctx.rt
	if err := validateBuiltinDeclarations(rt); err != nil {
		return err
	}
	for _, base := range builtinSimpleExpectationTable {
		if err := validateBuiltinSimpleID(ctx, base); err != nil {
			return err
		}
	}
	return validateBuiltinAnyType(rt)
}

func validateBuiltinSimpleID(ctx *schemaValidationContext, base builtinSimpleExpectation) error {
	rt := ctx.rt
	expectation := builtinSimpleExpectationWithBuiltins(base, rt.build.Builtin)
	id, err := validateBuiltinSimpleTypeInSchema(rt, expectation)
	if err != nil {
		return err
	}
	typ := rt.build.SimpleTypes[id]
	if err := validateBuiltinSimpleFacets(typ, expectation.facetExpectation(typ.Base)); err != nil {
		return err
	}
	for _, literal := range typ.Facets.bounds {
		if literal == nil {
			continue
		}
		if ctx.validatedBoundLiterals == nil {
			ctx.validatedBoundLiterals = make(map[*CompiledLiteral]struct{})
		}
		ctx.validatedBoundLiterals[literal] = struct{}{}
	}
	return nil
}

func validateBuiltinDeclarations(rt *schemaAudit) error {
	if err := ValidateBuiltinDeclarationCounts(BuiltinDeclarationCounts{
		SimpleTypes:      len(rt.build.SimpleTypes),
		Attributes:       len(rt.build.Attributes),
		ComplexTypes:     len(rt.build.ComplexTypes),
		Wildcards:        len(rt.build.Wildcards),
		AttributeUseSets: len(rt.build.AttributeUseSets),
		Models:           len(rt.build.Models),
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return validateBuiltinAttributes(rt)
}

func validateBuiltinAttributes(rt *schemaAudit) error {
	for _, seed := range builtinAttributeSeedTable {
		if err := validateBuiltinAttribute(rt, seed); err != nil {
			return err
		}
	}
	return nil
}

func validateBuiltinAttribute(rt *schemaAudit, seed BuiltinAttributeSeed) error {
	expectation := builtinAttributeExpectationForSeed(seed, rt.build.Builtin)
	name, ok := builtinAttributeQName(&rt.build.Names, expectation)
	if !ok {
		return xsderrors.InternalInvariant("builtin attribute name is missing")
	}
	id, ok := rt.build.GlobalAttributes[name]
	if !ok || !ValidAttributeID(id, len(rt.build.Attributes)) || rt.build.Attributes[id].Name != name {
		return xsderrors.InternalInvariant("builtin attribute binding does not match declaration")
	}
	typ := rt.build.Attributes[id].Type
	if expectation.builtin == BuiltinValidationNone {
		if typ != expectation.typ {
			return xsderrors.InternalInvariant("builtin attribute type does not match handle")
		}
		return nil
	}
	if !ValidSimpleTypeID(typ, len(rt.build.SimpleTypes)) || rt.build.SimpleTypes[typ].Builtin != expectation.builtin {
		return xsderrors.InternalInvariant("builtin attribute type does not match lexical validator")
	}
	return nil
}

func validateBuiltinSimpleTypeInSchema(rt *schemaAudit, exp builtinSimpleExpectation) (SimpleTypeID, error) {
	if exp.checkID && !ValidSimpleTypeID(exp.id, len(rt.build.SimpleTypes)) {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type references invalid declaration")
	}
	id, name, err := builtinSimpleTypeBinding(rt, exp)
	if err != nil {
		return NoSimpleType, err
	}
	if err := validateBuiltinSimpleTypeRecord(rt, id, name, exp); err != nil {
		return NoSimpleType, err
	}
	return id, nil
}

func builtinSimpleTypeBinding(
	rt *schemaAudit,
	exp builtinSimpleExpectation,
) (SimpleTypeID, QName, error) {
	q, ok := builtinSimpleQName(&rt.build.Names, exp.local)
	if !ok {
		return NoSimpleType, QName{}, xsderrors.InternalInvariant("builtin simple type name is missing")
	}
	typ, ok := rt.build.GlobalTypes[q]
	id, simple := typ.Simple()
	if !ok || !simple {
		return NoSimpleType, QName{}, xsderrors.InternalInvariant("builtin simple type handle does not match global type")
	}
	if exp.checkID && id != exp.id {
		return NoSimpleType, QName{}, xsderrors.InternalInvariant("builtin simple type handle does not match global type")
	}
	if !ValidSimpleTypeID(id, len(rt.build.SimpleTypes)) {
		return NoSimpleType, QName{}, xsderrors.InternalInvariant("builtin simple type references invalid declaration")
	}
	return id, q, nil
}

func validateBuiltinSimpleTypeRecord(
	rt *schemaAudit,
	id SimpleTypeID,
	q QName,
	exp builtinSimpleExpectation,
) error {
	st := rt.build.SimpleTypes[id]
	if st.Name != q {
		return xsderrors.InternalInvariant("builtin simple type name does not match handle: " + exp.local)
	}
	if !builtinSimpleBaseMatchesSchema(rt, st.Base, exp.baseLocal) {
		return xsderrors.InternalInvariant("builtin simple type base does not match handle: " + exp.local)
	}
	if !builtinSimpleBaseMatchesSchema(rt, st.ListItem, exp.listItemLocal) {
		return xsderrors.InternalInvariant("builtin simple type list item does not match handle: " + exp.local)
	}
	if st.Variety != exp.variety {
		return xsderrors.InternalInvariant("builtin simple type variety does not match handle: " + exp.local)
	}
	if st.Primitive != exp.primitive {
		return xsderrors.InternalInvariant("builtin simple type primitive does not match handle: " + exp.local)
	}
	if st.Whitespace != exp.whitespace {
		return xsderrors.InternalInvariant("builtin simple type whitespace does not match handle: " + exp.local)
	}
	if st.Builtin != exp.builtin {
		return xsderrors.InternalInvariant("builtin simple type lexical validator does not match handle: " + exp.local)
	}
	if st.Identity != exp.identity {
		return xsderrors.InternalInvariant("builtin simple type identity does not match handle: " + exp.local)
	}
	return nil
}

func builtinSimpleBaseMatchesSchema(rt *schemaAudit, id SimpleTypeID, local string) bool {
	if local == "" {
		return id == NoSimpleType
	}
	q, ok := builtinSimpleQName(&rt.build.Names, local)
	if !ok {
		return false
	}
	typ, ok := rt.build.GlobalTypes[q]
	if !ok {
		return false
	}
	expected, simple := typ.Simple()
	return simple && id == expected
}

func validateBuiltinSimpleFacets(st SimpleType, exp BuiltinSimpleFacetExpectation) error {
	if err := ValidateBuiltinSimpleFacets(NewBuiltinSimpleFacetValidation(st.Facets, exp), exp); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateBuiltinAnyType(rt *schemaAudit) error {
	anyType := rt.build.Builtin.AnyType
	if !ValidComplexTypeID(anyType, len(rt.build.ComplexTypes)) {
		return xsderrors.InternalInvariant("builtin anyType references invalid declaration")
	}
	q, ok := builtinAnyTypeQName(&rt.build.Names)
	if !ok {
		return xsderrors.InternalInvariant("builtin anyType name is missing")
	}
	typ, ok := rt.build.GlobalTypes[q]
	id, isComplex := typ.Complex()
	if !ok || !isComplex || id != anyType {
		return xsderrors.InternalInvariant("builtin anyType handle does not match global type")
	}
	ct := rt.build.ComplexTypes[anyType]
	if ct.Name != q ||
		ct.Base != (TypeID{}) ||
		ct.ContentKind != ContentMixed ||
		ct.TextType != NoSimpleType ||
		!ValidContentModelID(ct.Content, len(rt.build.Models)) ||
		rt.build.Models[ct.Content].Kind != ModelAny ||
		!ValidAttributeUseSetID(ct.Attrs, len(rt.build.AttributeUseSets)) {
		return xsderrors.InternalInvariant("builtin anyType shape does not match handle")
	}
	set := rt.build.AttributeUseSets[ct.Attrs]
	if len(set.Uses) != 0 || len(set.Index) != 0 || set.Wildcard == NoWildcard ||
		!ValidWildcardID(set.Wildcard, len(rt.build.Wildcards)) {
		return xsderrors.InternalInvariant("builtin anyType attribute set does not match handle")
	}
	w := rt.build.Wildcards[set.Wildcard]
	if w.Mode != WildcardAny || w.Process != ProcessLax {
		return xsderrors.InternalInvariant("builtin anyType attribute wildcard does not match handle")
	}
	return nil
}

func validateElementDeclShape(rt *schemaAudit, decl ElementDecl) error {
	if err := ValidateElementDeclRuntime(&rt.build.Names, NewElementDeclValidationForDecl(decl), DeclRefLimits{
		SimpleTypeCount:  len(rt.build.SimpleTypes),
		ComplexTypeCount: len(rt.build.ComplexTypes),
		ElementCount:     len(rt.build.Elements),
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateElementDeclValueConstraints(ctx *schemaValidationContext, decl ElementDecl) error {
	rt := ctx.rt
	defaultType, err := elementValueConstraintType(rt, decl)
	if err != nil {
		return err
	}
	if err := validateValueConstraintRuntime(ctx, decl.Default, defaultType, "element declaration default"); err != nil {
		return err
	}
	if err := ValidateElementDeclValueConstraintRuntime(&rt.build, defaultType, decl.Default != nil, decl.Fixed != nil); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return validateValueConstraintRuntime(ctx, decl.Fixed, defaultType, "element declaration fixed")
}

func validateAttributeDecl(ctx *schemaValidationContext, decl AttributeDecl) error {
	rt := ctx.rt
	if err := ValidateAttributeDeclRuntime(&rt.build.Names, NewAttributeDeclValidationForDecl(decl), DeclRefLimits{
		SimpleTypeCount: len(rt.build.SimpleTypes),
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := ValidateAttributeDeclValueConstraintRuntime(&rt.build, decl.Type, decl.Default != nil, decl.Fixed != nil); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := validateValueConstraintRuntime(ctx, decl.Default, decl.Type, "attribute declaration default"); err != nil {
		return err
	}
	return validateValueConstraintRuntime(ctx, decl.Fixed, decl.Type, "attribute declaration fixed")
}

func validateSimpleType(ctx *schemaValidationContext, id SimpleTypeID, st SimpleType) error {
	if err := validateSimpleTypeStructure(ctx, id, st); err != nil {
		return err
	}
	return validateSimpleTypeFacetAncestry(ctx.rt, st)
}

func validateSimpleTypeStructure(ctx *schemaValidationContext, id SimpleTypeID, st SimpleType) error {
	rt := ctx.rt
	if err := ValidateSimpleTypeRuntime(&rt.build.Names, NewSimpleTypeValidationForSimpleType(st), SimpleTypeRefLimits{
		SimpleTypeCount: len(rt.build.SimpleTypes),
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := ValidateSimpleTypeIdentity(&rt.build, rt.build.Builtin, id, NewSimpleTypeIdentityNodeForSimpleType(st), st.Identity); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := ValidateSimpleFastPathForSimpleType(st); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := validateSimpleTypeDerivation(rt, id, st); err != nil {
		return err
	}
	if err := validateSimpleTypeDerivationSources(rt, id, st); err != nil {
		return err
	}
	if err := ValidateFacetLegalityAndConsistencyForSimpleType(st); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateSimpleTypeFacetAncestry(rt *schemaAudit, st SimpleType) error {
	shape := FacetCardinalityShapeForSimpleType(st)
	if shape.Length.Present && (shape.MinLength.Present || shape.MaxLength.Present) {
		if !ValidSimpleTypeID(st.Base, len(rt.build.SimpleTypes)) {
			return xsderrors.InternalInvariant("length facet bounds require a simple base type")
		}
		if err := ValidateFacetCardinalityAncestry(shape, FacetCardinalityShapeForSimpleType(rt.build.SimpleTypes[st.Base])); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
	}
	return nil
}

func validateSimpleTypeGraph(rt *schemaAudit) error {
	if err := ValidateSimpleTypeGraphForSimpleTypes(rt.build.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := validateUnionSourceGraph(rt.build.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateSimpleTypeDerivationSources(rt *schemaAudit, id SimpleTypeID, st SimpleType) error {
	if err := validateSimpleTypeDerivationFinalSources(rt, id, st); err != nil {
		return err
	}
	hasSources, err := validateSimpleUnionSourcePresence(rt, st)
	if err != nil || !hasSources {
		return err
	}
	audit := simpleUnionSourceAudit{
		rt:    rt,
		id:    id,
		union: st.Union,
		seen:  make(map[SimpleTypeID]struct{}, len(st.Union)),
	}
	for _, source := range st.UnionSources {
		if err := audit.validateSource(source); err != nil {
			return err
		}
	}
	if audit.next != len(st.Union) {
		return xsderrors.InternalInvariant("simple union members do not match direct member provenance")
	}
	return nil
}

func validateSimpleTypeDerivationFinalSources(rt *schemaAudit, id SimpleTypeID, st SimpleType) error {
	if SimpleTypeRestrictionRequired(id, st.Base, rt.build.Builtin) {
		if err := ValidateSimpleTypeFinalAllows(rt.build.SimpleTypes[st.Base].Final, DerivationRestriction); err != nil {
			return xsderrors.InternalInvariant("simple restriction base final is invalid")
		}
	}
	if st.Variety == SimpleVarietyList && st.Base == rt.build.Builtin.AnySimpleType {
		if err := ValidateSimpleTypeFinalAllows(rt.build.SimpleTypes[st.ListItem].Final, DerivationList); err != nil {
			return xsderrors.InternalInvariant("simple list item final is invalid")
		}
	}
	return nil
}

func validateSimpleUnionSourcePresence(rt *schemaAudit, st SimpleType) (bool, error) {
	if len(st.UnionSources) == 0 {
		if st.Variety == SimpleVarietyUnion && st.Base == rt.build.Builtin.AnySimpleType {
			return false, xsderrors.InternalInvariant("native simple union is missing direct member provenance")
		}
		return false, nil
	}
	if st.Variety != SimpleVarietyUnion || st.Base != rt.build.Builtin.AnySimpleType {
		return false, xsderrors.InternalInvariant("simple union member provenance is attached to invalid type")
	}
	return true, nil
}

type simpleUnionSourceAudit struct {
	rt    *schemaAudit
	seen  map[SimpleTypeID]struct{}
	union []SimpleTypeID
	id    SimpleTypeID
	next  int
}

func (a *simpleUnionSourceAudit) validateSource(source SimpleTypeID) error {
	if !ValidSimpleTypeID(source, len(a.rt.build.SimpleTypes)) || source == a.id {
		return xsderrors.InternalInvariant("simple union member provenance references invalid type")
	}
	direct := a.rt.build.SimpleTypes[source]
	if err := ValidateSimpleTypeFinalAllows(direct.Final, DerivationUnion); err != nil {
		return xsderrors.InternalInvariant("simple union member final is invalid")
	}
	members := []SimpleTypeID{source}
	if direct.Variety == SimpleVarietyUnion {
		members = direct.Union
	}
	for _, member := range members {
		if _, ok := a.seen[member]; ok {
			continue
		}
		a.seen[member] = struct{}{}
		if a.next >= len(a.union) || a.union[a.next] != member {
			return xsderrors.InternalInvariant("simple union members do not match direct member provenance")
		}
		a.next++
	}
	return nil
}

func validateUnionSourceGraph(types []SimpleType) error {
	audit := unionSourceGraphAudit{
		types: types,
		state: make([]simpleTypeGraphState, len(types)),
		stack: make([]unionSourceGraphFrame, 0, min(len(types), 1_024)),
	}
	for root := range types {
		if audit.state[root] != simpleTypeGraphUnchecked || len(types[root].UnionSources) == 0 {
			continue
		}
		audit.push(SimpleTypeID(root))
		for len(audit.stack) != 0 {
			if err := audit.advance(); err != nil {
				return err
			}
		}
	}
	return nil
}

type unionSourceGraphFrame struct {
	id   SimpleTypeID
	next int
}

type unionSourceGraphAudit struct {
	types []SimpleType
	state []simpleTypeGraphState
	stack []unionSourceGraphFrame
}

func (a *unionSourceGraphAudit) advance() error {
	last := len(a.stack) - 1
	current := &a.stack[last]
	dependency, ok := nextUnionSourceDependency(a.types[current.id], &current.next)
	if !ok {
		a.complete(last, current.id)
		return nil
	}
	if !ValidSimpleTypeID(dependency, len(a.types)) {
		return errors.New("simple union provenance graph references invalid type")
	}
	switch a.state[dependency] {
	case simpleTypeGraphChecking:
		return errors.New("simple union provenance graph contains cycle")
	case simpleTypeGraphUnchecked:
		a.push(dependency)
	case simpleTypeGraphChecked:
	}
	return nil
}

func nextUnionSourceDependency(typ SimpleType, next *int) (SimpleTypeID, bool) {
	dependencyCount := len(typ.UnionSources)
	if typ.Base != NoSimpleType {
		dependencyCount++
	}
	if *next == dependencyCount {
		return NoSimpleType, false
	}
	dependency := typ.Base
	if typ.Base == NoSimpleType || *next != 0 {
		sourceIndex := *next
		if typ.Base != NoSimpleType {
			sourceIndex--
		}
		dependency = typ.UnionSources[sourceIndex]
	}
	*next++
	return dependency, true
}

func (a *unionSourceGraphAudit) push(id SimpleTypeID) {
	a.state[id] = simpleTypeGraphChecking
	a.stack = appendDFSFrame(a.stack, unionSourceGraphFrame{id: id}, len(a.types))
}

func (a *unionSourceGraphAudit) complete(last int, id SimpleTypeID) {
	a.state[id] = simpleTypeGraphChecked
	a.stack = a.stack[:last]
}

func validateCompiledFacetLiterals(ctx *schemaValidationContext) error {
	types := ctx.rt.build.SimpleTypes
	audit := compiledFacetLiteralAudit{
		ctx:      ctx,
		ancestry: newSimpleTypeBaseAncestry(types),
	}
	for i := range types {
		if err := audit.validateType(SimpleTypeID(i), &types[i].Facets); err != nil {
			return err
		}
	}
	return nil
}

type compiledFacetLiteralAudit struct {
	ctx          *schemaValidationContext
	enumerations map[simpleValueEnumerationSource]SimpleTypeID
	ancestry     simpleTypeBaseAncestry
}

func (a *compiledFacetLiteralAudit) validateType(owner SimpleTypeID, facets *FacetSet) error {
	if err := a.validateBounds(owner, facets.bounds); err != nil {
		return err
	}
	return a.validateEnumeration(owner, facets.Enumeration)
}

func (a *compiledFacetLiteralAudit) validateBounds(owner SimpleTypeID, bounds facetBounds) error {
	for _, literal := range bounds {
		if literal == nil {
			continue
		}
		if !a.ancestry.strictAncestor(literal.Type, owner) {
			return xsderrors.InternalInvariant("facet literal compilation type is not an owner ancestor")
		}
		if err := a.ctx.validateCompiledBoundLiteralOnce(literal); err != nil {
			return err
		}
	}
	return nil
}

func (a *compiledFacetLiteralAudit) validateEnumeration(owner SimpleTypeID, literals []CompiledLiteral) error {
	if len(literals) == 0 {
		return nil
	}
	source, _ := simpleValueEnumerationSourceForLiterals(literals)
	compilationType, audited := a.enumerationCompilationType(source, literals)
	if !a.ancestry.strictAncestor(compilationType, owner) {
		return xsderrors.InternalInvariant("facet literal compilation type is not an owner ancestor")
	}
	if audited {
		return nil
	}
	if err := a.validateEnumerationLiterals(literals, compilationType); err != nil {
		return err
	}
	if a.enumerations == nil {
		a.enumerations = make(map[simpleValueEnumerationSource]SimpleTypeID)
	}
	a.enumerations[source] = compilationType
	return nil
}

func (a *compiledFacetLiteralAudit) enumerationCompilationType(source simpleValueEnumerationSource, literals []CompiledLiteral) (SimpleTypeID, bool) {
	if compilationType, audited := a.enumerations[source]; audited {
		return compilationType, true
	}
	return literals[0].Type, false
}

func (a *compiledFacetLiteralAudit) validateEnumerationLiterals(literals []CompiledLiteral, compilationType SimpleTypeID) error {
	for i := range literals {
		if literals[i].Type != compilationType {
			return xsderrors.InternalInvariant("facet enumeration has mixed compilation types")
		}
		if err := a.ctx.validateCompiledFacetLiteral(literals[i]); err != nil {
			return err
		}
	}
	return nil
}

type simpleTypeBaseAncestry struct {
	start []int
	end   []int
}

type simpleTypeAncestryFrame struct {
	id        SimpleTypeID
	nextChild SimpleTypeID
	entered   bool
}

func newSimpleTypeBaseAncestry(types []SimpleType) simpleTypeBaseAncestry {
	firstChild, nextSibling := simpleTypeAncestryChildren(types)
	audit := simpleTypeAncestryAudit{
		ancestry:    simpleTypeBaseAncestry{start: make([]int, len(types)), end: make([]int, len(types))},
		firstChild:  firstChild,
		nextSibling: nextSibling,
		stack:       make([]simpleTypeAncestryFrame, 0, min(len(types), 1_024)),
	}
	for i := range types {
		if types[i].Base == NoSimpleType {
			audit.visit(SimpleTypeID(i), len(types))
		}
	}
	return audit.ancestry
}

func simpleTypeAncestryChildren(types []SimpleType) ([]SimpleTypeID, []SimpleTypeID) {
	firstChild := make([]SimpleTypeID, len(types))
	nextSibling := make([]SimpleTypeID, len(types))
	for i := range types {
		firstChild[i] = NoSimpleType
		nextSibling[i] = NoSimpleType
	}
	for i := range slices.Backward(types) {
		base := types[i].Base
		if base == NoSimpleType {
			continue
		}
		id := SimpleTypeID(i) //nolint:gosec // schema simple-type tables are bounded to uint32 IDs.
		nextSibling[id] = firstChild[base]
		firstChild[base] = id
	}
	return firstChild, nextSibling
}

type simpleTypeAncestryAudit struct {
	ancestry    simpleTypeBaseAncestry
	firstChild  []SimpleTypeID
	nextSibling []SimpleTypeID
	stack       []simpleTypeAncestryFrame
	clock       int
}

func (a *simpleTypeAncestryAudit) visit(root SimpleTypeID, typeCount int) {
	a.stack = appendDFSFrame(a.stack, simpleTypeAncestryFrame{id: root}, typeCount)
	for len(a.stack) != 0 {
		last := len(a.stack) - 1
		frame := &a.stack[last]
		if !frame.entered {
			a.enter(frame)
		}
		if frame.nextChild == NoSimpleType {
			a.complete(last, frame.id)
			continue
		}
		a.visitNextChild(frame, typeCount)
	}
}

func (a *simpleTypeAncestryAudit) enter(frame *simpleTypeAncestryFrame) {
	a.ancestry.start[frame.id] = a.clock
	a.clock++
	frame.nextChild = a.firstChild[frame.id]
	frame.entered = true
}

func (a *simpleTypeAncestryAudit) complete(last int, id SimpleTypeID) {
	a.ancestry.end[id] = a.clock
	a.stack = a.stack[:last]
}

func (a *simpleTypeAncestryAudit) visitNextChild(frame *simpleTypeAncestryFrame, typeCount int) {
	child := frame.nextChild
	frame.nextChild = a.nextSibling[child]
	a.stack = appendDFSFrame(a.stack, simpleTypeAncestryFrame{id: child}, typeCount)
}

func (a simpleTypeBaseAncestry) strictAncestor(ancestor, owner SimpleTypeID) bool {
	if ancestor == owner || !ValidSimpleTypeID(ancestor, len(a.start)) || !ValidSimpleTypeID(owner, len(a.start)) {
		return false
	}
	return a.start[ancestor] <= a.start[owner] && a.end[owner] <= a.end[ancestor]
}

func (ctx *schemaValidationContext) validateCompiledBoundLiteralOnce(literal *CompiledLiteral) error {
	if _, ok := ctx.validatedBoundLiterals[literal]; ok {
		return nil
	}
	if err := ctx.validateCompiledFacetLiteral(*literal); err != nil {
		return err
	}
	if ctx.validatedBoundLiterals == nil {
		ctx.validatedBoundLiterals = make(map[*CompiledLiteral]struct{})
	}
	ctx.validatedBoundLiterals[literal] = struct{}{}
	return nil
}

func (ctx *schemaValidationContext) validateCompiledFacetLiteral(literal CompiledLiteral) error {
	rt := ctx.rt
	if !ValidSimpleTypeID(literal.Type, len(rt.build.SimpleTypes)) {
		return xsderrors.InternalInvariant("facet literal references invalid compilation type")
	}
	replay, err := NewValueConstraintNameReplay(literal.ResolvedNames)
	if err != nil {
		return xsderrors.InternalInvariant("facet literal resolved-name proof is invalid")
	}
	value, err := rt.ValidateSimpleValue(literal.Type, literal.Lexical, replay.ResolveQName, SimpleNeedCanonical)
	if err != nil {
		return xsderrors.InternalInvariant("facet literal lexical replay failed")
	}
	if err := replay.ValidateConsumed(); err != nil {
		return xsderrors.InternalInvariant("facet literal resolved-name proof was not fully consumed")
	}
	expected := NewCompiledLiteralForSimpleType(rt.build.SimpleTypes[literal.Type], literal.Type, literal.Lexical, value.Canonical, nil)
	if !equalCompiledLiteralCache(literal, expected) {
		return xsderrors.InternalInvariant("facet literal cache does not match lexical value")
	}
	return nil
}

func equalCompiledLiteralCache(got, want CompiledLiteral) bool {
	if got.Type != want.Type || got.Canonical != want.Canonical || got.Actual.Valid != want.Actual.Valid {
		return false
	}
	if !got.Actual.Valid {
		return true
	}
	return got.Actual.Kind == want.Actual.Kind &&
		EqualPrimitiveActualValues(got.Actual, got.Canonical, want.Actual, want.Canonical)
}

func validateSimpleTypeDerivation(rt *schemaAudit, id SimpleTypeID, st SimpleType) error {
	if !SimpleTypeRestrictionRequired(id, st.Base, rt.build.Builtin) {
		return nil
	}
	base := rt.build.SimpleTypes[st.Base]
	if err := ValidateSimpleTypeRestrictionRuntime(
		NewSimpleTypeRestrictionValidationForSimpleType(st),
		NewSimpleTypeRestrictionValidationForSimpleType(base),
	); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := ValidateFixedFacetPreservation(FixedFacetPreservationForSimpleTypes(st, base)); err != nil {
		return xsderrors.InternalInvariant("simple type fixed facet restriction is invalid")
	}
	if err := ValidatePrimitiveFacetRestrictions(st, base.Facets, OrderedFacetStep{}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := ValidateFacetRestrictionForSimpleTypes(st, base); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateComplexTypeRecord(build *SchemaBuild, id ComplexTypeID, ct ComplexType) error {
	if err := ValidateComplexTypeRuntime(&build.Names, id, ct, build.Models, ComplexTypeRefLimits{
		SimpleTypeCount:      len(build.SimpleTypes),
		ComplexTypeCount:     len(build.ComplexTypes),
		AttributeUseSetCount: len(build.AttributeUseSets),
		AnyType:              build.Builtin.AnyType,
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateComplexTypeDerivation(rt *schemaAudit, id ComplexTypeID, ct ComplexType) error {
	if id == rt.build.Builtin.AnyType {
		return nil
	}
	if baseID, ok := ct.Base.Simple(); ok {
		return validateSimpleBaseComplexDerivation(rt, baseID, ct)
	}
	if err := ValidateComplexTypeDerivationRuntime(rt.build.Builtin.AnyType, id, ct); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	switch ct.Derivation {
	case DerivationKindExtension:
		return validateComplexExtensionRuntime(rt, ct)
	case DerivationKindRestriction:
		return validateComplexRestrictionRuntime(rt, ct)
	case DerivationKindNone:
		return xsderrors.InternalInvariant("complex derivation mode was not handled")
	}
	return nil
}

func validateSimpleBaseComplexDerivation(rt *schemaAudit, baseID SimpleTypeID, ct ComplexType) error {
	if err := ValidateComplexTypeSimpleBaseExtensionRuntime(&rt.build, baseID, ct); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return checkDerivedAttributeWildcard(rt, NoAttributeWildcardState(), NewAttributeWildcardStateForUseSet(rt.build.AttributeUseSets[ct.Attrs]), AttributeWildcardExtension)
}

func validateComplexExtensionRuntime(rt *schemaAudit, ct ComplexType) error {
	base, err := complexBaseRuntime(rt, ct)
	if err != nil {
		return err
	}
	if err := ValidateComplexTypeExtensionRuntime(&rt.build, base, ct, rt.build.Builtin.AnyType); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return validateAttributeUsesExtend(rt, base.Attrs, ct.Attrs)
}

func validateComplexRestrictionRuntime(rt *schemaAudit, ct ComplexType) error {
	base, err := complexBaseRuntime(rt, ct)
	if err != nil {
		return err
	}
	if err := ValidateComplexTypeRestrictionRuntime(&rt.build, rt.contentAnalysis, base, ct); err != nil {
		return contentModelAuditError(err)
	}
	if err := ValidateContentRestriction(
		&rt.build,
		base.Content,
		ct.Content,
		rt.contentModelWork,
		rt.contentAnalysis,
	); err != nil {
		return contentModelAuditError(err)
	}
	return validateAttributeUsesRestrict(rt, base.Attrs, ct.Attrs, ct.ExplicitDerivation)
}

func contentModelAuditError(err error) error {
	var diagnostic *xsderrors.Error
	if errors.As(err, &diagnostic) {
		return err
	}
	return xsderrors.InternalInvariant(err.Error())
}

func complexBaseRuntime(rt *schemaAudit, ct ComplexType) (ComplexType, error) {
	baseID, err := ComplexTypeDerivationBaseID(ct.Base, len(rt.build.ComplexTypes))
	if err != nil {
		return ComplexType{}, xsderrors.InternalInvariant(err.Error())
	}
	return rt.build.ComplexTypes[baseID], nil
}

func validateAttributeUsesExtend(rt *schemaAudit, baseID, derivedID AttributeUseSetID) error {
	base := rt.build.AttributeUseSets[baseID]
	derived := rt.build.AttributeUseSets[derivedID]
	if err := ValidateAttributeUseSetExtension(NewAttributeUseExtensionValidationsForUses(base.Uses), NewAttributeUseExtensionValidationsForUses(derived.Uses)); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return checkDerivedAttributeWildcard(rt, NewAttributeWildcardStateForUseSet(base), NewAttributeWildcardStateForUseSet(derived), AttributeWildcardExtension)
}

func validateAttributeUsesRestrict(rt *schemaAudit, baseID, derivedID AttributeUseSetID, bindWildcard bool) error {
	base := rt.build.AttributeUseSets[baseID]
	derived := rt.build.AttributeUseSets[derivedID]
	if err := ValidateAttributeUseSetRestriction(
		&rt.build,
		NewAttributeUseRestrictionValidationsForUses(base.Uses),
		NewAttributeUseRestrictionValidationsForUses(derived.Uses),
		NewAttributeWildcardStateForUseSet(base),
		NewAttributeWildcardStateForUseSet(derived),
		bindWildcard,
	); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateAttributeUseSetRuntime(ctx *schemaValidationContext, _ AttributeUseSetID, set AttributeUseSet) error {
	rt := ctx.rt
	if err := ValidateAttributeUseSetRecord(&rt.build.Names, &rt.build, set); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	for _, use := range set.Uses {
		if err := validateValueConstraintRuntime(ctx, use.Default, use.Type, "attribute use default"); err != nil {
			return err
		}
		if err := validateValueConstraintRuntime(ctx, use.Fixed, use.Type, "attribute use fixed"); err != nil {
			return err
		}
	}
	return nil
}

func checkDerivedAttributeWildcard(rt *schemaAudit, base, derived AttributeWildcardState, expected AttributeWildcardDerivation) error {
	if err := ValidateAttributeWildcardDerivation(&rt.build, base, derived, expected); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func elementValueConstraintType(rt *schemaAudit, decl ElementDecl) (SimpleTypeID, error) {
	if decl.Default == nil && decl.Fixed == nil {
		return NoSimpleType, nil
	}
	id, err := ElementValueConstraintType(&rt.build, rt.contentAnalysis, decl.Type)
	if err != nil {
		return NoSimpleType, contentModelAuditError(err)
	}
	return id, nil
}

func validateValueConstraintRuntime(ctx *schemaValidationContext, vc *ValueConstraint, expected SimpleTypeID, label string) error {
	if vc == nil {
		return nil
	}
	rt := ctx.rt
	cached := NewValueConstraintValidation(vc)
	if err := ValidateValueConstraintShape(&rt.build, cached, expected); err != nil {
		return xsderrors.InternalInvariant(label + " " + err.Error())
	}
	if expected == NoSimpleType {
		return nil
	}
	if err := validateSchemaBuildSimpleValuePayload(&rt.build, vc.Value, label); err != nil {
		return err
	}
	return ctx.validateValueConstraintReplay(vc, expected, label, cached)
}

// ValueConstraintSimpleType returns compiler-owned value-constraint metadata.
func (rt *SchemaBuild) ValueConstraintSimpleType(id SimpleTypeID) (ValueConstraintSimpleType, bool) {
	st, ok := SimpleTypeByID(rt.SimpleTypes, id)
	if !ok {
		return ValueConstraintSimpleType{}, false
	}
	return NewValueConstraintSimpleTypeForSimpleType(*st), true
}

// ValueConstraintComplexType returns compiler-owned value-constraint metadata.
func (rt *SchemaBuild) ValueConstraintComplexType(id ComplexTypeID) (ValueConstraintComplexType, bool) {
	ct, ok := ComplexTypeByID(rt.ComplexTypes, id)
	if !ok {
		return ValueConstraintComplexType{}, false
	}
	return NewValueConstraintComplexTypeForComplexType(*ct), true
}

func validateSchemaBuildSimpleValuePayload(build *SchemaBuild, value SimpleValue, label string) error {
	typ, ok := simpleValuePayloadTypeForBuild(build, value.Type)
	if !ok {
		return xsderrors.InternalInvariant(label + " references invalid simple value type")
	}
	err := ValidateSimpleValuePayload(value, typ)
	if err != nil {
		return xsderrors.InternalInvariant(label + " " + err.Error())
	}
	return nil
}

func (ctx *schemaValidationContext) validateValueConstraintReplay(vc *ValueConstraint, expected SimpleTypeID, label string, cached ValueConstraintValidation) error {
	err := ValidateValueConstraintReplay(cached, expected, vc.ResolvedNames, ctx.replayValueConstraintSimpleValue)
	if err != nil {
		return xsderrors.InternalInvariant(label + " " + err.Error())
	}
	return nil
}

func (ctx *schemaValidationContext) replayValueConstraintSimpleValue(id SimpleTypeID, lexical string, resolve ValueConstraintQNameResolver, needs SimpleValueNeed) (SimpleValue, error) {
	rt := ctx.rt
	return rt.ValidateSimpleValue(id, lexical, ResolveQNameParts(resolve), needs)
}

func simpleValuePayloadTypeForBuild(build *SchemaBuild, id SimpleTypeID) (SimpleValuePayloadType, bool) {
	st, ok := UsableSimpleType(build.SimpleTypes, id)
	if !ok {
		return SimpleValuePayloadType{}, false
	}
	return SimpleValuePayloadType{
		Primitive: st.Primitive,
		Variety:   st.Variety,
		Identity:  st.Identity,
	}, true
}
