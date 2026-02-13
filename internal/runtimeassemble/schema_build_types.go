package runtimeassemble

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/runtimeids"
	"github.com/jacoelho/xsd/internal/types"
)

func (b *schemaBuilder) buildTypes() error {
	xsdNS := types.XSDNamespace
	nextComplex := uint32(1)
	for _, name := range runtimeids.BuiltinTypeNames() {
		id := b.builtinIDs[name]
		sym := b.internQName(types.QName{Namespace: xsdNS, Local: string(name)})
		typ := runtime.Type{Name: sym}
		if builtin := builtins.Get(name); builtin != nil {
			base := builtin.BaseType()
			if base != nil {
				baseID, ok := b.runtimeTypeID(base)
				if !ok {
					return fmt.Errorf("runtime build: builtin type %s base %s not found", name, base.Name())
				}
				typ.Base = baseID
				typ.Derivation = runtime.DerRestriction
			}
		}
		if name == types.TypeNameAnyType {
			typ.Kind = runtime.TypeComplex
			typ.Complex = runtime.ComplexTypeRef{ID: nextComplex}
			b.anyTypeComplex = nextComplex
			nextComplex++
		} else {
			typ.Kind = runtime.TypeBuiltin
			typ.Validator = b.validatorForBuiltin(name)
		}
		b.rt.Types[id] = typ
		b.rt.GlobalTypes[sym] = id
		if name == types.TypeNameAnyType {
			b.rt.Builtin.AnyType = id
		}
		if name == types.TypeNameAnySimpleType {
			b.rt.Builtin.AnySimpleType = id
		}
	}

	for _, entry := range b.registry.TypeOrder {
		id := b.typeIDs[entry.ID]
		var sym runtime.SymbolID
		if !entry.QName.IsZero() {
			sym = b.internQName(entry.QName)
		} else if entry.Global {
			return fmt.Errorf("runtime build: global type %d missing name", entry.ID)
		}
		typ := runtime.Type{Name: sym}
		switch t := entry.Type.(type) {
		case *types.SimpleType:
			typ.Kind = runtime.TypeSimple
			if vid, ok := b.validators.TypeValidators[entry.ID]; ok {
				typ.Validator = vid
			} else if vid, ok := b.validators.ValidatorForType(t); ok {
				typ.Validator = vid
			}
			base, method := b.baseForSimpleType(t)
			if base != nil {
				baseID, ok := b.runtimeTypeID(base)
				if !ok {
					return fmt.Errorf("runtime build: type %s base %s not found", entry.QName, base.Name())
				}
				typ.Base = baseID
				typ.Derivation = method
			}
			typ.Final = toRuntimeDerivationSet(t.Final)
		case *types.ComplexType:
			typ.Kind = runtime.TypeComplex
			if t.Abstract {
				typ.Flags |= runtime.TypeAbstract
			}
			base := t.BaseType()
			if base != nil {
				baseID, ok := b.runtimeTypeID(base)
				if !ok {
					return fmt.Errorf("runtime build: type %s base %s not found", entry.QName, base.Name())
				}
				typ.Base = baseID
			}
			method := t.DerivationMethod
			if method == 0 {
				method = types.DerivationRestriction
			}
			typ.Derivation = toRuntimeDerivation(method)
			typ.Final = toRuntimeDerivationSet(t.Final)
			typ.Block = toRuntimeDerivationSet(t.Block)
			typ.Complex = runtime.ComplexTypeRef{ID: nextComplex}
			b.complexIDs[id] = nextComplex
			nextComplex++
		default:
			return fmt.Errorf("runtime build: unsupported type %T", entry.Type)
		}

		b.rt.Types[id] = typ
		if entry.Global {
			b.rt.GlobalTypes[sym] = id
		}
	}
	return nil
}
