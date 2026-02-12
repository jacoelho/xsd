package analysis

import (
	"fmt"

	model "github.com/jacoelho/xsd/internal/types"
)

func (b *builder) assignGlobalType(name model.QName, typ model.Type) error {
	if typ == nil {
		return fmt.Errorf("global type %s is nil", name)
	}
	if _, exists := b.registry.Types[name]; exists {
		return fmt.Errorf("duplicate global type %s", name)
	}
	id := b.nextType
	b.nextType++
	b.registry.Types[name] = id
	b.typeIDs[typ] = id
	b.registry.TypeOrder = append(b.registry.TypeOrder, TypeEntry{
		ID:     id,
		QName:  name,
		Type:   typ,
		Global: true,
	})
	return nil
}

func (b *builder) assignAnonymousType(typ model.Type) error {
	if typ == nil {
		return nil
	}
	if !typ.Name().IsZero() {
		return fmt.Errorf("expected anonymous type, got %s", typ.Name())
	}
	if existing, ok := b.typeIDs[typ]; ok {
		if _, seen := b.registry.anonymousTypes[typ]; !seen {
			b.registry.anonymousTypes[typ] = existing
		}
		return nil
	}
	id := b.nextType
	b.nextType++
	b.typeIDs[typ] = id
	b.registry.anonymousTypes[typ] = id
	b.registry.TypeOrder = append(b.registry.TypeOrder, TypeEntry{
		ID:     id,
		QName:  model.QName{},
		Type:   typ,
		Global: false,
	})
	return nil
}

func (b *builder) assignGlobalElement(decl *model.ElementDecl) error {
	if decl == nil {
		return fmt.Errorf("global element is nil")
	}
	if _, exists := b.registry.Elements[decl.Name]; exists {
		return fmt.Errorf("duplicate global element %s", decl.Name)
	}
	id := b.nextElem
	b.nextElem++
	b.registry.Elements[decl.Name] = id
	b.registry.ElementOrder = append(b.registry.ElementOrder, ElementEntry{
		ID:     id,
		QName:  decl.Name,
		Decl:   decl,
		Global: true,
	})
	return nil
}

func (b *builder) assignLocalElement(decl *model.ElementDecl) error {
	if decl == nil {
		return fmt.Errorf("local element is nil")
	}
	if _, exists := b.registry.localElements[decl]; exists {
		return nil
	}
	id := b.nextElem
	b.nextElem++
	b.registry.localElements[decl] = id
	b.registry.ElementOrder = append(b.registry.ElementOrder, ElementEntry{
		ID:     id,
		QName:  decl.Name,
		Decl:   decl,
		Global: false,
	})
	return nil
}

func (b *builder) assignGlobalAttribute(name model.QName, decl *model.AttributeDecl) error {
	if decl == nil {
		return fmt.Errorf("global attribute %s is nil", name)
	}
	if _, exists := b.registry.Attributes[name]; exists {
		return fmt.Errorf("duplicate global attribute %s", name)
	}
	id := b.nextAttr
	b.nextAttr++
	b.registry.Attributes[name] = id
	b.registry.AttributeOrder = append(b.registry.AttributeOrder, AttributeEntry{
		ID:     id,
		QName:  name,
		Decl:   decl,
		Global: true,
	})
	return nil
}

func (b *builder) assignLocalAttribute(decl *model.AttributeDecl) error {
	if decl == nil {
		return fmt.Errorf("local attribute is nil")
	}
	if _, exists := b.registry.localAttributes[decl]; exists {
		return nil
	}
	id := b.nextAttr
	b.nextAttr++
	b.registry.localAttributes[decl] = id
	b.registry.AttributeOrder = append(b.registry.AttributeOrder, AttributeEntry{
		ID:     id,
		QName:  decl.Name,
		Decl:   decl,
		Global: false,
	})
	return nil
}
