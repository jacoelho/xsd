package schema

import (
	"errors"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// schemaValidation owns the source-backed semantic checks performed before a
// schema program is sealed. It never escapes publication.
type schemaValidation struct {
	contentModelWork contentModelWork
	contentAnalysis  *contentModelAnalysis
	build            *schemaBuild
}

func validateSchema(rt *schemaValidation) error {
	if err := validateNameTable(rt); err != nil {
		return err
	}
	if err := validateRuntimeGlobals(rt); err != nil {
		return err
	}
	if err := validateBuiltinIDs(rt); err != nil {
		return err
	}
	return validateRuntimeComponents(rt)
}

// validateSchemaBuildOwnership checks the compiler's reverse indexes before
// publication. The publication program only retains forward read views, so a
// missing or duplicate owner binding must be rejected while the mutable build
// is still available and untouched for retry.
func validateSchemaBuildOwnership(build *schemaBuild) error {
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

func validateAttributeBuildOwnership(build *schemaBuild) error {
	for i, decl := range build.Attributes {
		id, ok := build.GlobalAttributes[decl.Name]
		if !ok || id != AttributeID(i) {
			return xsderrors.InternalInvariant("global attribute declaration is missing its exact name binding")
		}
	}
	return nil
}

func validateElementBuildOwnership(build *schemaBuild) error {
	for i, decl := range build.Elements {
		id, bound := build.GlobalElements[decl.Name]
		exact := bound && id == ElementID(i)
		switch decl.Scope { //nolint:exhaustive // default rejects invalid and future scope values.
		case declarationScopeGlobal:
			if !exact {
				return xsderrors.InternalInvariant("global element declaration is missing its exact name binding")
			}
		case declarationScopeNonGlobal:
			if exact {
				return xsderrors.InternalInvariant("non-global element declaration has a global name binding")
			}
		default:
			return xsderrors.InternalInvariant("element declaration scope is invalid")
		}
	}
	return nil
}

func validateSimpleTypeBuildOwnership(build *schemaBuild) error {
	for i, typ := range build.SimpleTypes {
		id, bound := build.GlobalTypes[typ.Name]
		exact := bound && id == SimpleRef(SimpleTypeID(i))
		switch typ.Scope { //nolint:exhaustive // default rejects invalid and future scope values.
		case declarationScopeGlobal:
			if !exact {
				return xsderrors.InternalInvariant("global simple type is missing its exact name binding")
			}
		case declarationScopeNonGlobal:
			if exact {
				return xsderrors.InternalInvariant("non-global simple type has a global name binding")
			}
		default:
			return xsderrors.InternalInvariant("simple type declaration scope is invalid")
		}
	}
	return nil
}

func validateComplexTypeBuildOwnership(build *schemaBuild) error {
	for i, typ := range build.ComplexTypes {
		id, bound := build.GlobalTypes[typ.Name]
		exact := bound && id == ComplexRef(ComplexTypeID(i))
		switch typ.Scope { //nolint:exhaustive // default rejects invalid and future scope values.
		case declarationScopeGlobal:
			if !exact {
				return xsderrors.InternalInvariant("global complex type is missing its exact name binding")
			}
		case declarationScopeNonGlobal:
			if exact {
				return xsderrors.InternalInvariant("non-global complex type has a global name binding")
			}
		default:
			return xsderrors.InternalInvariant("complex type declaration scope is invalid")
		}
	}
	return nil
}

func validateIdentityBuildOwnership(build *schemaBuild) error {
	for i, identity := range build.Identities {
		id, ok := build.GlobalIdentities[identity.Name]
		if !ok || id != IdentityConstraintID(i) {
			return xsderrors.InternalInvariant("identity constraint is missing its exact name binding")
		}
	}
	return nil
}

func validateNameTable(rt *schemaValidation) error {
	if err := validateRuntimeNameTable(&rt.build.Names); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeGlobals(rt *schemaValidation) error {
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

func validateGlobalAttributes(rt *schemaValidation) error {
	for q, id := range rt.build.GlobalAttributes {
		if !rt.build.Names.ValidQName(q) || !validAttributeID(id, len(rt.build.Attributes)) {
			return xsderrors.InternalInvariant("global attribute references invalid declaration")
		}
		if rt.build.Attributes[id].Name != q {
			return xsderrors.InternalInvariant("global attribute name does not match declaration")
		}
	}
	return nil
}

func validateGlobalElements(rt *schemaValidation) error {
	for q, id := range rt.build.GlobalElements {
		if !rt.build.Names.ValidQName(q) || !validElementID(id, len(rt.build.Elements)) {
			return xsderrors.InternalInvariant("global element references invalid declaration")
		}
		if rt.build.Elements[id].Name != q {
			return xsderrors.InternalInvariant("global element name does not match declaration")
		}
	}
	return nil
}

func validateGlobalTypes(rt *schemaValidation) error {
	for q, typ := range rt.build.GlobalTypes {
		name, ok := typeNameByID(rt.build.SimpleTypes, rt.build.ComplexTypes, typ)
		if !rt.build.Names.ValidQName(q) || !ok {
			return xsderrors.InternalInvariant("global type references invalid declaration")
		}
		if name != q {
			return xsderrors.InternalInvariant("global type name does not match declaration")
		}
	}
	return nil
}

func validateGlobalIdentities(rt *schemaValidation) error {
	for q, id := range rt.build.GlobalIdentities {
		if !rt.build.Names.ValidQName(q) || !validIdentityConstraintID(id, len(rt.build.Identities)) {
			return xsderrors.InternalInvariant("global identity references invalid declaration")
		}
		if rt.build.Identities[id].Name != q {
			return xsderrors.InternalInvariant("global identity name does not match declaration")
		}
	}
	return nil
}

func validateGlobalNotations(rt *schemaValidation) error {
	for q := range rt.build.Notations {
		if !rt.build.Names.ValidQName(q) {
			return xsderrors.InternalInvariant("notation references invalid name")
		}
	}
	return nil
}

func validateRuntimeComponents(rt *schemaValidation) error {
	if err := validateRuntimeSimpleComponents(rt); err != nil {
		return err
	}
	if err := validateRuntimeDeclarationComponents(rt); err != nil {
		return err
	}
	return validateRuntimeComplexComponents(rt)
}

func validateRuntimeSimpleComponents(rt *schemaValidation) error {
	if err := validateMissingSimpleType(rt); err != nil {
		return err
	}
	for i := range rt.build.SimpleTypes {
		if err := validateSimpleType(rt, SimpleTypeID(i), rt.build.SimpleTypes[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeDeclarationComponents(rt *schemaValidation) error {
	if err := validateRuntimeElementAndAttributeComponents(rt); err != nil {
		return err
	}
	return validateRuntimeContentModelComponents(rt)
}

func validateRuntimeElementAndAttributeComponents(rt *schemaValidation) error {
	if err := validateRuntimeElementAndAttributeRecords(rt); err != nil {
		return err
	}
	if err := validateRuntimeWildcards(rt); err != nil {
		return err
	}
	return validateRuntimeAttributeUseSets(rt)
}

func validateRuntimeElementAndAttributeRecords(rt *schemaValidation) error {
	for i := range rt.build.Elements {
		if err := validateElementDeclShape(rt, rt.build.Elements[i]); err != nil {
			return err
		}
	}
	for i := range rt.build.Attributes {
		if err := validateAttributeDecl(rt, rt.build.Attributes[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeWildcards(rt *schemaValidation) error {
	for i := range rt.build.Wildcards {
		if err := validateWildcard(&rt.build.Names, rt.build.Wildcards[i]); err != nil {
			return xsderrors.InternalInvariant(err.Error())
		}
	}
	return nil
}

func validateRuntimeAttributeUseSets(rt *schemaValidation) error {
	for i := range rt.build.AttributeUseSets {
		if err := validateAttributeUseSetRuntime(rt, attributeUseSetID(i), rt.build.AttributeUseSets[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeContentModelComponents(rt *schemaValidation) error {
	contentModelLimits := contentModelRefLimits{
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
	checker := newElementDeclarationConsistencyChecker(rt.build, rt.contentModelWork)
	for id := range rt.build.Models {
		if err := checker.checkModel(contentModelID(id)); err != nil {
			return schemaBuildValidationError(err)
		}
	}
	return nil
}

func validateRuntimeContentModel(rt *schemaValidation, model contentModelNode, limits contentModelRefLimits) error {
	if err := spendContentModelWork(rt.contentModelWork); err != nil {
		return err
	}
	for range model.Particles {
		if err := spendContentModelWork(rt.contentModelWork); err != nil {
			return err
		}
	}
	if err := validateContentModelRuntime(model, limits); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeComplexComponents(rt *schemaValidation) error {
	if err := validateRuntimeComplexTypeRecords(rt); err != nil {
		return err
	}
	if err := validateRuntimeComplexTypeDerivations(rt); err != nil {
		return err
	}
	if err := validateIdentityConstraints(&rt.build.Names, rt.build.Identities); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return validateRuntimeElementValueConstraints(rt)
}

func validateRuntimeComplexTypeRecords(rt *schemaValidation) error {
	for i := range rt.build.ComplexTypes {
		if err := validateComplexTypeRecord(rt.build, ComplexTypeID(i), rt.build.ComplexTypes[i]); err != nil {
			return err
		}
	}
	if err := validateComplexTypeGraph(rt.build.ComplexTypes); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateRuntimeComplexTypeDerivations(rt *schemaValidation) error {
	for i := range rt.build.ComplexTypes {
		if err := validateComplexTypeDerivation(rt, ComplexTypeID(i), rt.build.ComplexTypes[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeElementValueConstraints(rt *schemaValidation) error {
	for i := range rt.build.Elements {
		if err := validateElementDeclValueConstraints(rt, rt.build.Elements[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateMissingSimpleType(rt *schemaValidation) error {
	missingID, err := missingSimpleTypeID(rt)
	if err != nil || missingID == NoSimpleType {
		return err
	}
	if err := validateMissingSimpleTypeReferences(rt, missingID); err != nil {
		return err
	}
	return validateMissingSimpleTypeGlobals(rt, missingID)
}

func missingSimpleTypeID(rt *schemaValidation) (SimpleTypeID, error) {
	missingName, hasMissingName := rt.build.Names.LookupQName(vocab.EmptyNamespaceURI, missingSimpleTypeLocalName())
	missingID := NoSimpleType
	for i := range rt.build.SimpleTypes {
		st := rt.build.SimpleTypes[i]
		if !st.Missing {
			continue
		}
		expected := missingSimpleType(missingName, value.AnySimpleTypeID)
		expected.Scope = declarationScopeNonGlobal
		if missingID != NoSimpleType || !hasMissingName || st.Name != missingName || !intrinsicSimpleTypeEqual(st, expected) {
			return NoSimpleType, xsderrors.InternalInvariant("missing simple type sentinel is invalid")
		}
		missingID = SimpleTypeID(i)
	}
	return missingID, nil
}

// intrinsicSimpleTypeEqual compares a declaration with an intrinsic
// definition field by field. A test guard over the struct field sets fails when
// simpleType or its value specification gains a field this does not compare.
func intrinsicSimpleTypeEqual(got, want simpleType) bool {
	return got.Name == want.Name && got.Final == want.Final && got.Missing == want.Missing &&
		got.Scope == want.Scope && got.unionContainsList == want.unionContainsList &&
		got.unionHasFacets == want.unionHasFacets && intrinsicTypeSpecEqual(got.ValueSpec, want.ValueSpec)
}

// intrinsicTypeSpecEqual compares facet-free, member-free specifications.
// Intrinsic definitions have neither, so any union member or facet on either
// side is a difference.
func intrinsicTypeSpecEqual(got, want value.TypeSpec) bool {
	return len(got.Union) == 0 && len(want.Union) == 0 &&
		facetSpecEmpty(got.Facets) && facetSpecEmpty(want.Facets) &&
		got.Base == want.Base && got.ListItem == want.ListItem && got.Variety == want.Variety &&
		got.Primitive == want.Primitive && got.Whitespace == want.Whitespace &&
		got.WhitespacePresent == want.WhitespacePresent && got.Builtin == want.Builtin
}

func facetSpecEmpty(f value.FacetSpec) bool {
	var none value.CardinalityFacet
	return len(f.Patterns) == 0 && len(f.Enumeration) == 0 &&
		boundFacetEmpty(f.MinInclusive) && boundFacetEmpty(f.MaxExclusive) &&
		boundFacetEmpty(f.MinExclusive) && boundFacetEmpty(f.MaxInclusive) &&
		f.MinLength == none && f.FractionDigits == none && f.TotalDigits == none &&
		f.MaxLength == none && f.Length == none && f.Present == 0 && f.Fixed == 0
}

func boundFacetEmpty(b value.BoundFacet) bool {
	return !b.Present && b.Lexical == "" && b.Type == 0 &&
		b.Resolver.QName == nil && b.Resolver.Notation == nil
}

func validateMissingSimpleTypeReferences(rt *schemaValidation, missingID SimpleTypeID) error {
	for _, st := range rt.build.SimpleTypes {
		if !st.Missing && st.ValueSpec.Base == missingID {
			return xsderrors.InternalInvariant("missing simple type sentinel has invalid type reference")
		}
	}
	return validateMissingSimpleTypeComplexReferences(rt, missingID)
}

func validateMissingSimpleTypeComplexReferences(rt *schemaValidation, missingID SimpleTypeID) error {
	for _, ct := range rt.build.ComplexTypes {
		base, simpleBase := ct.Base.Simple()
		if ct.TextType == missingID || (simpleBase && base == missingID) {
			return xsderrors.InternalInvariant("missing simple type sentinel has invalid complex type reference")
		}
	}
	return nil
}

func validateMissingSimpleTypeGlobals(rt *schemaValidation, missingID SimpleTypeID) error {
	for _, typ := range rt.build.GlobalTypes {
		if simple, ok := typ.Simple(); ok && simple == missingID {
			return xsderrors.InternalInvariant("missing simple type sentinel is globally registered")
		}
	}
	return nil
}

func validateBuiltinIDs(rt *schemaValidation) error {
	if err := validateBuiltinDeclarations(rt); err != nil {
		return err
	}
	for id := range value.BuiltinTypeCount {
		if err := validateBuiltinSimpleBinding(rt, id); err != nil {
			return err
		}
	}
	if err := validateBuiltinXSIAttributes(rt); err != nil {
		return err
	}
	return validateBuiltinAnyType(rt)
}

func validateBuiltinXSIAttributes(rt *schemaValidation) error {
	for _, attribute := range builtinXSIAttributes {
		q, ok := rt.build.Names.LookupQName(vocab.XSINamespaceURI, attribute.local)
		if !ok {
			return xsderrors.InternalInvariant("intrinsic XSI attribute name is missing")
		}
		id, ok := rt.build.GlobalAttributes[q]
		if !ok || !validAttributeID(id, len(rt.build.Attributes)) {
			return xsderrors.InternalInvariant("intrinsic XSI attribute binding is missing")
		}
		decl := rt.build.Attributes[id]
		if decl.Name != q || decl.Type != attribute.typ || decl.Default != nil || decl.Fixed != nil {
			return xsderrors.InternalInvariant("intrinsic XSI attribute declaration differs from its definition")
		}
	}
	return validateBuiltinSchemaLocationType(rt)
}

func validateBuiltinSchemaLocationType(rt *schemaValidation) error {
	if !validSimpleTypeID(builtinSchemaLocationTypeID, len(rt.build.SimpleTypes)) {
		return xsderrors.InternalInvariant("intrinsic schemaLocation type is missing")
	}
	list := rt.build.SimpleTypes[builtinSchemaLocationTypeID]
	name, ok := rt.build.Names.LookupQName(vocab.XSINamespaceURI, "$schemaLocation")
	if !ok || list.Name != name || list.Scope != declarationScopeNonGlobal || list.Final != 0 || list.Missing ||
		!intrinsicTypeSpecEqual(list.ValueSpec, builtinSchemaLocationTypeSpec()) {
		return xsderrors.InternalInvariant("intrinsic schemaLocation type differs from its definition")
	}
	view, ok := rt.build.valueBuilder.TypeView(builtinSchemaLocationTypeID)
	if !ok || view.Variety != value.List || view.Primitive != value.PrimitiveString ||
		view.Base != value.AnySimpleTypeID || view.ListItem != value.BuiltinType(value.PrimitiveAnyURI) ||
		view.Whitespace != value.WhitespaceCollapse || view.Builtin != value.BuiltinNone ||
		len(view.Union) != 0 || view.HasEnumeration {
		return xsderrors.InternalInvariant("intrinsic schemaLocation value metadata differs from its definition")
	}
	return nil
}

func validateBuiltinSimpleBinding(rt *schemaValidation, id SimpleTypeID) error {
	name, ok := value.BuiltinTypeName(id)
	if !ok || !validSimpleTypeID(id, len(rt.build.SimpleTypes)) {
		return xsderrors.InternalInvariant("builtin simple type declaration is missing")
	}
	q, ok := rt.build.Names.LookupQName(vocab.XSDNamespaceURI, name)
	if !ok {
		return xsderrors.InternalInvariant("builtin simple type name is missing")
	}
	simple := rt.build.SimpleTypes[id]
	if simple.Name != q || simple.Scope != declarationScopeGlobal {
		return xsderrors.InternalInvariant("builtin simple type declaration does not match its value definition")
	}
	if rt.build.GlobalTypes[q] != SimpleRef(id) {
		return xsderrors.InternalInvariant("builtin simple type global binding does not match its value definition")
	}
	return nil
}

func validateBuiltinDeclarations(rt *schemaValidation) error {
	if err := validateBuiltinDeclarationCounts(builtinDeclarationCounts{
		SimpleTypes:      len(rt.build.SimpleTypes),
		Attributes:       len(rt.build.Attributes),
		ComplexTypes:     len(rt.build.ComplexTypes),
		Wildcards:        len(rt.build.Wildcards),
		AttributeUseSets: len(rt.build.AttributeUseSets),
		Models:           len(rt.build.Models),
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func validateBuiltinAnyType(rt *schemaValidation) error {
	anyType := rt.build.AnyType
	if !validComplexTypeID(anyType, len(rt.build.ComplexTypes)) {
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
		ct.ContentKind != contentMixed ||
		ct.TextType != NoSimpleType ||
		!validContentModelID(ct.Content, len(rt.build.Models)) ||
		rt.build.Models[ct.Content].Kind != modelAny ||
		!validAttributeUseSetID(ct.Attrs, len(rt.build.AttributeUseSets)) {
		return xsderrors.InternalInvariant("builtin anyType shape does not match handle")
	}
	set := rt.build.AttributeUseSets[ct.Attrs]
	if len(set.Uses) != 0 || len(set.Index) != 0 || set.Wildcard == NoWildcard ||
		!validWildcardID(set.Wildcard, len(rt.build.Wildcards)) {
		return xsderrors.InternalInvariant("builtin anyType attribute set does not match handle")
	}
	w := rt.build.Wildcards[set.Wildcard]
	if w.Mode != WildcardAny || w.Process != ProcessLax {
		return xsderrors.InternalInvariant("builtin anyType attribute wildcard does not match handle")
	}
	return nil
}

func validateElementDeclShape(rt *schemaValidation, decl elementDecl) error {
	if err := validateElementDeclRuntime(&rt.build.Names, decl, declRefLimits{
		SimpleTypeCount:  len(rt.build.SimpleTypes),
		ComplexTypeCount: len(rt.build.ComplexTypes),
		ElementCount:     len(rt.build.Elements),
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if simple, ok := decl.Type.Simple(); ok {
		return validateNotationTypeUse(rt.build, simple)
	}
	return nil
}

func validateElementDeclValueConstraints(rt *schemaValidation, decl elementDecl) error {
	defaultType, err := auditElementValueConstraintType(rt, decl)
	if err != nil {
		return err
	}
	if err := validateValueConstraintRuntime(rt, decl.Default, defaultType, "element declaration default"); err != nil {
		return err
	}
	if err := validateElementDeclValueConstraintRuntime(rt.build, defaultType, declarationValueConstraintOf(decl.Default, decl.Fixed)); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return validateValueConstraintRuntime(rt, decl.Fixed, defaultType, "element declaration fixed")
}

func validateAttributeDecl(rt *schemaValidation, decl attributeDecl) error {
	if err := validateAttributeDeclRuntime(&rt.build.Names, decl, declRefLimits{
		SimpleTypeCount: len(rt.build.SimpleTypes),
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := checkNotationTypeUse(rt.build, decl.Type); err != nil {
		return schemaBuildValidationError(err)
	}
	if err := validateAttributeDeclValueConstraintRuntime(rt.build, decl.Type, declarationValueConstraintOf(decl.Default, decl.Fixed)); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := validateValueConstraintRuntime(rt, decl.Default, decl.Type, "attribute declaration default"); err != nil {
		return err
	}
	return validateValueConstraintRuntime(rt, decl.Fixed, decl.Type, "attribute declaration fixed")
}

func validateSimpleType(rt *schemaValidation, id SimpleTypeID, st simpleType) error {
	if !rt.build.Names.ValidQName(st.Name) {
		return xsderrors.InternalInvariant("simple type references invalid name")
	}
	if !validSimpleFinalMask(st.Final) {
		return xsderrors.InternalInvariant("simple type final mask contains invalid derivation")
	}
	if rt.build.valueBuilder == nil {
		return xsderrors.InternalInvariant("simple type has no value builder")
	}
	if _, ok := rt.build.valueBuilder.InputRequirements(id); !ok {
		return xsderrors.InternalInvariant("simple type has no admitted value definition")
	}
	constrained, ok := rt.build.valueBuilder.UnionHasFacets(id)
	if !ok || constrained != st.unionHasFacets {
		return xsderrors.InternalInvariant("simple type union facets differ from value definition")
	}
	return validateNotationConstructorUses(rt.build, id)
}

func validateComplexTypeRecord(build *schemaBuild, id ComplexTypeID, ct complexTypeMetadata) error {
	if err := validateComplexTypeRuntime(&build.Names, id, ct, build.Models, complexTypeRefLimits{
		SimpleTypeCount:      len(build.SimpleTypes),
		ComplexTypeCount:     len(build.ComplexTypes),
		AttributeUseSetCount: len(build.AttributeUseSets),
		AnyType:              build.AnyType,
	}); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if ct.TextType != NoSimpleType {
		return validateNotationTypeUse(build, ct.TextType)
	}
	return nil
}

func validateComplexTypeDerivation(rt *schemaValidation, id ComplexTypeID, ct complexTypeMetadata) error {
	if id == rt.build.AnyType {
		return nil
	}
	if baseID, ok := ct.Base.Simple(); ok {
		return validateSimpleBaseComplexDerivation(rt, baseID, ct)
	}
	if err := validateComplexTypeDerivationRuntime(rt.build.AnyType, id, ct); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	switch ct.Derivation {
	case derivationKindExtension:
		return validateComplexExtensionRuntime(rt, ct)
	case derivationKindRestriction:
		return validateComplexRestrictionRuntime(rt, ct)
	case derivationKindNone:
		return xsderrors.InternalInvariant("complex derivation mode was not handled")
	}
	return nil
}

func validateSimpleBaseComplexDerivation(rt *schemaValidation, baseID SimpleTypeID, ct complexTypeMetadata) error {
	if err := validateComplexTypeSimpleBaseExtensionRuntime(rt.build, baseID, ct); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return checkDerivedAttributeWildcard(rt, attributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard}, rt.build.AttributeUseSets[ct.Attrs], attributeWildcardExtension)
}

func validateComplexExtensionRuntime(rt *schemaValidation, ct complexTypeMetadata) error {
	base, err := complexBaseRuntime(rt, ct)
	if err != nil {
		return err
	}
	if err := validateComplexTypeExtensionRuntime(rt.build, base, ct, rt.build.AnyType); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := validateComplexExtensionAttributeAncestry(rt.build, ct); err != nil {
		return schemaBuildValidationError(err)
	}
	return validateAttributeUsesExtend(rt, base.Attrs, ct.Attrs)
}

func validateComplexRestrictionRuntime(rt *schemaValidation, ct complexTypeMetadata) error {
	base, err := complexBaseRuntime(rt, ct)
	if err != nil {
		return err
	}
	if err := validateComplexTypeRestrictionRuntime(rt.build, rt.contentAnalysis, base, ct, rt.build.spendTypeDerivationWork); err != nil {
		return schemaBuildValidationError(err)
	}
	if err := validateComplexRestrictionAttributeWildcard(rt.build, ct); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if err := validateContentRestriction(
		rt.build,
		base.Content,
		ct.Content,
		rt.contentModelWork,
		rt.contentAnalysis,
	); err != nil {
		return schemaBuildValidationError(err)
	}
	binding := attributeWildcardUnbound
	if ct.ExplicitDerivation {
		binding = attributeWildcardBound
	}
	return validateAttributeUsesRestrict(rt, base.Attrs, ct.Attrs, binding)
}

func schemaBuildValidationError(err error) error {
	if diagnostic, ok := errors.AsType[*xsderrors.Error](err); ok && diagnostic != nil {
		return err
	}
	return xsderrors.InternalInvariant(err.Error())
}

func complexBaseRuntime(rt *schemaValidation, ct complexTypeMetadata) (complexTypeMetadata, error) {
	baseID, err := complexTypeDerivationBaseID(ct.Base, len(rt.build.ComplexTypes))
	if err != nil {
		return complexTypeMetadata{}, xsderrors.InternalInvariant(err.Error())
	}
	return rt.build.ComplexTypes[baseID], nil
}

func validateAttributeUsesExtend(rt *schemaValidation, baseID, derivedID attributeUseSetID) error {
	base := rt.build.AttributeUseSets[baseID]
	derived := rt.build.AttributeUseSets[derivedID]
	if err := validateAttributeUseSetExtension(base, derived); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return checkDerivedAttributeWildcard(rt, base, derived, attributeWildcardExtension)
}

func validateAttributeUsesRestrict(rt *schemaValidation, baseID, derivedID attributeUseSetID, binding attributeWildcardBinding) error {
	base := rt.build.AttributeUseSets[baseID]
	derived := rt.build.AttributeUseSets[derivedID]
	if err := validateAttributeUseSetRestriction(
		rt.build,
		base,
		derived,
		binding,
	); err != nil {
		return schemaBuildValidationError(err)
	}
	return nil
}

func validateAttributeUseSetRuntime(rt *schemaValidation, _ attributeUseSetID, set attributeUseSet) error {
	if err := validateAttributeUseSetRecord(&rt.build.Names, rt.build, set); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	for _, use := range set.Uses {
		if err := checkNotationTypeUse(rt.build, use.Type); err != nil {
			return schemaBuildValidationError(err)
		}
		if err := validateValueConstraintRuntime(rt, use.Default, use.Type, "attribute use default"); err != nil {
			return err
		}
		if err := validateValueConstraintRuntime(rt, use.Fixed, use.Type, "attribute use fixed"); err != nil {
			return err
		}
	}
	return nil
}

func checkDerivedAttributeWildcard(rt *schemaValidation, base, derived attributeUseSet, expected attributeWildcardDerivation) error {
	if err := validateAttributeWildcardDerivation(rt.build, base, derived, expected); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	return nil
}

func auditElementValueConstraintType(rt *schemaValidation, decl elementDecl) (SimpleTypeID, error) {
	if decl.Default == nil && decl.Fixed == nil {
		return NoSimpleType, nil
	}
	id, err := elementValueConstraintType(rt.build, rt.contentAnalysis, decl.Type)
	if err != nil {
		return NoSimpleType, schemaBuildValidationError(err)
	}
	return id, nil
}

// validateValueConstraintRuntime audits cached value-constraint shape, QName
// context and resolved-name proof structurally; it never re-evaluates the
// lexical value of any element or attribute value constraint, because
// compilation charged and proved that evaluation.
func validateValueConstraintRuntime(rt *schemaValidation, vc *valueConstraint, expected SimpleTypeID, label string) error {
	if vc == nil {
		return nil
	}
	cached := newValueConstraintValidation(vc)
	if err := validateValueConstraintShape(rt.build, cached, expected); err != nil {
		return xsderrors.InternalInvariant(label + " " + err.Error())
	}
	application := valueConstraintApplication(vc.Lexical, vc.Canonical, vc.Value)
	if err := validateValueConstraintQNameContext(application, vc.qnameContext, vc.ResolvedNames); err != nil {
		return xsderrors.InternalInvariant(label + " " + err.Error())
	}
	return nil
}

// ValueConstraintSimpleType returns compiler-owned value-constraint metadata.
func (rt *schemaBuild) ValueConstraintSimpleType(id SimpleTypeID) (valueConstraintSimpleType, bool) {
	if rt.valueBuilder == nil {
		return valueConstraintSimpleType{}, false
	}
	view, ok := rt.valueBuilder.TypeView(id)
	if !ok {
		return valueConstraintSimpleType{}, false
	}
	return valueConstraintSimpleType{
		Union: view.Union, ListItem: view.ListItem,
		Variety: view.Variety, Primitive: view.Primitive,
		HasEnumeration: view.HasEnumeration,
	}, true
}

// ValueConstraintComplexType returns compiler-owned value-constraint metadata.
func (rt *schemaBuild) ValueConstraintComplexType(id ComplexTypeID) (valueConstraintComplexType, bool) {
	ct, ok := complexTypeByID(rt.ComplexTypes, id)
	if !ok {
		return valueConstraintComplexType{}, false
	}
	return newValueConstraintComplexTypeForComplexType(*ct), true
}
