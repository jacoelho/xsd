package runtime

import (
	"slices"

	"github.com/jacoelho/xsd/xsderrors"
)

// schemaAudit joins compiler-owned source records with a candidate published
// runtime only while publication invariants are checked. Schema never retains
// this source after publication.
type schemaAudit struct {
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
	if err := validateBuiltins(&ctx); err != nil {
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
	for q, id := range rt.build.GlobalAttributes {
		if !rt.build.Names.ValidQName(q) || !ValidAttributeID(id, len(rt.build.Attributes)) {
			return xsderrors.InternalInvariant("global attribute references invalid declaration")
		}
		if rt.build.Attributes[id].Name != q {
			return xsderrors.InternalInvariant("global attribute name does not match declaration")
		}
	}
	for q, id := range rt.build.GlobalElements {
		if !rt.build.Names.ValidQName(q) || !ValidElementID(id, len(rt.build.Elements)) {
			return xsderrors.InternalInvariant("global element references invalid declaration")
		}
		if rt.build.Elements[id].Name != q {
			return xsderrors.InternalInvariant("global element name does not match declaration")
		}
	}
	for q, typ := range rt.build.GlobalTypes {
		name, ok := TypeNameByID(rt.build.SimpleTypes, rt.build.ComplexTypes, typ)
		if !rt.build.Names.ValidQName(q) || !ok {
			return xsderrors.InternalInvariant("global type references invalid declaration")
		}
		if name != q {
			return xsderrors.InternalInvariant("global type name does not match declaration")
		}
	}
	for q, id := range rt.build.GlobalIdentities {
		if !rt.build.Names.ValidQName(q) || !ValidIdentityConstraintID(id, len(rt.build.Identities)) {
			return xsderrors.InternalInvariant("global identity references invalid declaration")
		}
		if rt.build.Identities[id].Name != q {
			return xsderrors.InternalInvariant("global identity name does not match declaration")
		}
	}
	for q := range rt.build.Notations {
		if !rt.build.Names.ValidQName(q) {
			return xsderrors.InternalInvariant("notation references invalid name")
		}
	}
	return nil
}

func validateRuntimeSubstitutions(rt *schemaAudit) error {
	if err := ValidateSubstitutionMaps(
		&rt.build,
		&rt.build.Names,
		rt.build.Elements,
		rt.build.GlobalElements,
		rt.build.Substitutions,
		rt.build.SubstitutionIndex.byName,
	); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeComponents(ctx *schemaValidationContext) error {
	rt := ctx.rt
	for i := range rt.build.SimpleTypes {
		if err := validateSimpleType(ctx, SimpleTypeID(i), rt.build.SimpleTypes[i]); err != nil {
			return err
		}
	}
	if err := validateSimpleTypeGraph(rt); err != nil {
		return err
	}
	if err := validateCompiledFacetLiterals(ctx); err != nil {
		return err
	}
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
	for i := range rt.build.Wildcards {
		if err := ValidateWildcard(&rt.build.Names, rt.build.Wildcards[i]); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
	}
	for i := range rt.build.AttributeUseSets {
		if err := validateAttributeUseSetRuntime(ctx, AttributeUseSetID(i), rt.build.AttributeUseSets[i]); err != nil {
			return err
		}
	}
	contentModelLimits := ContentModelRefLimits{
		ElementCount:      len(rt.build.Elements),
		ContentModelCount: len(rt.build.Models),
		WildcardCount:     len(rt.build.Wildcards),
	}
	for i := range rt.build.Models {
		if err := ValidateContentModelRuntime(rt.build.Models[i], contentModelLimits); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
	}
	if err := validateContentModelGraph(rt.build.Models); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	for i := range rt.build.ComplexTypes {
		if err := validateComplexTypeRecord(&rt.build, ComplexTypeID(i), rt.build.ComplexTypes[i]); err != nil {
			return err
		}
	}
	if err := validateComplexTypeGraph(rt.build.ComplexTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	for i := range rt.build.ComplexTypes {
		if err := validateComplexTypeDerivation(rt, ComplexTypeID(i), rt.build.ComplexTypes[i]); err != nil {
			return err
		}
	}
	if err := ValidateIdentityConstraints(&rt.build.Names, rt.build.Identities); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	for i := range rt.build.Elements {
		if err := validateElementDeclValueConstraints(ctx, rt.build.Elements[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeReadProjections(rt *schemaAudit) error {
	if err := validateAttributeDeclReads(rt); err != nil {
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
	if err := validateComplexTypeReads(rt); err != nil {
		return err
	}
	if err := validateWildcardReads(rt); err != nil {
		return err
	}
	return validateCompiledModelReads(rt)
}

func validateAttributeDeclReads(rt *schemaAudit) error {
	if err := ValidateAttributeDeclReadProjectionForDecls(rt.runtime.Attributes, rt.build.Attributes); err != nil {
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

func validateElementValueConstraintReads(rt *schemaAudit) error {
	if err := ValidateElementValueConstraintReadProjectionForDecls(rt.runtime.ElementValueConstraints, rt.build.Elements); err != nil {
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

func validateSimpleValueReads(rt *schemaAudit) error {
	if err := validateSimpleValueRouteReadProjectionForTypes(rt.runtime.SimpleValueRoutes, rt.build.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := validateSimpleValueColdReadProjectionForTypes(rt.runtime.SimpleValueCold, rt.build.SimpleTypes); err != nil {
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

func validateElementNames(rt *schemaAudit) error {
	if err := ValidateElementNameReadProjectionForDecls(rt.runtime.ElementNames, rt.build.Elements); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateTypeDerivations(rt *schemaAudit) error {
	if err := ValidateTypeDerivationReadProjection(rt.runtime.TypeDerivations, rt.build.Builtin.AnyType, rt.build.SimpleTypes, rt.build.ComplexTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateElementStartInfos(rt *schemaAudit) error {
	if err := ValidateElementStartInfosForElementDecls(rt.runtime.ElementStarts, rt.build.Elements); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateElementIdentityConstraintReads(rt *schemaAudit) error {
	if err := ValidateElementIdentityConstraintReadProjectionForDecls(rt.runtime.ElementIdentities, rt.build.Elements); err != nil {
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
	if err := validateCompiledModelReadProjectionTable(rt.runtime.CompiledModels, rt.build.CompiledModels); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeCompiledModels(rt *schemaAudit) error {
	if err := validateCompiledModelsRuntime(&rt.build.Names, &rt.build, rt.build.Models, rt.build.CompiledModels, true); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeChoiceLimits(rt *schemaAudit) error {
	if err := ValidateChoiceLimitDerivations(
		&rt.build,
		rt.build.ComplexTypes,
		rt.build.Models,
		rt.build.Builtin.AnyType,
	); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateBuiltins(ctx *schemaValidationContext) error {
	return validateBuiltinIDs(ctx)
}

func validateBuiltinIDs(ctx *schemaValidationContext) error {
	rt := ctx.rt
	if err := validateBuiltinDeclarations(rt); err != nil {
		return err
	}
	for _, base := range builtinSimpleExpectationTable {
		exp := builtinSimpleExpectationWithBuiltins(base, rt.build.Builtin)
		id, err := validateBuiltinSimpleTypeInSchema(rt, exp)
		if err != nil {
			return err
		}
		st := rt.build.SimpleTypes[id]
		if err := validateBuiltinSimpleFacets(st, exp.facetExpectation(st.Base)); err != nil {
			return err
		}
		for _, literal := range st.Facets.bounds {
			if literal == nil {
				continue
			}
			if ctx.validatedBoundLiterals == nil {
				ctx.validatedBoundLiterals = make(map[*CompiledLiteral]struct{})
			}
			ctx.validatedBoundLiterals[literal] = struct{}{}
		}
	}
	return validateBuiltinAnyType(rt)
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
		exp := builtinAttributeExpectationForSeed(seed, rt.build.Builtin)
		q, ok := builtinAttributeQName(&rt.build.Names, exp)
		if !ok {
			return xsderrors.InternalInvariant("builtin attribute name is missing")
		}
		id, ok := rt.build.GlobalAttributes[q]
		if !ok || !ValidAttributeID(id, len(rt.build.Attributes)) || rt.build.Attributes[id].Name != q {
			return xsderrors.InternalInvariant("builtin attribute binding does not match declaration")
		}
		typ := rt.build.Attributes[id].Type
		if exp.builtin == BuiltinValidationNone {
			if typ != exp.typ {
				return xsderrors.InternalInvariant("builtin attribute type does not match handle")
			}
			continue
		}
		if !ValidSimpleTypeID(typ, len(rt.build.SimpleTypes)) || rt.build.SimpleTypes[typ].Builtin != exp.builtin {
			return xsderrors.InternalInvariant("builtin attribute type does not match lexical validator")
		}
	}
	return nil
}

func validateBuiltinSimpleTypeInSchema(rt *schemaAudit, exp builtinSimpleExpectation) (SimpleTypeID, error) {
	if exp.checkID && !ValidUint32Index(uint32(exp.id), len(rt.build.SimpleTypes)) {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type references invalid declaration")
	}
	q, ok := builtinSimpleQName(&rt.build.Names, exp.local)
	if !ok {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type name is missing")
	}
	typ, ok := rt.build.GlobalTypes[q]
	id, simple := typ.Simple()
	if !ok || !simple {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type handle does not match global type")
	}
	if exp.checkID && id != exp.id {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type handle does not match global type")
	}
	if !ValidSimpleTypeID(id, len(rt.build.SimpleTypes)) {
		return NoSimpleType, xsderrors.InternalInvariant("builtin simple type references invalid declaration")
	}
	st := rt.build.SimpleTypes[id]
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
		IdentityCount:    len(rt.build.Identities),
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
	if err := ValidateFacetLegalityAndConsistencyForSimpleType(st); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateSimpleTypeGraph(rt *schemaAudit) error {
	if err := ValidateSimpleTypeGraphForSimpleTypes(rt.build.SimpleTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateCompiledFacetLiterals(ctx *schemaValidationContext) error {
	types := ctx.rt.build.SimpleTypes
	ancestry := newSimpleTypeBaseAncestry(types)
	var enumerations map[simpleValueEnumerationSource]SimpleTypeID
	for i := range ctx.rt.build.SimpleTypes {
		owner := SimpleTypeID(i)
		facets := &types[i].Facets
		for _, literal := range facets.bounds {
			if literal == nil {
				continue
			}
			if !ancestry.strictAncestor(literal.Type, owner) {
				return xsderrors.InternalInvariant("facet literal compilation type is not an owner ancestor")
			}
			if err := ctx.validateCompiledBoundLiteralOnce(literal); err != nil {
				return err
			}
		}
		if len(facets.Enumeration) == 0 {
			continue
		}
		source, _ := simpleValueEnumerationSourceForLiterals(facets.Enumeration)
		compilationType, audited := enumerations[source]
		if !audited {
			compilationType = facets.Enumeration[0].Type
		}
		if !ancestry.strictAncestor(compilationType, owner) {
			return xsderrors.InternalInvariant("facet literal compilation type is not an owner ancestor")
		}
		if !audited {
			if enumerations == nil {
				enumerations = make(map[simpleValueEnumerationSource]SimpleTypeID)
			}
			for j := range facets.Enumeration {
				literal := &facets.Enumeration[j]
				if literal.Type != compilationType {
					return xsderrors.InternalInvariant("facet enumeration has mixed compilation types")
				}
				if err := ctx.validateCompiledFacetLiteral(*literal); err != nil {
					return err
				}
			}
			enumerations[source] = compilationType
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

	ancestry := simpleTypeBaseAncestry{start: make([]int, len(types)), end: make([]int, len(types))}
	stack := make([]simpleTypeAncestryFrame, 0, min(len(types), 1_024))
	clock := 0
	for i := range types {
		if types[i].Base != NoSimpleType {
			continue
		}
		stack = appendDFSFrame(stack, simpleTypeAncestryFrame{id: SimpleTypeID(i)}, len(types))
		for len(stack) != 0 {
			last := len(stack) - 1
			frame := &stack[last]
			if !frame.entered {
				ancestry.start[frame.id] = clock
				clock++
				frame.nextChild = firstChild[frame.id]
				frame.entered = true
			}
			if frame.nextChild == NoSimpleType {
				ancestry.end[frame.id] = clock
				stack = stack[:last]
				continue
			}
			child := frame.nextChild
			frame.nextChild = nextSibling[child]
			stack = appendDFSFrame(stack, simpleTypeAncestryFrame{id: child}, len(types))
		}
	}
	return ancestry
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
	if err := ValidateComplexTypeRestrictionRuntime(&rt.build, base, ct); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return validateAttributeUsesRestrict(rt, base.Attrs, ct.Attrs, ct.ExplicitDerivation)
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
	id, err := ElementValueConstraintType(&rt.build, decl.Type)
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
