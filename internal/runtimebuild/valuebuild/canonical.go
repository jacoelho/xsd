package valuebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/num"
)

type canonicalizeMode uint8

const (
	canonicalizeGeneral canonicalizeMode = iota
	canonicalizeDefault
)

func (c *artifactCompiler) normalizeLexical(lexical string, spec schemair.SimpleTypeSpec) string {
	normalized := value.NormalizeWhitespace(valueWhitespace(runtimeWhitespace(spec.Whitespace)), []byte(lexical), nil)
	return string(normalized)
}

func valueWhitespace(mode runtime.WhitespaceMode) value.WhitespaceMode {
	switch mode {
	case runtime.WSReplace:
		return value.WhitespaceReplace
	case runtime.WSCollapse:
		return value.WhitespaceCollapse
	default:
		return value.WhitespacePreserve
	}
}

func (c *artifactCompiler) canonicalizeNormalized(lexical, normalized string, spec schemair.SimpleTypeSpec, ctx map[string]string, mode canonicalizeMode) ([]byte, error) {
	switch spec.Variety {
	case schemair.TypeVarietyList:
		item, ok := c.specForRef(spec.Item)
		if !ok {
			return nil, fmt.Errorf("list type missing item type")
		}
		var buf []byte
		count := 0
		for itemLex := range value.FieldsXMLWhitespaceStringSeq(normalized) {
			itemNorm := c.normalizeLexical(itemLex, item)
			canon, err := c.canonicalizeNormalized(itemLex, itemNorm, item, ctx, mode)
			if err != nil {
				return nil, err
			}
			if count > 0 {
				buf = append(buf, ' ')
			}
			buf = append(buf, canon...)
			count++
		}
		if count == 0 {
			return []byte{}, nil
		}
		return buf, nil
	case schemair.TypeVarietyUnion:
		return c.canonicalizeUnion(lexical, spec, ctx, mode)
	default:
		return canonicalizeAtomic(normalized, spec, ctx)
	}
}

func (c *artifactCompiler) canonicalizeUnion(lexical string, spec schemair.SimpleTypeSpec, ctx map[string]string, mode canonicalizeMode) ([]byte, error) {
	if len(spec.Members) == 0 {
		return nil, fmt.Errorf("union has no member types")
	}
	for _, ref := range spec.Members {
		member, ok := c.specForRef(ref)
		if !ok {
			continue
		}
		memberLex := c.normalizeLexical(lexical, member)
		switch mode {
		case canonicalizeDefault:
			if err := c.validatePartialFacets(memberLex, member, member.Facets, ctx); err != nil {
				continue
			}
			canon, err := c.canonicalizeNormalized(lexical, memberLex, member, ctx, mode)
			if err != nil {
				continue
			}
			if err := c.validateEnumSets(lexical, memberLex, member, ctx); err != nil {
				continue
			}
			return canon, nil
		default:
			if err := c.validateMemberFacets(memberLex, member, member.Facets, ctx, true); err != nil {
				continue
			}
			canon, err := c.canonicalizeNormalized(lexical, memberLex, member, ctx, mode)
			if err == nil {
				return canon, nil
			}
		}
	}
	return nil, fmt.Errorf("union value does not match any member type")
}

func canonicalizeAtomic(normalized string, spec schemair.SimpleTypeSpec, ctx map[string]string) ([]byte, error) {
	if spec.QNameOrNotation {
		return value.CanonicalQName([]byte(normalized), mapResolver(ctx), nil)
	}

	primitive := primitiveForSpec(spec)
	if kind, ok := value.KindFromPrimitiveName(primitive); ok {
		tv, err := value.Parse(kind, []byte(normalized))
		if err != nil {
			return nil, err
		}
		return []byte(value.Canonical(tv)), nil
	}

	switch primitive {
	case "anySimpleType":
		return []byte(normalized), nil
	case "string":
		return value.CanonicalizeString([]byte(normalized), func(data []byte) error {
			return runtime.ValidateStringKind(stringKindForBuiltin(specBuiltinName(spec)), data)
		})
	case "anyURI":
		return value.CanonicalizeAnyURI([]byte(normalized))
	case "decimal":
		if spec.IntegerDerived {
			_, canon, err := value.CanonicalizeInteger([]byte(normalized), func(v num.Int) error {
				return runtime.ValidateIntegerKind(integerKindForBuiltin(specBuiltinName(spec)), v)
			})
			if err != nil {
				return nil, err
			}
			return canon, nil
		}
		_, canon, err := value.CanonicalizeDecimal([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "boolean":
		_, canon, err := value.CanonicalizeBoolean([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "float":
		_, _, canon, err := value.CanonicalizeFloat32([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "double":
		_, _, canon, err := value.CanonicalizeFloat64([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "duration":
		_, canon, err := value.CanonicalizeDuration([]byte(normalized))
		if err != nil {
			return nil, err
		}
		return canon, nil
	case "hexBinary":
		return value.CanonicalizeHexBinary([]byte(normalized))
	case "base64Binary":
		return value.CanonicalizeBase64Binary([]byte(normalized))
	default:
		return nil, fmt.Errorf("unsupported primitive type %s", primitive)
	}
}
