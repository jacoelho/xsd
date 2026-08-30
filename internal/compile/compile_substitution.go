package compile

import (
	"errors"
	"strconv"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) compileSubstitutions() error {
	compilation := newSubstitutionCompilation(c)
	if err := compilation.linkHeads(); err != nil {
		return err
	}
	compilation.propagateInheritedTypes()
	if err := compilation.rejectCycles(); err != nil {
		return err
	}
	if err := compilation.finalizeElementConstraints(); err != nil {
		return err
	}
	table, err := c.rt.buildSubstitutionTable(compilation.elements, c.limits.MaxSubstitutionClosureEntries)
	if err != nil {
		return c.substitutionTableError(err, compilation.elements)
	}
	c.installFinalizedElements(compilation.elements, table)
	c.pendingElementConstraints = nil
	return nil
}

type substitutionCompilation struct {
	compiler     *compiler
	elements     []runtime.ElementDecl
	children     [][]runtime.ElementID
	indegree     []uint8
	inheritsType []bool
	members      []runtime.QName
}

func newSubstitutionCompilation(c *compiler) substitutionCompilation {
	elements := c.elementCopies()
	return substitutionCompilation{
		compiler: c, elements: elements,
		children: make([][]runtime.ElementID, len(elements)),
		indegree: make([]uint8, len(elements)), inheritsType: make([]bool, len(elements)),
		members: sortedBuildQNames(&c.rt, c.elementRaw),
	}
}

func (s *substitutionCompilation) linkHeads() error {
	for _, member := range s.members {
		if err := s.linkHead(member); err != nil {
			return err
		}
	}
	return nil
}

func (s *substitutionCompilation) linkHead(memberQName runtime.QName) error {
	raw := s.compiler.elementRaw[memberQName]
	headLexical, ok := raw.node.attr(vocab.XSDAttrSubstitutionGroup)
	if !ok {
		return nil
	}
	member, ok := s.compiler.elementDone[memberQName]
	if !ok || !runtime.ValidElementID(member, len(s.elements)) {
		return xsderrors.InternalInvariant("substitution member was not compiled")
	}
	s.inheritsType[member] = elementUsesSubstitutionType(raw.node)
	headQName, err := s.compiler.resolveQNameChecked(raw.node, raw.ctx, headLexical)
	if err != nil {
		return err
	}
	head, ok := s.compiler.elementDone[headQName]
	if !ok {
		return nil
	}
	if !runtime.ValidElementID(head, len(s.elements)) {
		return xsderrors.InternalInvariant("substitution head was not compiled")
	}
	s.elements[member].SubstHead = head
	s.children[head] = append(s.children[head], member)
	s.indegree[member] = 1
	return nil
}

func (s *substitutionCompilation) propagateInheritedTypes() {
	queue := make([]runtime.ElementID, 0, len(s.elements))
	for id, degree := range s.indegree {
		if degree == 0 {
			queue = append(queue, runtime.ElementID(id))
		}
	}
	for next := 0; next < len(queue); next++ {
		head := queue[next]
		for _, member := range s.children[head] {
			if s.inheritsType[member] {
				s.elements[member].Type = s.elements[head].Type
			}
			s.indegree[member] = 0
			queue = append(queue, member)
		}
	}
}

func (s *substitutionCompilation) rejectCycles() error {
	for _, memberQName := range s.members {
		member, ok := s.compiler.elementDone[memberQName]
		if !ok || s.indegree[member] == 0 {
			continue
		}
		return s.cycleError(member)
	}
	return nil
}

func (s *substitutionCompilation) cycleError(member runtime.ElementID) error {
	cycle, err := substitutionCycleElement(member, s.elements)
	if err != nil {
		return err
	}
	name := s.elements[cycle].Name
	diagnostic := xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "cyclic substitution group "+s.compiler.rt.formatName(name))
	if raw, exists := s.compiler.elementRaw[name]; exists {
		return withSchemaCompileLocation(raw.node, diagnostic)
	}
	return diagnostic
}

func (s *substitutionCompilation) finalizeElementConstraints() error {
	for _, pending := range s.compiler.pendingElementConstraints {
		if err := s.finalizeElementConstraint(pending); err != nil {
			return err
		}
	}
	return nil
}

func (s *substitutionCompilation) finalizeElementConstraint(pending pendingElementConstraint) error {
	if !runtime.ValidElementID(pending.element, len(s.elements)) {
		return xsderrors.InternalInvariant("pending element constraint references invalid element")
	}
	decl := s.elements[pending.element]
	if decl.Default != nil || decl.Fixed != nil {
		return xsderrors.InternalInvariant("pending element constraint targets finalized declaration")
	}
	switch pending.kind {
	case runtime.DeclarationValueConstraintDefault:
		decl.Default = &runtime.ValueConstraint{Lexical: pending.lexical}
	case runtime.DeclarationValueConstraintFixed:
		decl.Fixed = &runtime.ValueConstraint{Lexical: pending.lexical}
	case runtime.DeclarationValueConstraintNone, runtime.DeclarationValueConstraintConflict:
		return xsderrors.InternalInvariant("pending element constraint has invalid kind")
	default:
		err := xsderrors.InternalInvariant("pending element constraint has invalid kind")
		return err
	}
	if err := s.compiler.validateElementValueConstraints(&decl, pending.node, s.compiler.simpleTypeUnavailable); err != nil {
		return withSchemaCompileLocation(pending.node, err)
	}
	s.elements[pending.element] = decl
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
	err := SchemaComponentMissingError(SchemaComponentType, c.rt.formatName(q))
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
