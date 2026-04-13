package semantics

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// ErrCircularReference reports a cycle while validating derived type model.
var ErrCircularReference = errors.New("circular type reference")

// IDPolicy controls whether ID-only types are accepted.
type IDPolicy uint8

const (
	// IDPolicyAllow allows validating ID-only model.
	IDPolicyAllow IDPolicy = iota
	// IDPolicyDisallow rejects ID-only types for default/fixed values.
	IDPolicyDisallow
)

type valueValidationMode uint8

const (
	valueValidationDefaultFixed valueValidationMode = iota
	valueValidationFacet
)

type schemaValueValidator struct {
	schema              *parser.Schema
	convert             model.DeferredFacetConverter
	mode                valueValidationMode
	idPolicy            IDPolicy
	errorOnPlaceholder  bool
	requireQNameContext bool
}

// ValidateDefaultOrFixedResolved validates default/fixed lexical values
// against resolved schema-time type
func ValidateDefaultOrFixedResolved(
	schema *parser.Schema,
	value string,
	typ model.Type,
	context map[string]string,
	policy IDPolicy,
) error {
	validator := schemaValueValidator{
		schema:              schema,
		mode:                valueValidationDefaultFixed,
		idPolicy:            policy,
		errorOnPlaceholder:  true,
		requireQNameContext: false,
	}
	return validator.validateValue(value, typ, context, make(map[model.Type]bool))
}

// ValidateWithFacets validates lexical values with schema-time facet
// conversion.
func ValidateWithFacets(
	schema *parser.Schema,
	value string,
	typ model.Type,
	context map[string]string,
	convert model.DeferredFacetConverter,
) error {
	validator := schemaValueValidator{
		schema:              schema,
		convert:             convert,
		mode:                valueValidationFacet,
		idPolicy:            IDPolicyAllow,
		errorOnPlaceholder:  false,
		requireQNameContext: true,
	}
	return validator.validateValue(value, typ, context, make(map[model.Type]bool))
}

func (v schemaValueValidator) validateValue(
	value string,
	typ model.Type,
	context map[string]string,
	visited map[model.Type]bool,
) error {
	if typ == nil {
		return nil
	}
	if err := v.validateTypePolicies(typ, v.idPolicy); err != nil {
		return err
	}
	if ct, ok := typ.(*model.ComplexType); ok {
		if visited[typ] {
			return ErrCircularReference
		}
		visited[typ] = true
		defer delete(visited, typ)
		return v.validateComplexType(value, ct, context, visited)
	}
	if typ.IsBuiltin() {
		return v.validateBuiltinValue(typ, value, context)
	}
	st, ok := typ.(*model.SimpleType)
	if !ok {
		return nil
	}
	return v.validateSimpleTypeValue(value, st, context)
}

func (v schemaValueValidator) validateBuiltinValue(typ model.Type, lexical string, context map[string]string) error {
	normalized := model.NormalizeWhiteSpace(lexical, typ)
	if model.IsQNameOrNotationType(typ) {
		if v.requireQNameContext && context == nil {
			return fmt.Errorf("namespace context unavailable for QName/NOTATION value")
		}
		if err := ValidateQNameContext(normalized, context); err != nil {
			return err
		}
	}
	return validateBuiltin(typ, normalized)
}

func (v schemaValueValidator) validateSimpleTypeValue(
	lexical string,
	st *model.SimpleType,
	context map[string]string,
) error {
	opts := v.buildSimpleTypeValidationOptions()
	return model.ValidateSimpleTypeWithOptions(st, lexical, context, opts)
}

func (v schemaValueValidator) buildSimpleTypeValidationOptions() model.SimpleTypeValidationOptions {
	validateTypeWithPolicy := func(current model.Type, scope model.SimpleTypeValidationScope) error {
		idPolicy := v.idPolicy
		if v.mode == valueValidationDefaultFixed && scope == model.SimpleTypeValidationScopeUnionMember {
			idPolicy = IDPolicyAllow
		}
		return v.validateTypePolicies(current, idPolicy)
	}

	opts := model.SimpleTypeValidationOptions{
		ResolveListItem: func(current *model.SimpleType) model.Type {
			return parser.ResolveListItemType(v.schema, current)
		},
		ResolveUnionMembers: func(current *model.SimpleType) []model.Type {
			return parser.ResolveUnionMemberTypes(v.schema, current)
		},
		ResolveFacetType: func(name model.QName) model.Type {
			return parser.ResolveSimpleTypeReferenceAllowMissing(v.schema, name)
		},
		ConvertDeferredFacets: v.convert,
		RequireQNameContext:   v.requireQNameContext,
		CycleError:            ErrCircularReference,
		ValidateType:          validateTypeWithPolicy,
		ValidateFacets: func(normalized string, current *model.SimpleType, ctx map[string]string) error {
			return ValidateSimpleTypeFacets(v.schema, current, normalized, ctx, v.convert)
		},
	}

	configureSimpleTypeMode(&opts, v.mode)
	return opts
}

func configureSimpleTypeMode(opts *model.SimpleTypeValidationOptions, mode valueValidationMode) {
	switch mode {
	case valueValidationDefaultFixed:
		opts.UnionNoMatch = func(current *model.SimpleType, normalized string, firstErr error, sawCycle bool) error {
			if firstErr != nil {
				return firstErr
			}
			if sawCycle {
				return fmt.Errorf("cannot validate default/fixed value for circular union type '%s'", current.Name().Local)
			}
			return fmt.Errorf("value '%s' does not match any member type of union '%s'", normalized, current.Name().Local)
		}
		opts.ListItemErr = func(current *model.SimpleType, err error, isCycle bool) error {
			if isCycle {
				return fmt.Errorf("cannot validate default/fixed value for circular list item type '%s'", current.Name().Local)
			}
			return err
		}
	default:
		opts.UnionNoMatch = func(_ *model.SimpleType, normalized string, firstErr error, _ bool) error {
			if firstErr != nil {
				return firstErr
			}
			return fmt.Errorf("value %q does not match any member type of union", normalized)
		}
	}
}

func (v schemaValueValidator) validateComplexType(
	value string,
	ct *model.ComplexType,
	context map[string]string,
	visited map[model.Type]bool,
) error {
	sc, ok := ct.Content().(*model.SimpleContent)
	if !ok {
		return nil
	}
	baseType := parser.ResolveSimpleContentBaseTypeFromContent(v.schema, sc)
	if baseType == nil {
		return nil
	}

	if v.mode == valueValidationDefaultFixed {
		if sc.Restriction != nil {
			if err := v.validateValue(value, baseType, context, visited); err != nil {
				return err
			}
			return ValidateRestrictionFacets(v.schema, sc.Restriction, baseType, value, context, v.convert)
		}
		return v.validateValue(value, baseType, context, visited)
	}

	if sc.Restriction != nil {
		if err := ValidateRestrictionFacets(v.schema, sc.Restriction, baseType, value, context, v.convert); err != nil {
			return err
		}
	}
	return v.validateValue(value, baseType, context, visited)
}

func validateBuiltin(typ model.Type, normalizedValue string) error {
	bt := model.GetBuiltinNS(typ.Name().Namespace, typ.Name().Local)
	if bt == nil {
		return nil
	}
	return bt.Validate(normalizedValue)
}

func (v schemaValueValidator) validateTypePolicies(typ model.Type, policy IDPolicy) error {
	if typ == nil {
		return nil
	}
	if v.errorOnPlaceholder {
		if st, ok := typ.(*model.SimpleType); ok && model.IsPlaceholderSimpleType(st) {
			return fmt.Errorf("type %s not resolved", st.QName)
		}
	}
	if policy != IDPolicyDisallow {
		return nil
	}
	if typ.IsBuiltin() {
		if parser.IsIDOnlyType(typ.Name()) {
			return fmt.Errorf("type '%s' cannot have default or fixed values", typ.Name().Local)
		}
		return nil
	}
	st, ok := typ.(*model.SimpleType)
	if !ok {
		return nil
	}
	if parser.IsIDOnlyDerivedType(v.schema, st) {
		return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", st.Name().Local)
	}
	return nil
}
