package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
)

func (c *compiler) compileSubstitutions() error {
	direct := make(map[runtime.ElementID][]runtime.ElementID)
	for _, memberQName := range sortedBuildQNames(&c.rt, c.elementRaw) {
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
		member := c.rt.elementCopy(memberID)
		if elementUsesSubstitutionType(raw.node) {
			headType, _ := c.rt.ElementType(headID)
			member.Type = headType
			replayErr := c.validateElementValueConstraints(&member, raw.node)
			if replayErr != nil {
				return withSchemaCompileLocation(raw.node, replayErr)
			}
			c.completeElement(memberID, member)
		}
		head := c.rt.elementCopy(headID)
		err = ValidateSubstitutionMembership(
			&c.rt,
			head,
			member,
			SubstitutionMembershipLabels{
				MemberName: c.rt.formatName(member.Name),
				MemberType: c.rt.TypeLabel(member.Type),
				HeadName:   c.rt.formatName(head.Name),
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
	name, ok := c.rt.ElementName(id)
	if !ok {
		return "", false
	}
	return c.rt.formatName(name), true
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
	err := CheckSchemaComponentExists(SchemaComponentType, false, c.rt.formatName(q))
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
