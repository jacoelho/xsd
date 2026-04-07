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

// ValidateDefaultOrFixedResolved validates default/fixed lexical values
// against resolved schema-time type
func ValidateDefaultOrFixedResolved(
	schema *parser.Schema,
	value string,
	typ model.Type,
	context map[string]string,
	policy IDPolicy,
) error {
	settings := validationSettings{
		mode:                valueValidationDefaultFixed,
		idPolicy:            policy,
		errorOnPlaceholder:  true,
		requireQNameContext: false,
	}
	return validateValue(schema, value, typ, context, make(map[model.Type]bool), settings)
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
	settings := validationSettings{
		mode:                valueValidationFacet,
		idPolicy:            IDPolicyAllow,
		errorOnPlaceholder:  false,
		requireQNameContext: true,
		convert:             convert,
	}
	return validateValue(schema, value, typ, context, make(map[model.Type]bool), settings)
}

type validationSettings struct {
	convert             model.DeferredFacetConverter
	mode                valueValidationMode
	idPolicy            IDPolicy
	errorOnPlaceholder  bool
	requireQNameContext bool
}

func validateValue(
	schema *parser.Schema,
	value string,
	typ model.Type,
	context map[string]string,
	visited map[model.Type]bool,
	settings validationSettings,
) error {
	if typ == nil {
		return nil
	}
	if err := validateTypePolicies(schema, typ, settings.errorOnPlaceholder, settings.idPolicy); err != nil {
		return err
	}
	if ct, ok := typ.(*model.ComplexType); ok {
		if visited[typ] {
			return ErrCircularReference
		}
		visited[typ] = true
		defer delete(visited, typ)
		return validateComplexType(schema, value, ct, context, visited, settings)
	}
	if typ.IsBuiltin() {
		return validateBuiltinValue(typ, value, context, settings.requireQNameContext)
	}
	st, ok := typ.(*model.SimpleType)
	if !ok {
		return nil
	}
	return validateSimpleTypeValue(schema, value, st, context, settings)
}

func validateBuiltinValue(typ model.Type, lexical string, context map[string]string, requireQNameContext bool) error {
	normalized := model.NormalizeWhiteSpace(lexical, typ)
	if model.IsQNameOrNotationType(typ) {
		if requireQNameContext && context == nil {
			return fmt.Errorf("namespace context unavailable for QName/NOTATION value")
		}
		if err := ValidateQNameContext(normalized, context); err != nil {
			return err
		}
	}
	return validateBuiltin(typ, normalized)
}

func validateSimpleTypeValue(
	schema *parser.Schema,
	lexical string,
	st *model.SimpleType,
	context map[string]string,
	settings validationSettings,
) error {
	opts := buildSimpleTypeValidationOptions(schema, settings)
	return model.ValidateSimpleTypeWithOptions(st, lexical, context, opts)
}

func buildSimpleTypeValidationOptions(schema *parser.Schema, settings validationSettings) model.SimpleTypeValidationOptions {
	validateTypeWithPolicy := func(current model.Type, scope model.SimpleTypeValidationScope) error {
		idPolicy := settings.idPolicy
		if settings.mode == valueValidationDefaultFixed && scope == model.SimpleTypeValidationScopeUnionMember {
			idPolicy = IDPolicyAllow
		}
		return validateTypePolicies(schema, current, settings.errorOnPlaceholder, idPolicy)
	}

	opts := model.SimpleTypeValidationOptions{
		ResolveListItem: func(current *model.SimpleType) model.Type {
			return parser.ResolveListItemType(schema, current)
		},
		ResolveUnionMembers: func(current *model.SimpleType) []model.Type {
			return parser.ResolveUnionMemberTypes(schema, current)
		},
		ResolveFacetType: func(name model.QName) model.Type {
			return parser.ResolveSimpleTypeReferenceAllowMissing(schema, name)
		},
		ConvertDeferredFacets: settings.convert,
		RequireQNameContext:   settings.requireQNameContext,
		CycleError:            ErrCircularReference,
		ValidateType:          validateTypeWithPolicy,
		ValidateFacets: func(normalized string, current *model.SimpleType, ctx map[string]string) error {
			return ValidateSimpleTypeFacets(schema, current, normalized, ctx, settings.convert)
		},
	}

	configureSimpleTypeMode(&opts, settings.mode)
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

func validateComplexType(
	schema *parser.Schema,
	value string,
	ct *model.ComplexType,
	context map[string]string,
	visited map[model.Type]bool,
	settings validationSettings,
) error {
	sc, ok := ct.Content().(*model.SimpleContent)
	if !ok {
		return nil
	}
	baseType := parser.ResolveSimpleContentBaseTypeFromContent(schema, sc)
	if baseType == nil {
		return nil
	}

	if settings.mode == valueValidationDefaultFixed {
		if sc.Restriction != nil {
			if err := validateValue(schema, value, baseType, context, visited, settings); err != nil {
				return err
			}
			return ValidateRestrictionFacets(schema, sc.Restriction, baseType, value, context, settings.convert)
		}
		return validateValue(schema, value, baseType, context, visited, settings)
	}

	if sc.Restriction != nil {
		if err := ValidateRestrictionFacets(schema, sc.Restriction, baseType, value, context, settings.convert); err != nil {
			return err
		}
	}
	return validateValue(schema, value, baseType, context, visited, settings)
}

func validateBuiltin(typ model.Type, normalizedValue string) error {
	bt := model.GetBuiltinNS(typ.Name().Namespace, typ.Name().Local)
	if bt == nil {
		return nil
	}
	return bt.Validate(normalizedValue)
}

func validateTypePolicies(schema *parser.Schema, typ model.Type, errorOnPlaceholder bool, policy IDPolicy) error {
	if typ == nil {
		return nil
	}
	if errorOnPlaceholder {
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
	if parser.IsIDOnlyDerivedType(schema, st) {
		return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", st.Name().Local)
	}
	return nil
}
