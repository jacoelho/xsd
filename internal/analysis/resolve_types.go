package analysis

import (
	"github.com/jacoelho/xsd/internal/typeresolve"
	model "github.com/jacoelho/xsd/internal/types"
)

func (r *referenceResolver) resolveType(typ model.Type) error {
	if typ == nil || typ.IsBuiltin() {
		return nil
	}

	switch typed := typ.(type) {
	case *model.SimpleType:
		if model.IsPlaceholderSimpleType(typed) {
			return r.resolveTypeQName(typed.QName)
		}
		return r.resolveSimpleType(typed)
	case *model.ComplexType:
		return r.resolveComplexType(typed)
	default:
		return nil
	}
}

func (r *referenceResolver) resolveSimpleType(st *model.SimpleType) error {
	if st == nil {
		return nil
	}
	return r.simpleTypeState.Resolve(st, nil, func() error {
		if st.Restriction != nil {
			if err := r.resolveTypeQName(st.Restriction.Base); err != nil {
				return err
			}
			if st.Restriction.SimpleType != nil {
				if err := r.resolveType(st.Restriction.SimpleType); err != nil {
					return err
				}
			}
		}
		if st.List != nil {
			if st.List.InlineItemType != nil {
				if err := r.resolveType(st.List.InlineItemType); err != nil {
					return err
				}
			}
			if !st.List.ItemType.IsZero() {
				if err := r.resolveTypeQName(st.List.ItemType); err != nil {
					return err
				}
			}
		}
		if st.Union != nil {
			for _, member := range st.Union.MemberTypes {
				if err := r.resolveTypeQName(member); err != nil {
					return err
				}
			}
			for _, inline := range st.Union.InlineTypes {
				if err := r.resolveType(inline); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *referenceResolver) resolveComplexType(ct *model.ComplexType) error {
	if ct == nil {
		return nil
	}
	return r.complexTypeState.Resolve(ct, nil, func() error {
		switch content := ct.Content().(type) {
		case *model.ElementContent:
			if err := r.resolveParticle(content.Particle); err != nil {
				return err
			}
		case *model.SimpleContent:
			if err := r.resolveSimpleContent(content); err != nil {
				return err
			}
		case *model.ComplexContent:
			if err := r.resolveComplexContent(content); err != nil {
				return err
			}
		case *model.EmptyContent:
			// no-op
		}

		if err := r.resolveAttributes(ct.Attributes(), ct.AttrGroups); err != nil {
			return err
		}
		return nil
	})
}

func (r *referenceResolver) resolveSimpleContent(content *model.SimpleContent) error {
	if content == nil {
		return nil
	}
	if !content.Base.IsZero() {
		if err := r.resolveTypeQName(content.Base); err != nil {
			return err
		}
	}
	if ext := content.ExtensionDef(); ext != nil {
		if err := r.resolveTypeQName(ext.Base); err != nil {
			return err
		}
		return r.resolveAttributes(ext.Attributes, ext.AttrGroups)
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := r.resolveTypeQName(restr.Base); err != nil {
			return err
		}
		if restr.SimpleType != nil {
			if err := r.resolveType(restr.SimpleType); err != nil {
				return err
			}
		}
		return r.resolveAttributes(restr.Attributes, restr.AttrGroups)
	}
	return nil
}

func (r *referenceResolver) resolveComplexContent(content *model.ComplexContent) error {
	if content == nil {
		return nil
	}
	if !content.Base.IsZero() {
		if err := r.resolveTypeQName(content.Base); err != nil {
			return err
		}
	}
	if ext := content.ExtensionDef(); ext != nil {
		if err := r.resolveTypeQName(ext.Base); err != nil {
			return err
		}
		if err := r.resolveParticle(ext.Particle); err != nil {
			return err
		}
		return r.resolveAttributes(ext.Attributes, ext.AttrGroups)
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := r.resolveTypeQName(restr.Base); err != nil {
			return err
		}
		if err := r.resolveParticle(restr.Particle); err != nil {
			return err
		}
		return r.resolveAttributes(restr.Attributes, restr.AttrGroups)
	}
	return nil
}

func (r *referenceResolver) resolveTypeQName(qname model.QName) error {
	return typeresolve.ValidateTypeQName(r.schema, qname)
}
