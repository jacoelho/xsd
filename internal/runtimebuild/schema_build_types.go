package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) buildTypes() error {
	if b.schema == nil {
		return fmt.Errorf("runtime build: schema ir is nil")
	}
	nextComplex := uint32(1)
	for i, entry := range b.schema.BuiltinTypes {
		id := runtime.TypeID(i + 1)
		if id == 0 {
			return fmt.Errorf("runtime build: builtin type %s missing ID", entry.Name.Local)
		}
		sym := b.internIRName(entry.Name)
		typ := runtime.Type{Name: sym}
		if !isZeroTypeRef(entry.Base) {
			baseID, ok := b.runtimeTypeIDFromIRRef(entry.Base)
			if !ok {
				return fmt.Errorf("runtime build: builtin type %s base %s not found", entry.Name.Local, formatIRName(entry.Base.Name))
			}
			typ.Base = baseID
			typ.Derivation = runtime.DerRestriction
		}
		if entry.AnyType {
			typ.Kind = runtime.TypeComplex
			typ.Complex = runtime.ComplexTypeRef{ID: nextComplex}
			b.anyTypeComplex = nextComplex
			nextComplex++
		} else {
			typ.Kind = runtime.TypeBuiltin
			typ.Validator = b.artifacts.BuiltinValidators[entry.Name.Local]
		}
		b.rt.Types[id] = typ
		b.rt.GlobalTypes[sym] = id
		if entry.AnyType {
			b.rt.Builtin.AnyType = id
		}
		if entry.AnySimpleType {
			b.rt.Builtin.AnySimpleType = id
		}
	}

	for _, entry := range b.schema.Types {
		id := b.userTypeRuntimeID(entry.ID)
		if id == 0 {
			return fmt.Errorf("runtime build: type %d missing runtime ID", entry.ID)
		}
		var sym runtime.SymbolID
		if !isZeroName(entry.Name) {
			sym = b.internIRName(entry.Name)
		} else if entry.Global {
			return fmt.Errorf("runtime build: global type %d missing name", entry.ID)
		}
		typ := runtime.Type{Name: sym}
		switch entry.Kind {
		case schemair.TypeSimple:
			typ.Kind = runtime.TypeSimple
			if vid, ok := b.artifacts.TypeValidators[entry.ID]; ok {
				typ.Validator = vid
			}
			if !isZeroTypeRef(entry.Base) {
				baseID, ok := b.runtimeTypeIDFromIRRef(entry.Base)
				if !ok {
					return fmt.Errorf("runtime build: type %s base %s not found", formatIRName(entry.Name), formatIRName(entry.Base.Name))
				}
				typ.Base = baseID
				typ.Derivation = toRuntimeIRDerivation(entry.Derivation)
			}
			typ.Final = toRuntimeIRDerivation(entry.Final)
		case schemair.TypeComplex:
			typ.Kind = runtime.TypeComplex
			if entry.Abstract {
				typ.Flags |= runtime.TypeAbstract
			}
			if !isZeroTypeRef(entry.Base) {
				baseID, ok := b.runtimeTypeIDFromIRRef(entry.Base)
				if !ok {
					return fmt.Errorf("runtime build: type %s base %s not found", formatIRName(entry.Name), formatIRName(entry.Base.Name))
				}
				typ.Base = baseID
			}
			typ.Derivation = toRuntimeIRDerivation(entry.Derivation)
			typ.Final = toRuntimeIRDerivation(entry.Final)
			typ.Block = toRuntimeIRDerivation(entry.Block)
			typ.Complex = runtime.ComplexTypeRef{ID: nextComplex}
			b.complexIDs[id] = nextComplex
			nextComplex++
		default:
			return fmt.Errorf("runtime build: unsupported IR type kind %d", entry.Kind)
		}

		b.rt.Types[id] = typ
		if entry.Global {
			b.rt.GlobalTypes[sym] = id
		}
	}
	return nil
}

func (b *schemaBuilder) runtimeTypeIDFromIRRef(ref schemair.TypeRef) (runtime.TypeID, bool) {
	if isZeroTypeRef(ref) {
		return 0, false
	}
	if ref.Builtin {
		id := b.builtinRuntimeID(ref.Name.Local)
		return id, id != 0
	}
	id := b.userTypeRuntimeID(ref.ID)
	return id, id != 0
}

func (b *schemaBuilder) builtinRuntimeID(local string) runtime.TypeID {
	for i, entry := range b.schema.BuiltinTypes {
		if entry.Name.Local == local {
			return runtime.TypeID(i + 1)
		}
	}
	return 0
}

func (b *schemaBuilder) userTypeRuntimeID(id schemair.TypeID) runtime.TypeID {
	if id == 0 {
		return 0
	}
	return runtime.TypeID(len(b.schema.BuiltinTypes)) + runtime.TypeID(id)
}

func toRuntimeIRDerivation(value schemair.Derivation) runtime.DerivationMethod {
	var out runtime.DerivationMethod
	if value&schemair.DerivationExtension != 0 {
		out |= runtime.DerExtension
	}
	if value&schemair.DerivationRestriction != 0 {
		out |= runtime.DerRestriction
	}
	if value&schemair.DerivationList != 0 {
		out |= runtime.DerList
	}
	if value&schemair.DerivationUnion != 0 {
		out |= runtime.DerUnion
	}
	return out
}

func isZeroTypeRef(ref schemair.TypeRef) bool {
	return ref.ID == 0 && !ref.Builtin && isZeroName(ref.Name)
}

func isZeroName(name schemair.Name) bool {
	return name.Namespace == "" && name.Local == ""
}
