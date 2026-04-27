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
	nextComplex, err := b.buildBuiltinTypes(1)
	if err != nil {
		return err
	}
	return b.buildUserTypes(nextComplex)
}

func (b *schemaBuilder) buildBuiltinTypes(startComplexID uint32) (uint32, error) {
	nextComplex := startComplexID
	for i, entry := range b.schema.BuiltinTypes {
		id := runtime.TypeID(i + 1)
		if id == 0 {
			return 0, fmt.Errorf("runtime build: builtin type %s missing ID", entry.Name.Local)
		}
		sym := b.internIRName(entry.Name)
		typ := runtime.Type{Name: sym}
		if !entry.Base.IsZero() {
			baseID, ok := b.runtimeTypeIDFromIRRef(entry.Base)
			if !ok {
				return 0, fmt.Errorf("runtime build: builtin type %s base %s not found", entry.Name.Local, formatIRName(entry.Base.TypeName()))
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
		if err := b.assembler.SetType(id, typ); err != nil {
			return 0, err
		}
		if err := b.assembler.SetGlobalType(sym, id); err != nil {
			return 0, err
		}
		if entry.AnyType {
			builtin := b.rt.BuiltinTypes()
			builtin.AnyType = id
			if err := b.assembler.SetBuiltin(builtin); err != nil {
				return 0, err
			}
		}
		if entry.AnySimpleType {
			builtin := b.rt.BuiltinTypes()
			builtin.AnySimpleType = id
			if err := b.assembler.SetBuiltin(builtin); err != nil {
				return 0, err
			}
		}
	}
	return nextComplex, nil
}

func (b *schemaBuilder) buildUserTypes(nextComplex uint32) error {
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
			if !entry.Base.IsZero() {
				baseID, ok := b.runtimeTypeIDFromIRRef(entry.Base)
				if !ok {
					return fmt.Errorf("runtime build: type %s base %s not found", formatIRName(entry.Name), formatIRName(entry.Base.TypeName()))
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
			if !entry.Base.IsZero() {
				baseID, ok := b.runtimeTypeIDFromIRRef(entry.Base)
				if !ok {
					return fmt.Errorf("runtime build: type %s base %s not found", formatIRName(entry.Name), formatIRName(entry.Base.TypeName()))
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

		if err := b.assembler.SetType(id, typ); err != nil {
			return err
		}
		if entry.Global {
			if err := b.assembler.SetGlobalType(sym, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *schemaBuilder) runtimeTypeIDFromIRRef(ref schemair.TypeRef) (runtime.TypeID, bool) {
	if ref.IsZero() {
		return 0, false
	}
	if ref.IsBuiltin() {
		id := b.builtinRuntimeID(ref.TypeName().Local)
		return id, id != 0
	}
	id := b.userTypeRuntimeID(ref.TypeID())
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

func isZeroName(name schemair.Name) bool {
	return name.Namespace == "" && name.Local == ""
}
