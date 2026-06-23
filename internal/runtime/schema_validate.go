package runtime

import "github.com/jacoelho/xsd/xsderrors"

type schemaValidationMode uint8

const (
	schemaValidationFull schemaValidationMode = iota
	schemaValidationPublication
)

// ValidateSchema validates runtime schema invariants.
func ValidateSchema(rt *Schema) error {
	return validateSchema(rt, schemaValidationFull)
}

// ValidateSchemaPublication validates invariants needed to publish a
// compiler-owned runtime. It assumes construction already enforced derivation,
// facet, and value-replay rules.
func ValidateSchemaPublication(rt *Schema) error {
	return validateSchema(rt, schemaValidationPublication)
}

// ValidateCompilerPublication validates the direct-reference invariants needed
// when the compiler hands off an unpublished runtime schema. Full schema audits
// remain in ValidateSchema and ValidateSchemaPublication.
func ValidateCompilerPublication(rt *Schema) error {
	ctx := schemaValidationContext{rt: rt, mode: schemaValidationPublication}
	if err := validateRuntimeGlobals(rt); err != nil {
		return err
	}
	if err := validateRuntimeSubstitutions(rt); err != nil {
		return err
	}
	if err := validateRuntimeComponents(&ctx); err != nil {
		return err
	}
	return validateRuntimeCompiledModels(rt, schemaValidationPublication)
}

func validateSchema(rt *Schema, mode schemaValidationMode) error {
	ctx := schemaValidationContext{rt: rt, mode: mode}
	if err := validateNameTable(rt); err != nil {
		return err
	}
	if err := validateRuntimeGlobals(rt); err != nil {
		return err
	}
	if err := validateRuntimeSubstitutions(rt); err != nil {
		return err
	}
	if err := validateBuiltins(rt, mode); err != nil {
		return err
	}
	if err := validateRuntimeComponents(&ctx); err != nil {
		return err
	}
	if err := validateRuntimeChoiceLimits(rt, mode); err != nil {
		return err
	}
	return validateRuntimeCompiledModels(rt, mode)
}

type schemaValidationContext struct {
	rt                   *Schema
	simpleValueCallbacks SimpleValueCallbacks
	mode                 schemaValidationMode
}

func validateNameTable(rt *Schema) error {
	if err := ValidateRuntimeNameTable(&rt.Names); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeGlobals(rt *Schema) error {
	if rt == nil {
		return xsderrors.InternalInvariant("runtime globals require schema")
	}
	for q, id := range rt.GlobalAttributes {
		if !rt.Names.ValidQName(q) || !ValidAttributeID(id, len(rt.Attributes)) {
			return xsderrors.InternalInvariant("global attribute references invalid declaration")
		}
		if rt.Attributes[id].Name != q {
			return xsderrors.InternalInvariant("global attribute name does not match declaration")
		}
	}
	for q, id := range rt.GlobalElements {
		if !rt.Names.ValidQName(q) || !ValidElementID(id, len(rt.Elements)) {
			return xsderrors.InternalInvariant("global element references invalid declaration")
		}
		if rt.Elements[id].Name != q {
			return xsderrors.InternalInvariant("global element name does not match declaration")
		}
	}
	for q, typ := range rt.GlobalTypes {
		name, ok := TypeNameByID(rt.SimpleTypes, rt.ComplexTypes, typ)
		if !rt.Names.ValidQName(q) || !ok {
			return xsderrors.InternalInvariant("global type references invalid declaration")
		}
		if name != q {
			return xsderrors.InternalInvariant("global type name does not match declaration")
		}
	}
	for q, id := range rt.GlobalIdentities {
		if !rt.Names.ValidQName(q) || !ValidIdentityConstraintID(id, len(rt.Identities)) {
			return xsderrors.InternalInvariant("global identity references invalid declaration")
		}
		if rt.Identities[id].Name != q {
			return xsderrors.InternalInvariant("global identity name does not match declaration")
		}
	}
	for q := range rt.Notations {
		if !rt.Names.ValidQName(q) {
			return xsderrors.InternalInvariant("notation references invalid name")
		}
	}
	return nil
}

func validateRuntimeSubstitutions(rt *Schema) error {
	if err := ValidateSubstitutionMaps(
		rt,
		&rt.Names,
		rt.Elements,
		rt.GlobalElements,
		rt.Substitutions,
		rt.SubstitutionLookup,
	); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeComponents(ctx *schemaValidationContext) error {
	rt := ctx.rt
	for i := range rt.Elements {
		if err := validateElementDecl(ctx, rt.Elements[i]); err != nil {
			return err
		}
	}
	for i := range rt.Attributes {
		if err := validateAttributeDecl(ctx, rt.Attributes[i]); err != nil {
			return err
		}
	}
	if rt.ReadProjectionsPublished() {
		if err := validateRuntimeReadProjections(rt); err != nil {
			return err
		}
	}
	for i := range rt.SimpleTypes {
		if err := validateSimpleType(ctx, SimpleTypeID(i), rt.SimpleTypes[i]); err != nil {
			return err
		}
	}
	if err := validateSimpleTypeGraph(rt); err != nil {
		return err
	}
	for i := range rt.Wildcards {
		if err := ValidateWildcard(&rt.Names, rt.Wildcards[i]); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
	}
	for i := range rt.AttributeUseSets {
		if err := validateAttributeUseSetRuntime(ctx, AttributeUseSetID(i), rt.AttributeUseSets[i]); err != nil {
			return err
		}
	}
	contentModelLimits := ContentModelRefLimits{
		ElementCount:      len(rt.Elements),
		ContentModelCount: len(rt.Models),
		WildcardCount:     len(rt.Wildcards),
	}
	for i := range rt.Models {
		if err := ValidateContentModelRuntime(rt.Models[i], contentModelLimits); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
	}
	for i := range rt.ComplexTypes {
		if err := validateComplexType(ctx, ComplexTypeID(i), rt.ComplexTypes[i], rt.Models); err != nil {
			return err
		}
	}
	if err := ValidateIdentityConstraints(&rt.Names, rt.Identities); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeReadProjections(rt *Schema) error {
	if err := validateAttributeDeclReads(rt); err != nil {
		return err
	}
	if err := validateGlobalReadMaps(rt); err != nil {
		return err
	}
	if err := validateSubstitutionReadMaps(rt); err != nil {
		return err
	}
	if err := validateNameReads(rt); err != nil {
		return err
	}
	if err := validateNotationReads(rt); err != nil {
		return err
	}
	if err := validateElementValueConstraintReads(rt); err != nil {
		return err
	}
	if err := validateElementNames(rt); err != nil {
		return err
	}
	if err := validateTypeDerivations(rt); err != nil {
		return err
	}
	if err := validateSimpleTypePrimitives(rt); err != nil {
		return err
	}
	if err := validateSimpleTypeIdentities(rt); err != nil {
		return err
	}
	if err := validateSimpleTypeFinals(rt); err != nil {
		return err
	}
	if err := validateSimpleValueReads(rt); err != nil {
		return err
	}
	if err := validateAttributeUseSetReads(rt); err != nil {
		return err
	}
	if err := validateElementStartInfos(rt); err != nil {
		return err
	}
	if err := validateElementIdentityConstraintReads(rt); err != nil {
		return err
	}
	if err := validateIdentityConstraintReads(rt); err != nil {
		return err
	}
	if err := validateComplexTypeInfos(rt); err != nil {
		return err
	}
	if err := validateComplexAttributeUseSetIDs(rt); err != nil {
		return err
	}
	if err := validateComplexContentModelIDs(rt); err != nil {
		return err
	}
	if err := validateComplexSimpleContentReads(rt); err != nil {
		return err
	}
	if err := validateComplexChildContentReads(rt); err != nil {
		return err
	}
	if err := validateComplexTextContentReads(rt); err != nil {
		return err
	}
	if err := validateWildcardReads(rt); err != nil {
		return err
	}
	if err := validateCompiledModelViews(rt); err != nil {
		return err
	}
	if err := ValidateElementTextContentForSimpleType(rt.SimpleTextContentRead); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateAttributeDeclReads(rt *Schema) error {
	if err := ValidateAttributeDeclReadProjectionForDecls(rt.AttributeDeclReads, rt.Attributes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateGlobalReadMaps(rt *Schema) error {
	reads := NewGlobalReadMapProjection(rt.GlobalAttributeReads, rt.GlobalElementReads, rt.GlobalTypeReads)
	globals := NewGlobalDeclarationMaps(rt.GlobalAttributes, rt.GlobalElements, rt.GlobalTypes)
	if err := ValidateGlobalReadMaps(reads, globals); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateNotationReads(rt *Schema) error {
	if err := ValidateNotationReadMap(rt.NotationReads, &rt.Names, rt.Notations); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateElementValueConstraintReads(rt *Schema) error {
	if err := ValidateElementValueConstraintReadProjectionForDecls(rt.ElementValueConstraintReads, rt.Elements); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateSubstitutionReadMaps(rt *Schema) error {
	if err := ValidateSubstitutionReadMaps(rt.SubstitutionReads, rt.SubstitutionLookupReads, rt.Substitutions, rt.SubstitutionLookup); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateNameReads(rt *Schema) error {
	if err := ValidateNameReadProjection(rt.NameReads, &rt.Names); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateSimpleTypePrimitives(rt *Schema) error {
	if err := ValidateSimpleTypePrimitiveReadProjectionForTypes(rt.SimpleTypePrimitives, rt.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateSimpleTypeIdentities(rt *Schema) error {
	if err := ValidateSimpleTypeIdentityReadProjectionForTypes(rt.SimpleTypeIdentities, rt.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateSimpleTypeFinals(rt *Schema) error {
	if err := ValidateSimpleTypeFinalReadProjectionForTypes(rt.SimpleTypeFinals, rt.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateSimpleValueReads(rt *Schema) error {
	if len(rt.SimpleValueTypeReads) == 0 {
		if err := ValidateSimpleValueQNameResolverNeedsForSimpleTypes(rt.SimpleValueQNameResolverNeeds, rt.SimpleTypes); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
		return nil
	}
	if err := ValidateSimpleValueTypeReadProjectionForTypes(rt.SimpleValueTypeReads, rt.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if len(rt.SimpleValueFacetReads.Index) != 0 {
		if err := ValidateSimpleValueFacetReadProjectionForTypes(rt.SimpleValueFacetReads, rt.SimpleTypes); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
	}
	if err := ValidateSimpleValueQNameResolverNeedsForSimpleTypes(rt.SimpleValueQNameResolverNeeds, rt.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateAttributeUseSetReads(rt *Schema) error {
	if len(rt.SimpleValueTypeReads) == 0 {
		if err := ValidateAttributeUseSetReadProjectionForSetsWithSimpleTypes(rt.AttributeUseSetReads, &rt.Names, rt.AttributeUseSets, rt.SimpleTypes); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
		return nil
	}
	if err := ValidateAttributeUseSetReadProjectionForSetsWithTypeReads(rt.AttributeUseSetReads, &rt.Names, rt.AttributeUseSets, rt.SimpleValueTypeReads); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateElementNames(rt *Schema) error {
	if err := ValidateElementNameReadProjectionForDecls(rt.ElementNames, rt.Elements); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateTypeDerivations(rt *Schema) error {
	if err := ValidateTypeDerivationReadProjection(rt.TypeDerivations, rt.Builtin.AnyType, rt.SimpleTypes, rt.ComplexTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateElementStartInfos(rt *Schema) error {
	if err := ValidateElementStartInfosForElementDecls(rt.ElementStartInfos, rt.Elements); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateElementIdentityConstraintReads(rt *Schema) error {
	if err := ValidateElementIdentityConstraintReadProjectionForDecls(rt.ElementIdentityConstraintReads, rt.Elements); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateIdentityConstraintReads(rt *Schema) error {
	if err := ValidateIdentityConstraintReadProjection(rt.IdentityConstraintReads, rt.Identities); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateComplexTypeInfos(rt *Schema) error {
	if err := ValidateTypeInfosForComplexTypes(rt.ComplexTypeInfos, rt.ComplexTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateComplexAttributeUseSetIDs(rt *Schema) error {
	if err := ValidateComplexAttributeUseSetIDProjection(rt.ComplexAttributeUseSetIDs, rt.ComplexTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateComplexContentModelIDs(rt *Schema) error {
	if err := ValidateComplexContentModelIDProjection(rt.ComplexContentModelIDs, rt.ComplexTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateComplexSimpleContentReads(rt *Schema) error {
	if err := ValidateSimpleContentTypeReadProjectionTable(rt.ComplexSimpleContentReads, rt.ComplexTypes, len(rt.SimpleTypes)); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateComplexChildContentReads(rt *Schema) error {
	if err := ValidateElementChildContentProjection(rt.ComplexChildContentReads, rt.ComplexTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateComplexTextContentReads(rt *Schema) error {
	if err := ValidateElementTextContentProjection(rt.ComplexTextContentReads, rt.ComplexTypes, false); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := ValidateElementTextContentProjection(rt.FixedComplexTextContentReads, rt.ComplexTypes, true); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateWildcardReads(rt *Schema) error {
	if err := ValidateWildcardViewProjectionTable(rt.WildcardReads, &rt.Names, rt.Wildcards); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateCompiledModelViews(rt *Schema) error {
	if err := ValidateCompiledModelViewProjectionTable(rt.CompiledModelViews, rt.CompiledModels); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeCompiledModels(rt *Schema, mode schemaValidationMode) error {
	validateUPA := mode == schemaValidationFull
	if err := validateCompiledModelsRuntime(&rt.Names, rt, rt.Models, rt.CompiledModels, validateUPA); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeChoiceLimits(rt *Schema, mode schemaValidationMode) error {
	if mode == schemaValidationPublication {
		return nil
	}
	if err := ValidateChoiceLimitDerivations(
		rt,
		rt.ComplexTypes,
		rt.Models,
		rt.Builtin.AnyType,
	); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateBuiltins(rt *Schema, mode schemaValidationMode) error {
	if mode == schemaValidationPublication {
		if err := validateBuiltinDeclarations(rt); err != nil {
			return err
		}
		return validateBuiltinAnyType(rt)
	}
	return validateBuiltinIDs(rt)
}

func validateBuiltinIDs(rt *Schema) error {
	if err := validateBuiltinDeclarations(rt); err != nil {
		return err
	}
	for _, base := range builtinSimpleExpectationTable {
		exp := builtinSimpleExpectationWithBuiltins(base, rt.Builtin)
		id, err := validateBuiltinSimpleTypeInSchema(rt, exp)
		if err != nil {
			return err
		}
		if err := validateBuiltinSimpleFacets(rt.SimpleTypes[id], exp.facetExpectation(id)); err != nil {
			return err
		}
	}
	return validateBuiltinAnyType(rt)
}

func validateBuiltinDeclarations(rt *Schema) error {
	if err := ValidateBuiltinDeclarationCounts(BuiltinDeclarationCounts{
		SimpleTypes:      len(rt.SimpleTypes),
		Attributes:       len(rt.Attributes),
		ComplexTypes:     len(rt.ComplexTypes),
		Wildcards:        len(rt.Wildcards),
		AttributeUseSets: len(rt.AttributeUseSets),
		Models:           len(rt.Models),
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return validateBuiltinAttributes(rt)
}

func validateBuiltinAttributes(rt *Schema) error {
	for _, seed := range builtinAttributeSeedTable {
		exp := builtinAttributeExpectationForSeed(seed, rt.Builtin)
		q, ok := builtinAttributeQName(&rt.Names, exp)
		if !ok {
			return xsderrors.InternalInvariant("builtin attribute name is missing")
		}
		id, ok := rt.GlobalAttributes[q]
		if !ok || !ValidAttributeID(id, len(rt.Attributes)) || rt.Attributes[id].Name != q {
			return xsderrors.InternalInvariant("builtin attribute binding does not match declaration")
		}
		typ := rt.Attributes[id].Type
		if exp.builtin == BuiltinValidationNone {
			if typ != exp.typ {
				return xsderrors.InternalInvariant("builtin attribute type does not match handle")
			}
			continue
		}
		if !ValidSimpleTypeID(typ, len(rt.SimpleTypes)) || rt.SimpleTypes[typ].Builtin != exp.builtin {
			return xsderrors.InternalInvariant("builtin attribute type does not match lexical validator")
		}
	}
	return nil
}

func validateBuiltinSimpleTypeInSchema(rt *Schema, exp builtinSimpleExpectation) (SimpleTypeID, error) {
	if exp.checkID && !ValidUint32Index(uint32(exp.id), len(rt.SimpleTypes)) {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type references invalid declaration")
	}
	q, ok := builtinSimpleQName(&rt.Names, exp.local)
	if !ok {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type name is missing")
	}
	typ, ok := rt.GlobalTypes[q]
	id, simple := typ.Simple()
	if !ok || !simple {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type handle does not match global type")
	}
	if exp.checkID && id != exp.id {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type handle does not match global type")
	}
	if !ValidSimpleTypeID(id, len(rt.SimpleTypes)) {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type references invalid declaration")
	}
	st := rt.SimpleTypes[id]
	if st.Name != q {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type name does not match handle: " + exp.local)
	}
	if !builtinSimpleBaseMatchesSchema(rt, st.Base, exp.baseLocal) {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type base does not match handle: " + exp.local)
	}
	if !builtinSimpleBaseMatchesSchema(rt, st.ListItem, exp.listItemLocal) {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type list item does not match handle: " + exp.local)
	}
	if st.Variety != exp.variety {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type variety does not match handle: " + exp.local)
	}
	if st.Primitive != exp.primitive {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type primitive does not match handle: " + exp.local)
	}
	if st.Whitespace != exp.whitespace {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type whitespace does not match handle: " + exp.local)
	}
	if st.Builtin != exp.builtin {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type lexical validator does not match handle: " + exp.local)
	}
	if st.Identity != exp.identity {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type identity does not match handle: " + exp.local)
	}
	return id, nil
}

func builtinSimpleBaseMatchesSchema(rt *Schema, id SimpleTypeID, local string) bool {
	if local == "" {
		return id == NoSimpleType
	}
	q, ok := builtinSimpleQName(&rt.Names, local)
	if !ok {
		return false
	}
	typ, ok := rt.GlobalTypes[q]
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

func validateBuiltinAnyType(rt *Schema) error {
	anyType := rt.Builtin.AnyType
	if !ValidComplexTypeID(anyType, len(rt.ComplexTypes)) {
		return xsderrors.InternalInvariant("builtin anyType references invalid declaration")
	}
	q, ok := builtinAnyTypeQName(&rt.Names)
	if !ok {
		return xsderrors.InternalInvariant("builtin anyType name is missing")
	}
	typ, ok := rt.GlobalTypes[q]
	id, isComplex := typ.Complex()
	if !ok || !isComplex || id != anyType {
		return xsderrors.InternalInvariant("builtin anyType handle does not match global type")
	}
	ct := rt.ComplexTypes[anyType]
	if ct.Name != q ||
		ct.Base != (TypeID{}) ||
		ct.ContentKind != ContentMixed ||
		ct.TextType != NoSimpleType ||
		!ValidContentModelID(ct.Content, len(rt.Models)) ||
		rt.Models[ct.Content].Kind != ModelAny ||
		!ValidAttributeUseSetID(ct.Attrs, len(rt.AttributeUseSets)) {
		return xsderrors.InternalInvariant("builtin anyType shape does not match handle")
	}
	set := rt.AttributeUseSets[ct.Attrs]
	if len(set.Uses) != 0 || len(set.Index) != 0 || set.Wildcard == NoWildcard ||
		!ValidWildcardID(set.Wildcard, len(rt.Wildcards)) {
		return xsderrors.InternalInvariant("builtin anyType attribute set does not match handle")
	}
	w := rt.Wildcards[set.Wildcard]
	if w.Mode != WildcardAny || w.Process != ProcessLax {
		return xsderrors.InternalInvariant("builtin anyType attribute wildcard does not match handle")
	}
	return nil
}

func validateElementDecl(ctx *schemaValidationContext, decl ElementDecl) error {
	rt := ctx.rt
	if err := ValidateElementDeclRuntime(&rt.Names, NewElementDeclValidationForDecl(decl), DeclRefLimits{
		SimpleTypeCount:  len(rt.SimpleTypes),
		ComplexTypeCount: len(rt.ComplexTypes),
		ElementCount:     len(rt.Elements),
		IdentityCount:    len(rt.Identities),
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	defaultType, err := elementValueConstraintType(rt, decl)
	if err != nil {
		return err
	}
	if err := validateValueConstraintRuntime(ctx, decl.Default, defaultType, "element declaration default"); err != nil {
		return err
	}
	if err := ValidateElementDeclValueConstraintRuntime(rt, defaultType, decl.Default != nil, decl.Fixed != nil); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return validateValueConstraintRuntime(ctx, decl.Fixed, defaultType, "element declaration fixed")
}

func validateAttributeDecl(ctx *schemaValidationContext, decl AttributeDecl) error {
	rt := ctx.rt
	if err := ValidateAttributeDeclRuntime(&rt.Names, NewAttributeDeclValidationForDecl(decl), DeclRefLimits{
		SimpleTypeCount: len(rt.SimpleTypes),
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := ValidateAttributeDeclValueConstraintRuntime(rt, decl.Type, decl.Default != nil, decl.Fixed != nil); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := validateValueConstraintRuntime(ctx, decl.Default, decl.Type, "attribute declaration default"); err != nil {
		return err
	}
	return validateValueConstraintRuntime(ctx, decl.Fixed, decl.Type, "attribute declaration fixed")
}

func validateSimpleType(ctx *schemaValidationContext, id SimpleTypeID, st SimpleType) error {
	rt := ctx.rt
	if err := ValidateSimpleTypeRuntime(&rt.Names, NewSimpleTypeValidationForSimpleType(st), SimpleTypeRefLimits{
		SimpleTypeCount: len(rt.SimpleTypes),
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if ctx.mode == schemaValidationFull {
		if err := ValidateSimpleTypeIdentity(rt, rt.Builtin, id, NewSimpleTypeIdentityNodeForSimpleType(st), st.Identity); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
		if err := ValidateSimpleFastPathForSimpleType(st); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
		if err := validateSimpleTypeDerivation(rt, id, st); err != nil {
			return err
		}
		if err := ValidateFacetLegalityAndConsistencyForSimpleType(st); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
	}
	return nil
}

func validateSimpleTypeGraph(rt *Schema) error {
	if err := ValidateSimpleTypeGraphForSimpleTypes(rt.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateSimpleTypeDerivation(rt *Schema, id SimpleTypeID, st SimpleType) error {
	if !SimpleTypeRestrictionRequired(id, st.Base, rt.Builtin) {
		return nil
	}
	base := rt.SimpleTypes[st.Base]
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

func validateComplexType(ctx *schemaValidationContext, id ComplexTypeID, ct ComplexType, models []ContentModel) error {
	rt := ctx.rt
	if err := ValidateComplexTypeRuntime(&rt.Names, id, ct, models, ComplexTypeRefLimits{
		SimpleTypeCount:      len(rt.SimpleTypes),
		ComplexTypeCount:     len(rt.ComplexTypes),
		AttributeUseSetCount: len(rt.AttributeUseSets),
		AnyType:              rt.Builtin.AnyType,
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if ctx.mode == schemaValidationPublication {
		return nil
	}
	return validateComplexTypeDerivation(rt, id, ct)
}

func validateComplexTypeDerivation(rt *Schema, id ComplexTypeID, ct ComplexType) error {
	if id == rt.Builtin.AnyType {
		return nil
	}
	if baseID, ok := ct.Base.Simple(); ok {
		return validateSimpleBaseComplexDerivation(rt, baseID, ct)
	}
	if err := ValidateComplexTypeDerivationRuntime(rt.Builtin.AnyType, id, ct); err != nil {
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

func validateSimpleBaseComplexDerivation(rt *Schema, baseID SimpleTypeID, ct ComplexType) error {
	if err := ValidateComplexTypeSimpleBaseExtensionRuntime(rt, baseID, ct); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return checkDerivedAttributeWildcard(rt, NoAttributeWildcardState(), NewAttributeWildcardStateForUseSet(rt.AttributeUseSets[ct.Attrs]), AttributeWildcardExtension)
}

func validateComplexExtensionRuntime(rt *Schema, ct ComplexType) error {
	base, err := complexBaseRuntime(rt, ct)
	if err != nil {
		return err
	}
	if err := ValidateComplexTypeExtensionRuntime(rt, base, ct, rt.Builtin.AnyType); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return validateAttributeUsesExtend(rt, base.Attrs, ct.Attrs)
}

func validateComplexRestrictionRuntime(rt *Schema, ct ComplexType) error {
	base, err := complexBaseRuntime(rt, ct)
	if err != nil {
		return err
	}
	if err := ValidateComplexTypeRestrictionRuntime(rt, base, ct); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return validateAttributeUsesRestrict(rt, base.Attrs, ct.Attrs, ct.ExplicitDerivation)
}

func complexBaseRuntime(rt *Schema, ct ComplexType) (ComplexType, error) {
	baseID, err := ComplexTypeDerivationBaseID(ct.Base, len(rt.ComplexTypes))
	if err != nil {
		return ComplexType{}, xsderrors.InternalInvariant(err.Error())
	}
	return rt.ComplexTypes[baseID], nil
}

func validateAttributeUsesExtend(rt *Schema, baseID, derivedID AttributeUseSetID) error {
	base := rt.AttributeUseSets[baseID]
	derived := rt.AttributeUseSets[derivedID]
	if err := ValidateAttributeUseSetExtension(NewAttributeUseExtensionValidationsForUses(base.Uses), NewAttributeUseExtensionValidationsForUses(derived.Uses)); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return checkDerivedAttributeWildcard(rt, NewAttributeWildcardStateForUseSet(base), NewAttributeWildcardStateForUseSet(derived), AttributeWildcardExtension)
}

func validateAttributeUsesRestrict(rt *Schema, baseID, derivedID AttributeUseSetID, bindWildcard bool) error {
	base := rt.AttributeUseSets[baseID]
	derived := rt.AttributeUseSets[derivedID]
	if err := ValidateAttributeUseSetRestriction(
		rt,
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
	if err := ValidateAttributeUseSetRecord(&rt.Names, rt, set); err != nil {
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

func checkDerivedAttributeWildcard(rt *Schema, base, derived AttributeWildcardState, expected AttributeWildcardDerivation) error {
	if err := ValidateAttributeWildcardDerivation(rt, base, derived, expected); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func elementValueConstraintType(rt *Schema, decl ElementDecl) (SimpleTypeID, error) {
	if decl.Default == nil && decl.Fixed == nil {
		return NoSimpleType, nil
	}
	id, err := ElementValueConstraintType(rt, decl.Type)
	if err != nil {
		return NoSimpleType, xsderrors.InternalInvariant(err.Error())
	}
	return id, nil
}

func validateValueConstraintRuntime(ctx *schemaValidationContext, vc *ValueConstraint, expected SimpleTypeID, label string) error {
	if vc == nil {
		return nil
	}
	rt := ctx.rt
	cached := NewValueConstraintValidation(vc)
	if err := ValidateValueConstraintShape(rt, cached, expected); err != nil {
		return xsderrors.InternalInvariant(label + " " + err.Error())
	}
	if expected == NoSimpleType {
		return nil
	}
	if err := ValidateRuntimeSimpleValuePayload(rt, vc.Value, label); err != nil {
		return err
	}
	if ctx.mode == schemaValidationPublication {
		return nil
	}
	return ctx.validateValueConstraintReplay(vc, expected, label, cached)
}

// ValueConstraintSimpleType returns value-constraint validation metadata for a simple type.
func (rt *Schema) ValueConstraintSimpleType(id SimpleTypeID) (ValueConstraintSimpleType, bool) {
	st, ok := rt.SimpleType(id)
	if !ok {
		var zero ValueConstraintSimpleType
		return zero, false
	}
	return NewValueConstraintSimpleTypeForSimpleType(*st), true
}

// ValueConstraintComplexType returns value-constraint validation metadata for a complex type.
func (rt *Schema) ValueConstraintComplexType(id ComplexTypeID) (ValueConstraintComplexType, bool) {
	ct, ok := rt.ComplexType(id)
	if !ok {
		var zero ValueConstraintComplexType
		return zero, false
	}
	return NewValueConstraintComplexTypeForComplexType(*ct), true
}

// ValidateRuntimeSimpleValuePayload validates simple-value payload invariants against runtime metadata.
func ValidateRuntimeSimpleValuePayload(rt *Schema, value SimpleValue, label string) error {
	typ, ok := simpleValuePayloadTypeForRuntime(rt, value.Type)
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
	if !rt.ReadProjectionsPublished() {
		cb := ctx.simpleValueCallbacks
		if cb.Type == nil {
			cb = NewSimpleValueCallbacksForSimpleTypes(rt.SimpleTypes, compilerNotationLookup(rt), nil, xsderrors.IsUnsupported)
			ctx.simpleValueCallbacks = cb
		}
		cb.ResolveQName = ResolveQNameParts(resolve)
		return ValidateSimpleValue(cb, id, lexical, needs)
	}
	return rt.ValidateSimpleValue(id, lexical, ResolveQNameParts(resolve), needs)
}

func simpleValuePayloadTypeForRuntime(rt *Schema, id SimpleTypeID) (SimpleValuePayloadType, bool) {
	if rt.ReadProjectionsPublished() && len(rt.SimpleValueTypeReads) != 0 {
		read, ok := rt.runtimeSimpleValueTypeRead(id)
		if !ok {
			return SimpleValuePayloadType{}, false
		}
		return NewSimpleValuePayloadTypeForType(read.Type, ok)
	}
	st, ok := rt.UsableSimpleType(id)
	if !ok {
		return SimpleValuePayloadType{}, false
	}
	return SimpleValuePayloadType{
		Primitive: st.Primitive,
		Variety:   st.Variety,
		Identity:  st.Identity,
	}, true
}
