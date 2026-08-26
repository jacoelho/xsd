package compile

import (
	"errors"
	"strconv"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) compileSubstitutions() error {
	elements := c.elementCopies()
	children := make([][]runtime.ElementID, len(elements))
	indegree := make([]uint8, len(elements))
	inheritsType := make([]bool, len(elements))
	members := sortedBuildQNames(&c.rt, c.elementRaw)

	for _, memberQName := range members {
		raw := c.elementRaw[memberQName]
		headLex, ok := raw.node.attr(vocab.XSDAttrSubstitutionGroup)
		if !ok {
			continue
		}
		memberID, ok := c.elementDone[memberQName]
		if !ok || !runtime.ValidElementID(memberID, len(elements)) {
			return xsderrors.InternalInvariant("substitution member was not compiled")
		}
		inheritsType[memberID] = elementUsesSubstitutionType(raw.node)
		headQName, err := c.resolveQNameChecked(raw.node, raw.ctx, headLex)
		if err != nil {
			return err
		}
		headID, ok := c.elementDone[headQName]
		if !ok {
			continue
		}
		if !runtime.ValidElementID(headID, len(elements)) {
			return xsderrors.InternalInvariant("substitution head was not compiled")
		}
		elements[memberID].SubstHead = headID
		children[headID] = append(children[headID], memberID)
		indegree[memberID] = 1
	}

	queue := make([]runtime.ElementID, 0, len(elements))
	for id, degree := range indegree {
		if degree == 0 {
			queue = append(queue, runtime.ElementID(id))
		}
	}
	for next := 0; next < len(queue); next++ {
		headID := queue[next]
		for _, memberID := range children[headID] {
			if inheritsType[memberID] {
				elements[memberID].Type = elements[headID].Type
			}
			indegree[memberID] = 0
			queue = append(queue, memberID)
		}
	}
	for _, memberQName := range members {
		memberID, ok := c.elementDone[memberQName]
		if !ok || indegree[memberID] == 0 {
			continue
		}
		cycleID, err := substitutionCycleElement(memberID, elements)
		if err != nil {
			return err
		}
		cycleName := elements[cycleID].Name
		cycleErr := xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "cyclic substitution group "+c.rt.formatName(cycleName))
		if raw, exists := c.elementRaw[cycleName]; exists {
			return withSchemaCompileLocation(raw.node, cycleErr)
		}
		return cycleErr
	}

	for _, pending := range c.pendingElementConstraints {
		if !runtime.ValidElementID(pending.element, len(elements)) {
			return xsderrors.InternalInvariant("pending element constraint references invalid element")
		}
		decl := elements[pending.element]
		if decl.Default != nil || decl.Fixed != nil {
			return xsderrors.InternalInvariant("pending element constraint targets finalized declaration")
		}
		if pending.hasDefault {
			decl.Default = &runtime.ValueConstraint{Lexical: pending.defaultLexical}
		}
		if pending.hasFixed {
			decl.Fixed = &runtime.ValueConstraint{Lexical: pending.fixedLexical}
		}
		if err := c.validateElementValueConstraints(&decl, pending.node, c.simpleTypeUnavailable); err != nil {
			return withSchemaCompileLocation(pending.node, err)
		}
		elements[pending.element] = decl
	}

	table, err := c.rt.buildSubstitutionTable(elements, c.limits.MaxSubstitutionClosureEntries)
	if err != nil {
		return c.substitutionTableError(err, elements)
	}
	c.installFinalizedElements(elements, table)
	c.pendingElementConstraints = nil
	return nil
}

func (c *compiler) substitutionTableError(err error, elements []runtime.ElementDecl) error {
	if cycle, ok := errors.AsType[runtime.SubstitutionCycleError](err); ok {
		name, nameOK := c.rt.ElementName(cycle.Element)
		if nameOK {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "cyclic substitution group "+c.rt.formatName(name))
		}
	}
	if limitErr, ok := errors.AsType[runtime.SubstitutionClosureLimitError](err); ok {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "substitution-group closure exceeds MaxSubstitutionClosureEntries ("+strconv.Itoa(limitErr.Limit)+")")
	}
	if membership, ok := errors.AsType[runtime.SubstitutionMembershipError](err); ok &&
		runtime.ValidElementID(membership.Member, len(elements)) && runtime.ValidElementID(membership.Head, len(elements)) {
		member := elements[membership.Member]
		head := elements[membership.Head]
		diagnostic := substitutionMembershipDiagnostic(membership.Cause, SubstitutionMembershipLabels{
			MemberName: c.rt.formatName(member.Name),
			MemberType: c.rt.TypeLabel(member.Type),
			HeadName:   c.rt.formatName(head.Name),
			HeadType:   c.rt.TypeLabel(head.Type),
		})
		if raw, exists := c.elementRaw[member.Name]; exists {
			return withSchemaCompileLocation(raw.node, diagnostic)
		}
		return diagnostic
	}
	return xsderrors.InternalInvariant(err.Error())
}

func substitutionCycleElement(start runtime.ElementID, elements []runtime.ElementDecl) (runtime.ElementID, error) {
	current := start
	for range elements {
		if !runtime.ValidElementID(current, len(elements)) {
			return runtime.NoElement, xsderrors.InternalInvariant("substitution cycle references invalid element")
		}
		current = elements[current].SubstHead
	}
	if !runtime.ValidElementID(current, len(elements)) {
		return runtime.NoElement, xsderrors.InternalInvariant("substitution residual does not lead to a cycle")
	}
	return current, nil
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

func (c *compiler) typeQNameMayBeUnavailable(q runtime.QName) bool {
	return c.rt.namespaceURI(q.Namespace) != vocab.XSDNamespaceURI
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
