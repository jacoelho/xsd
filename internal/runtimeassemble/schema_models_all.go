package runtimeassemble

import (
	"fmt"

	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (b *schemaBuilder) addAllModel(group *model.ModelGroup) (runtime.ModelRef, error) {
	if group == nil {
		return runtime.ModelRef{Kind: runtime.ModelNone}, nil
	}
	minOccurs, ok := group.MinOccurs.Int()
	if !ok {
		return runtime.ModelRef{}, fmt.Errorf("runtime build: all group minOccurs too large")
	}
	if group.MaxOccurs.IsUnbounded() {
		return runtime.ModelRef{}, fmt.Errorf("runtime build: all group maxOccurs unbounded")
	}
	if maxOccurs, ok := group.MaxOccurs.Int(); !ok || maxOccurs > 1 {
		return runtime.ModelRef{}, fmt.Errorf("runtime build: all group maxOccurs must be <= 1")
	}

	allModel := runtime.AllModel{
		MinOccurs: uint32(minOccurs),
		Mixed:     false,
	}
	for _, particle := range group.Particles {
		elem, ok := particle.(*model.ElementDecl)
		if !ok || elem == nil {
			return runtime.ModelRef{}, fmt.Errorf("runtime build: all group member must be element")
		}
		elemID, ok := b.runtimeElemID(elem)
		if !ok {
			return runtime.ModelRef{}, fmt.Errorf("runtime build: all group element %s missing ID", elem.Name)
		}
		minOccurs := elem.MinOcc()
		optional := minOccurs.IsZero()
		member := runtime.AllMember{
			Elem:     elemID,
			Optional: optional,
		}
		if elem.IsReference {
			member.AllowsSubst = true
			head := elem
			if resolved := b.resolveSubstitutionHead(elem); resolved != nil {
				head = resolved
			}
			list, err := models.ExpandSubstitutionMembers(head, b.substitutionMembers)
			if err != nil {
				return runtime.ModelRef{}, err
			}
			if len(list) > 0 {
				member.SubstOff = uint32(len(b.rt.Models.AllSubst))
				for _, decl := range list {
					if decl == nil {
						continue
					}
					memberID, ok := b.runtimeElemID(decl)
					if !ok {
						return runtime.ModelRef{}, fmt.Errorf("runtime build: all group substitution element %s missing ID", decl.Name)
					}
					b.rt.Models.AllSubst = append(b.rt.Models.AllSubst, memberID)
				}
				member.SubstLen = uint32(len(b.rt.Models.AllSubst)) - member.SubstOff
			}
		}
		allModel.Members = append(allModel.Members, member)
	}

	id := uint32(len(b.rt.Models.All))
	b.rt.Models.All = append(b.rt.Models.All, allModel)
	return runtime.ModelRef{Kind: runtime.ModelAll, ID: id}, nil
}

func (b *schemaBuilder) addRejectAllModel() runtime.ModelRef {
	id := uint32(len(b.rt.Models.NFA))
	b.rt.Models.NFA = append(b.rt.Models.NFA, runtime.NFAModel{
		Nullable:  false,
		Start:     runtime.BitsetRef{},
		Accept:    runtime.BitsetRef{},
		FollowOff: 0,
		FollowLen: 0,
	})
	return runtime.ModelRef{Kind: runtime.ModelNFA, ID: id}
}

func (b *schemaBuilder) substitutionMembers(head *model.ElementDecl) []*model.ElementDecl {
	if head == nil {
		return nil
	}
	queue := []model.QName{head.Name}
	seen := make(map[model.QName]bool)
	seen[head.Name] = true
	out := make([]*model.ElementDecl, 0)

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, memberName := range b.schema.SubstitutionGroups[name] {
			if seen[memberName] {
				continue
			}
			seen[memberName] = true
			decl := b.schema.ElementDecls[memberName]
			if decl == nil {
				continue
			}
			out = append(out, decl)
			queue = append(queue, memberName)
		}
	}
	return out
}

func (b *schemaBuilder) resolveSubstitutionHead(decl *model.ElementDecl) *model.ElementDecl {
	if decl == nil || !decl.IsReference || b == nil || b.schema == nil {
		return decl
	}
	if head := b.schema.ElementDecls[decl.Name]; head != nil {
		return head
	}
	return decl
}
