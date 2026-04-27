package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) applyGlobalIndexes() error {
	if b == nil || b.rt == nil {
		return fmt.Errorf("runtime build: runtime schema is nil")
	}
	if b.schema == nil {
		return fmt.Errorf("runtime build: schema ir is nil")
	}
	symbols := b.rt.SymbolCount()
	if err := b.assembler.SetGlobalTypes(make([]runtime.TypeID, symbols+1)); err != nil {
		return err
	}
	if err := b.assembler.SetGlobalElements(make([]runtime.ElemID, symbols+1)); err != nil {
		return err
	}
	if err := b.assembler.SetGlobalAttributes(make([]runtime.AttrID, symbols+1)); err != nil {
		return err
	}

	for _, entry := range b.schema.GlobalIndexes.Types {
		sym, err := b.lookupIRSymbol(entry.Name)
		if err != nil {
			return err
		}
		id, err := b.runtimeTypeIDFromIR(entry)
		if err != nil {
			return err
		}
		if err := b.assembler.SetGlobalType(sym, id); err != nil {
			return err
		}
	}
	for _, entry := range b.schema.GlobalIndexes.Elements {
		sym, err := b.lookupIRSymbol(entry.Name)
		if err != nil {
			return err
		}
		id := runtime.ElemID(entry.Element)
		if id == 0 {
			return fmt.Errorf("runtime build: global element %s missing runtime ID", formatIRName(entry.Name))
		}
		if err := b.assembler.SetGlobalElement(sym, id); err != nil {
			return err
		}
	}
	for _, entry := range b.schema.GlobalIndexes.Attributes {
		sym, err := b.lookupIRSymbol(entry.Name)
		if err != nil {
			return err
		}
		id := runtime.AttrID(entry.Attribute)
		if id == 0 {
			return fmt.Errorf("runtime build: global attribute %s missing runtime ID", formatIRName(entry.Name))
		}
		if err := b.assembler.SetGlobalAttribute(sym, id); err != nil {
			return err
		}
	}
	return nil
}

func (b *schemaBuilder) runtimeTypeIDFromIR(entry schemair.GlobalTypeIndex) (runtime.TypeID, error) {
	if entry.Builtin {
		id := b.builtinRuntimeID(entry.Name.Local)
		if id == 0 {
			return 0, fmt.Errorf("runtime build: builtin type %s missing runtime ID", formatIRName(entry.Name))
		}
		return id, nil
	}
	id := b.userTypeRuntimeID(entry.TypeDecl)
	if id == 0 {
		return 0, fmt.Errorf("runtime build: global type %s missing runtime ID", formatIRName(entry.Name))
	}
	return id, nil
}

func (b *schemaBuilder) lookupIRSymbol(name schemair.Name) (runtime.SymbolID, error) {
	if b == nil || b.rt == nil {
		return 0, fmt.Errorf("runtime build: runtime schema is nil")
	}
	var nsID runtime.NamespaceID
	if name.Namespace == "" {
		nsID = b.rt.KnownNamespaces().Empty
	} else {
		nsID = b.rt.NamespaceLookup([]byte(name.Namespace))
	}
	if nsID == 0 {
		return 0, fmt.Errorf("runtime build: namespace %q missing for %s", name.Namespace, formatIRName(name))
	}
	sym := b.rt.SymbolLookup(nsID, []byte(name.Local))
	if sym == 0 {
		return 0, fmt.Errorf("runtime build: symbol %s missing", formatIRName(name))
	}
	return sym, nil
}

func formatIRName(name schemair.Name) string {
	if name.Namespace == "" {
		return name.Local
	}
	return "{" + name.Namespace + "}" + name.Local
}
