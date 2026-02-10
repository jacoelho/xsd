package schemaanalysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// AncestorIndex stores ancestor chains and cumulative derivation masks by TypeID.
type AncestorIndex struct {
	IDs     []TypeID
	Masks   []model.DerivationMethod
	Offsets []uint32
	Lengths []uint32
}

// BuildAncestors computes ancestor chains for all types in registry order.
// Built-in base types terminate a chain and are not included.
func BuildAncestors(schema *parser.Schema, registry *Registry) (*AncestorIndex, error) {
	if registry == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	if err := RequireResolved(schema); err != nil {
		return nil, err
	}
	if err := validateSchemaInput(schema); err != nil {
		return nil, err
	}

	maxID := len(registry.TypeOrder)
	index := &AncestorIndex{
		IDs:     []TypeID{},
		Masks:   []model.DerivationMethod{},
		Offsets: make([]uint32, maxID+1),
		Lengths: make([]uint32, maxID+1),
	}

	for _, entry := range registry.TypeOrder {
		if entry.ID == 0 {
			return nil, fmt.Errorf("type %s has invalid ID", entry.QName)
		}
		offset := uint32(len(index.IDs))
		ids, masks, err := buildAncestorChain(schema, registry, entry.Type)
		if err != nil {
			return nil, err
		}
		index.IDs = append(index.IDs, ids...)
		index.Masks = append(index.Masks, masks...)
		index.Offsets[entry.ID] = offset
		index.Lengths[entry.ID] = uint32(len(ids))
	}

	return index, nil
}

func buildAncestorChain(schema *parser.Schema, registry *Registry, typ model.Type) ([]TypeID, []model.DerivationMethod, error) {
	var ids []TypeID
	var masks []model.DerivationMethod
	cumulative := model.DerivationMethod(0)
	current := typ

	for current != nil {
		baseType, method, err := baseTypeFor(schema, current)
		if err != nil {
			return nil, nil, fmt.Errorf("type %s: %w", typeNameOrKind(current), err)
		}
		if baseType == nil {
			break
		}
		baseQName := baseType.Name()
		if baseQName.Namespace == model.XSDNamespace {
			break
		}
		baseID, ok := lookupAncestorTypeID(registry, baseType)
		if !ok {
			return nil, nil, fmt.Errorf("type %s base %s missing ID", typeNameOrKind(current), typeNameOrKind(baseType))
		}
		cumulative |= method
		ids = append(ids, baseID)
		masks = append(masks, cumulative)
		current = baseType
	}

	return ids, masks, nil
}

func lookupAncestorTypeID(registry *Registry, typ model.Type) (TypeID, bool) {
	if typ == nil || registry == nil {
		return 0, false
	}
	name := typ.Name()
	if !name.IsZero() {
		id, ok := registry.Types[name]
		return id, ok
	}
	return registry.LookupAnonymousTypeID(typ)
}

func typeNameOrKind(typ model.Type) string {
	if typ == nil {
		return "<nil>"
	}
	name := typ.Name()
	if !name.IsZero() {
		return name.String()
	}
	return fmt.Sprintf("%T", typ)
}
