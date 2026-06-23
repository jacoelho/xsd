package runtime

import (
	goruntime "runtime"
	"sync/atomic"

	"github.com/jacoelho/xsd/xsderrors"
)

const (
	schemaPrepareIdle uint32 = iota
	schemaPrepareRunning
	schemaPrepareReady
)

// ReadProjectionsPublished reports whether any validation read projection has
// been built. A partially published runtime is invalid and must be rejected by
// ValidateSchema rather than silently rebuilt.
func (rt *Schema) ReadProjectionsPublished() bool {
	return rt != nil && rt.readProjectionsPublished
}

// EnsurePublished builds validation read projections once for a moved schema.
func (rt *Schema) EnsurePublished() error {
	if rt == nil {
		return xsderrors.InternalInvariant("nil schema runtime")
	}
	if rt.ReadProjectionsPublished() {
		return nil
	}
	rt.publishReadProjections(false)
	return ValidateSchema(rt)
}

func (rt *Schema) publishReadProjections(hotPaths bool) {
	globalReads := NewGlobalReadMapProjection(rt.GlobalAttributes, rt.GlobalElements, rt.GlobalTypes)
	rt.GlobalAttributeReads = globalReads.Attributes
	rt.GlobalElementReads = globalReads.Elements
	rt.GlobalTypeReads = globalReads.Types
	rt.SubstitutionReads = rt.Substitutions
	rt.SubstitutionLookupReads = rt.SubstitutionLookup
	rt.NameReads = NewBorrowedNameReadView(&rt.Names)
	rt.NotationReads = NewNotationReadMap(&rt.Names, rt.Notations)
	rt.publishSimpleValueReadProjections(hotPaths)
	rt.AttributeDeclReads = NewAttributeDeclReadsForDecls(rt.Attributes)
	if hotPaths {
		rt.AttributeUseSetReads = NewAttributeUseSetReadsForSetsWithTypeReads(&rt.Names, rt.AttributeUseSets, rt.SimpleValueTypeReads)
	} else {
		rt.AttributeUseSetReads = NewAttributeUseSetReadsForSetsWithSimpleTypes(&rt.Names, rt.AttributeUseSets, rt.SimpleTypes)
	}
	rt.TypeDerivations = NewTypeDerivationReadForTypes(rt.Builtin.AnyType, rt.SimpleTypes, rt.ComplexTypes)
	rt.SimpleTypePrimitives = NewSimpleTypePrimitiveReadsForTypes(rt.SimpleTypes)
	rt.SimpleTypeIdentities = NewSimpleTypeIdentityReadsForTypes(rt.SimpleTypes)
	rt.SimpleTypeFinals = NewSimpleTypeFinalReadsForTypes(rt.SimpleTypes)
	rt.ElementNames = NewElementNameReadsForDecls(rt.Elements)
	rt.ElementStartInfos = NewElementStartInfosForElementDecls(rt.Elements)
	rt.ElementIdentityConstraintReads = NewElementIdentityConstraintReadsForDecls(rt.Elements)
	rt.IdentityConstraintReads = NewIdentityConstraintReads(rt.Identities)
	rt.ComplexTypeInfos = NewTypeInfosForComplexTypes(rt.ComplexTypes)
	rt.ComplexAttributeUseSetIDs = NewComplexAttributeUseSetIDProjection(rt.ComplexTypes)
	rt.ComplexContentModelIDs = NewComplexContentModelIDProjection(rt.ComplexTypes)
	rt.ComplexSimpleContentReads = NewSimpleContentTypeReadsForComplexTypes(rt.ComplexTypes)
	rt.ComplexChildContentReads = NewElementChildContentsForComplexTypes(rt.ComplexTypes)
	rt.ComplexTextContentReads = NewElementTextContentsForComplexTypes(rt.ComplexTypes, false)
	rt.FixedComplexTextContentReads = NewElementTextContentsForComplexTypes(rt.ComplexTypes, true)
	rt.WildcardReads = NewWildcardViews(&rt.Names, rt.Wildcards)
	rt.CompiledModelViews = NewBorrowedCompiledModelViews(rt.CompiledModels)
	rt.ElementValueConstraintReads = NewElementValueConstraintReadsForDecls(rt.Elements)
	rt.SimpleTextContentRead = NewElementTextContentForSimpleType()
	rt.readProjectionsPublished = true
	rt.validationHotPathsPrepared = false
}

func (rt *Schema) publishSimpleValueReadProjections(hotPaths bool) {
	rt.SimpleValueFacetReads = SimpleValueFacetReadTable{}
	if hotPaths {
		rt.SimpleValueTypeReads = NewSimpleValueTypeReadsForSimpleTypes(rt.SimpleTypes)
		rt.SimpleValueQNameResolverNeeds = NewSimpleValueQNameResolverNeedsForTypeReads(rt.SimpleValueTypeReads)
		rt.simpleValueCallbacks = NewSimpleValueCallbacksForTypeReadsAndSimpleTypes(
			rt.SimpleValueTypeReads,
			rt.SimpleTypes,
			notationReadLookup(rt.NotationReads),
			nil,
			nil,
		)
		rt.rawSimpleValueCallbacks = NewRawSimpleValueCallbacksForTypeReads(rt.SimpleValueTypeReads)
		return
	}
	rt.SimpleValueTypeReads = nil
	rt.SimpleValueQNameResolverNeeds = NewSimpleValueQNameResolverNeedsForSimpleTypes(rt.SimpleTypes)
	rt.simpleValueCallbacks = NewSimpleValueCallbacksForSimpleTypes(
		rt.SimpleTypes,
		notationReadLookup(rt.NotationReads),
		nil,
		nil,
	)
	rt.rawSimpleValueCallbacks = NewRawSimpleValueCallbacksForSimpleTypes(rt.SimpleTypes)
}

// PrepareValidationHotPaths builds validation read projections and hot tables
// outside the compile publication path.
func (rt *Schema) PrepareValidationHotPaths() error {
	if rt == nil {
		return xsderrors.InternalInvariant("nil schema runtime")
	}
	for {
		switch atomic.LoadUint32(&rt.prepareState) {
		case schemaPrepareReady:
			return nil
		case schemaPrepareIdle:
			if atomic.CompareAndSwapUint32(&rt.prepareState, schemaPrepareIdle, schemaPrepareRunning) {
				err := rt.prepareValidationHotPaths()
				if err != nil {
					atomic.StoreUint32(&rt.prepareState, schemaPrepareIdle)
					return err
				}
				atomic.StoreUint32(&rt.prepareState, schemaPrepareReady)
				return nil
			}
		default:
			goruntime.Gosched()
		}
	}
}

func (rt *Schema) prepareValidationHotPaths() error {
	if rt.validationHotPathsPrepared {
		return nil
	}
	if !rt.ReadProjectionsPublished() {
		rt.publishReadProjections(true)
	} else if len(rt.SimpleValueTypeReads) == 0 {
		rt.publishSimpleValueReadProjections(true)
	}
	if err := ValidateSchemaPublication(rt); err != nil {
		return err
	}
	rt.validationHotPathsPrepared = true
	return nil
}
