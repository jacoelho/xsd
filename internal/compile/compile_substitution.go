package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
)

func (c *compiler) compileSubstitutions() error {
	direct := make(map[runtime.ElementID][]runtime.ElementID)
	for _, memberQName := range SortedQNames(c.elementRaw, c.rt.Names) {
		raw := c.elementRaw[memberQName]
		headLex, ok := raw.node.attr(vocab.XSDAttrSubstitutionGroup)
		if !ok {
			continue
		}
		memberID, err := c.compileElementByQName(memberQName)
		if err != nil {
			return err
		}
		headQName, err := c.resolveQNameChecked(raw.node, raw.ctx, headLex)
		if err != nil {
			return err
		}
		if _, ok := c.elementRaw[headQName]; !ok {
			continue
		}
		headID, err := c.compileElementByQName(headQName)
		if err != nil {
			return err
		}
		if elementUsesSubstitutionType(raw.node) {
			member := c.rt.Elements[memberID]
			member.Type = c.rt.Elements[headID].Type
			replayErr := c.validateElementValueConstraints(&member, raw.node)
			if replayErr != nil {
				return withSchemaCompileLocation(raw.node, replayErr)
			}
			c.completeElement(memberID, member)
		}
		head := c.rt.Elements[headID]
		member := c.rt.Elements[memberID]
		err = ValidateSubstitutionMembership(
			&c.rt,
			head,
			member,
			SubstitutionMembershipLabels{
				MemberName: c.rt.Names.Format(member.Name),
				MemberType: c.rt.TypeLabel(member.Type),
				HeadName:   c.rt.Names.Format(head.Name),
				HeadType:   c.rt.TypeLabel(head.Type),
			},
		)
		if err != nil {
			return err
		}
		member.SubstHead = headID
		c.completeElement(memberID, member)
		direct[headID] = append(direct[headID], memberID)
	}
	substitutions, err := BuildSubstitutionClosure(direct, c.substitutionCycleLabel)
	if err != nil {
		return err
	}
	c.installSubstitutions(substitutions)
	return nil
}

func (c *compiler) substitutionCycleLabel(id runtime.ElementID) (string, bool) {
	if !runtime.ValidElementID(id, len(c.rt.Elements)) {
		return "", false
	}
	return c.rt.Names.Format(c.rt.Elements[id].Name), true
}

func elementUsesSubstitutionType(n *rawNode) bool {
	if _, ok := n.attr(vocab.XSDAttrType); ok {
		return false
	}
	if n.firstXS(vocab.XSDElemSimpleType) != nil || n.firstXS(vocab.XSDElemComplexType) != nil {
		return false
	}
	_, ok := n.attr(vocab.XSDAttrSubstitutionGroup)
	return ok
}

func (c *compiler) resolveTypeQName(q runtime.QName) (runtime.TypeID, error) {
	if id, ok := c.simpleDone[q]; ok {
		return runtime.SimpleRef(id), nil
	}
	if id, ok := c.complexDone[q]; ok {
		return runtime.ComplexRef(id), nil
	}
	if _, ok := c.simpleRaw[q]; ok {
		id, err := c.compileSimpleByQName(q)
		if err != nil {
			return runtime.TypeID{}, err
		}
		return runtime.SimpleRef(id), nil
	}
	if _, ok := c.complexRaw[q]; ok {
		id, err := c.compileComplexByQName(q)
		if err != nil {
			return runtime.TypeID{}, err
		}
		return runtime.ComplexRef(id), nil
	}
	err := CheckSchemaComponentExists(SchemaComponentType, false, c.rt.Names.Format(q))
	return runtime.TypeID{}, err
}

func (c *compiler) typeQNameKnown(q runtime.QName) bool {
	return c.simpleTypeQNameKnown(q) || c.complexTypeQNameKnown(q)
}

func (c *compiler) simpleTypeQNameKnown(q runtime.QName) bool {
	if _, ok := c.simpleDone[q]; ok {
		return true
	}
	_, ok := c.simpleRaw[q]
	return ok
}

func (c *compiler) complexTypeQNameKnown(q runtime.QName) bool {
	if _, ok := c.complexDone[q]; ok {
		return true
	}
	_, ok := c.complexRaw[q]
	return ok
}
