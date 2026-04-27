package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) addAllModel(group contentmodel.TreeParticle) (runtime.ModelRef, error) {
	if group.Kind != contentmodel.TreeGroup || group.Group != contentmodel.TreeAll {
		return runtime.ModelRef{Kind: runtime.ModelNone}, nil
	}
	if group.Min.Unbounded {
		return runtime.ModelRef{}, fmt.Errorf("runtime build: all group minOccurs unbounded")
	}
	if group.Max.Unbounded {
		return runtime.ModelRef{}, fmt.Errorf("runtime build: all group maxOccurs unbounded")
	}
	if group.Max.Value > 1 {
		return runtime.ModelRef{}, fmt.Errorf("runtime build: all group maxOccurs must be <= 1")
	}

	allModel := runtime.AllModel{
		MinOccurs: group.Min.Value,
		Mixed:     false,
	}
	for _, child := range group.Children {
		if child.Kind != contentmodel.TreeElement {
			return runtime.ModelRef{}, fmt.Errorf("runtime build: all group member must be element")
		}
		elemID := runtime.ElemID(child.ElementID)
		if _, ok := b.rt.Element(elemID); !ok {
			return runtime.ModelRef{}, fmt.Errorf("runtime build: all group element %d missing ID", child.ElementID)
		}
		member := runtime.AllMember{
			Elem:     elemID,
			Optional: treeOccursZero(child.Min),
		}
		if child.AllowsSubstitution {
			member.AllowsSubst = true
			list, err := b.substitutionMembers(child.ElementID)
			if err != nil {
				return runtime.ModelRef{}, err
			}
			if len(list) > 0 {
				member.SubstOff = uint32(b.rt.AllSubstitutionCount())
				for _, id := range list {
					if _, err := b.assembler.AppendAllSubstitution(runtime.ElemID(id)); err != nil {
						return runtime.ModelRef{}, err
					}
				}
				member.SubstLen = uint32(b.rt.AllSubstitutionCount()) - member.SubstOff
			}
		}
		allModel.Members = append(allModel.Members, member)
	}

	return b.assembler.AppendAllModel(allModel)
}

func (b *schemaBuilder) addRejectAllModel() (runtime.ModelRef, error) {
	return b.assembler.AppendNFAModel(runtime.NFAModel{
		Nullable:  false,
		Start:     runtime.BitsetRef{},
		Accept:    runtime.BitsetRef{},
		FollowOff: 0,
		FollowLen: 0,
	})
}

func (b *schemaBuilder) substitutionMembers(head uint32) ([]uint32, error) {
	if head == 0 || int(head) > len(b.schema.Elements) {
		return nil, fmt.Errorf("runtime build: substitution head %d out of range", head)
	}
	headElem := b.schema.Elements[head-1]
	if headElem.Block&1 != 0 {
		if headElem.Abstract {
			return nil, fmt.Errorf("abstract head %s blocks substitution", formatIRName(headElem.Name))
		}
		return []uint32{head}, nil
	}

	headType, ok := b.rt.Type(b.mustTypeID(headElem.TypeDecl))
	if !ok {
		return nil, fmt.Errorf("runtime build: substitution head type out of range")
	}
	blocked := blockedDerivations(headType, headElem.Block)
	seen := map[schemaElementKey]bool{{id: head}: true}
	out := make([]uint32, 0)
	if !headElem.Abstract {
		out = append(out, head)
	}
	queue := []uint32{head}
	hasMember := false
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, member := range b.schema.Elements {
			if uint32(member.SubstitutionHead) != current {
				continue
			}
			key := schemaElementKey{id: uint32(member.ID)}
			if seen[key] {
				continue
			}
			seen[key] = true
			queue = append(queue, uint32(member.ID))
			hasMember = true
			if member.Abstract {
				continue
			}
			if blocked != 0 && !b.derivationAllowed(member.TypeDecl, headElem.TypeDecl, blocked) {
				continue
			}
			out = append(out, uint32(member.ID))
		}
	}
	if len(out) == 0 {
		if headElem.Abstract && !hasMember {
			return out, nil
		}
		return nil, fmt.Errorf("abstract head %s has no substitutable members", formatIRName(headElem.Name))
	}
	return out, nil
}

type schemaElementKey struct {
	id uint32
}

func (b *schemaBuilder) derivationAllowed(derived, base schemair.TypeRef, blocked runtime.DerivationMethod) bool {
	derivedID, ok := b.runtimeTypeIDFromIRRef(derived)
	if !ok {
		return true
	}
	baseID, ok := b.runtimeTypeIDFromIRRef(base)
	if !ok {
		return true
	}
	derivedType, ok := b.rt.Type(derivedID)
	if !ok {
		return true
	}
	ids := b.rt.AncestorIDs(derivedType.AncOff, derivedType.AncLen)
	masks := b.rt.AncestorMasks(derivedType.AncMaskOff, derivedType.AncLen)
	for i, id := range ids {
		if id != baseID {
			continue
		}
		if i >= len(masks) {
			return true
		}
		return masks[i]&blocked == 0
	}
	return true
}

func (b *schemaBuilder) mustTypeID(ref schemair.TypeRef) runtime.TypeID {
	id, _ := b.runtimeTypeIDFromIRRef(ref)
	return id
}

func blockedDerivations(typ runtime.Type, block schemair.ElementBlock) runtime.DerivationMethod {
	out := typ.Block
	if block&schemair.ElementBlockExtension != 0 {
		out |= runtime.DerExtension
	}
	if block&schemair.ElementBlockRestriction != 0 {
		out |= runtime.DerRestriction
	}
	return out
}
