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
		baseQName, method := baseForType(current)
		if baseQName.IsZero() {
			break
		}
		if baseQName.Namespace == model.XSDNamespace {
			break
		}
		baseType := schema.TypeDefs[baseQName]
		if baseType == nil {
			return nil, nil, fmt.Errorf("type %s base %s not found", current.Name(), baseQName)
		}
		baseID, ok := registry.Types[baseQName]
		if !ok {
			return nil, nil, fmt.Errorf("type %s base %s missing ID", current.Name(), baseQName)
		}
		cumulative |= method
		ids = append(ids, baseID)
		masks = append(masks, cumulative)
		current = baseType
	}

	return ids, masks, nil
}

func baseForType(typ model.Type) (model.QName, model.DerivationMethod) {
	switch typed := typ.(type) {
	case *model.SimpleType:
		return baseForSimpleType(typed)
	case *model.ComplexType:
		return baseForComplexType(typed)
	default:
		return model.QName{}, 0
	}
}

func baseForSimpleType(st *model.SimpleType) (model.QName, model.DerivationMethod) {
	if st == nil {
		return model.QName{}, 0
	}
	if st.List != nil {
		return model.AnySimpleTypeQName(), model.DerivationList
	}
	if st.Union != nil {
		return model.AnySimpleTypeQName(), model.DerivationUnion
	}
	if st.Restriction != nil {
		return st.Restriction.Base, model.DerivationRestriction
	}
	return model.QName{}, 0
}

func baseForComplexType(ct *model.ComplexType) (model.QName, model.DerivationMethod) {
	if ct == nil {
		return model.QName{}, 0
	}
	baseQName := model.QName{}
	if content := ct.Content(); content != nil {
		baseQName = content.BaseTypeQName()
	}
	if baseQName.IsZero() {
		if ct.QName.Namespace == model.XSDNamespace && ct.QName.Local == "anyType" {
			return model.QName{}, 0
		}
		return model.AnyTypeQName(), model.DerivationRestriction
	}
	method := ct.DerivationMethod
	if method == 0 {
		method = model.DerivationRestriction
	}
	return baseQName, method
}
