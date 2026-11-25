package schema

import (
	"errors"
	"strconv"

	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// compileSubstitutions runs after every type and substitution affiliation is
// complete. It finalizes element value constraints and effective substitution
// types on a copy of the elements and installs them with the bounded transitive
// substitution table together, so a failure leaves the build unchanged. That
// table is the only substitution lookup the published schema retains.
func (c *compiler) compileSubstitutions() error {
	compilation := newSubstitutionCompilation(c)
	if err := compilation.linkAffiliations(); err != nil {
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
	elements     []elementDecl
	members      [][]ElementID
	indegree     []uint8
	inheritsType []bool
	elementNames []QName
}

func newSubstitutionCompilation(c *compiler) substitutionCompilation {
	elements := c.elementCopies()
	return substitutionCompilation{
		compiler: c, elements: elements,
		members:  make([][]ElementID, len(elements)),
		indegree: make([]uint8, len(elements)), inheritsType: make([]bool, len(elements)),
		elementNames: sortedBuildQNames(&c.rt, c.elementComponents),
	}
}

func (s *substitutionCompilation) linkAffiliations() error {
	for _, name := range s.elementNames {
		if err := s.linkAffiliation(name); err != nil {
			return err
		}
	}
	return nil
}

func (s *substitutionCompilation) linkAffiliation(memberQName QName) error {
	raw := s.compiler.elementComponents[memberQName]
	source, err := requiredSemanticSource[*schemaElementSource](raw.sourceNode())
	if err != nil {
		return err
	}
	if !source.SubstitutionGroup.Lexical.Present {
		return nil
	}
	member, ok := s.compiler.rt.GlobalElements[memberQName]
	if !ok || !validElementID(member, len(s.elements)) {
		return xsderrors.InternalInvariant("substitution member was not compiled", nil)
	}
	s.inheritsType[member] = elementUsesSubstitutionType(raw.sourceNode(), source)
	headQName, err := s.compiler.referenceQName(raw.ctx, raw.sourceNode(), source.SubstitutionGroup.Name)
	if err != nil {
		return err
	}
	head, ok := s.compiler.rt.GlobalElements[headQName]
	if !ok {
		return nil
	}
	if !validElementID(head, len(s.elements)) {
		return xsderrors.InternalInvariant("substitution head was not compiled", nil)
	}
	s.elements[member].SubstitutionGroupAffiliation = head
	s.members[head] = append(s.members[head], member)
	s.indegree[member] = 1
	return nil
}

func (s *substitutionCompilation) propagateInheritedTypes() {
	queue := make([]ElementID, 0, len(s.elements))
	for id, degree := range s.indegree {
		if degree == 0 {
			queue = append(queue, ElementID(id))
		}
	}
	for next := 0; next < len(queue); next++ {
		head := queue[next]
		for _, member := range s.members[head] {
			if s.inheritsType[member] {
				s.elements[member].Type = s.elements[head].Type
			}
			s.indegree[member] = 0
			queue = append(queue, member)
		}
	}
}

func (s *substitutionCompilation) rejectCycles() error {
	for _, memberQName := range s.elementNames {
		member, ok := s.compiler.rt.GlobalElements[memberQName]
		if !ok || s.indegree[member] == 0 {
			continue
		}
		return s.cycleError(member)
	}
	return nil
}

func (s *substitutionCompilation) cycleError(member ElementID) error {
	cycle, err := substitutionCycleElement(member, s.elements)
	if err != nil {
		return err
	}
	name := s.elements[cycle].Name
	return s.compiler.atGlobalElement(name, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "cyclic substitution group "+s.compiler.rt.formatName(name), nil))
}

// atGlobalElement locates diagnostic at the declaration of the global element
// name. Substitution group members and heads are global declarations, so a
// name without one is an internal invariant.
func (c *compiler) atGlobalElement(name QName, diagnostic error) error {
	raw, exists := c.elementComponents[name]
	if !exists {
		return xsderrors.InternalInvariant("substitution group element "+c.rt.formatName(name)+" has no global declaration", nil)
	}
	return withSchemaCompileLocation(raw.sourceNode(), diagnostic)
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
	if !validElementID(pending.element, len(s.elements)) {
		return xsderrors.InternalInvariant("pending element constraint references invalid element", nil)
	}
	decl := s.elements[pending.element]
	if decl.ValueConstraint != nil {
		return xsderrors.InternalInvariant("pending element constraint targets finalized declaration", nil)
	}
	decl.ValueConstraint = pending.constraint
	if err := s.compiler.validateElementValueConstraints(&decl, pending.node, s.compiler.rt.simpleTypeUnavailable); err != nil {
		return withSchemaCompileLocation(pending.node, err)
	}
	s.elements[pending.element] = decl
	return nil
}

func (c *compiler) substitutionTableError(err error, elements []elementDecl) error {
	if cycle, ok := errors.AsType[substitutionCycleError](err); ok {
		name, nameOK := c.rt.ElementName(cycle.Element)
		if nameOK {
			return c.atGlobalElement(name, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "cyclic substitution group "+c.rt.formatName(name), nil))
		}
	}
	if limitErr, ok := errors.AsType[substitutionClosureLimitError](err); ok {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "substitution-group closure exceeds MaxSubstitutionClosureEntries ("+strconv.Itoa(limitErr.Limit)+")", nil)
	}
	if membership, ok := errors.AsType[substitutionMembershipError](err); ok &&
		validElementID(membership.Member, len(elements)) && validElementID(membership.Head, len(elements)) {
		return c.substitutionMembershipError(membership, elements[membership.Member], elements[membership.Head])
	}
	if diagnostic, ok := errors.AsType[*xsderrors.Error](err); ok && diagnostic != nil {
		return err
	}
	return xsderrors.InternalInvariant("", err)
}

// substitutionMembershipError reports the rejected membership of member in
// the substitution group of head, located at member.
func (c *compiler) substitutionMembershipError(membership substitutionMembershipError, member, head elementDecl) error {
	memberType, err := c.rt.elementTypeLabel(member.Type)
	if err != nil {
		return err
	}
	headType, err := c.rt.elementTypeLabel(head.Type)
	if err != nil {
		return err
	}
	diagnostic := substitutionMembershipDiagnostic(membership.Cause, substitutionMembershipLabels{
		MemberName: c.rt.formatName(member.Name),
		MemberType: memberType,
		HeadName:   c.rt.formatName(head.Name),
		HeadType:   headType,
	})
	return c.atGlobalElement(member.Name, diagnostic)
}

func substitutionCycleElement(start ElementID, elements []elementDecl) (ElementID, error) {
	current := start
	for range elements {
		if !validElementID(current, len(elements)) {
			return NoElement, xsderrors.InternalInvariant("substitution cycle references invalid element", nil)
		}
		current = elements[current].SubstitutionGroupAffiliation
	}
	if !validElementID(current, len(elements)) {
		return NoElement, xsderrors.InternalInvariant("substitution residual does not lead to a cycle", nil)
	}
	return current, nil
}

func elementUsesSubstitutionType(n *schemaNode, source *schemaElementSource) bool {
	if source.Type.Lexical.Present {
		return false
	}
	if n.firstXS(vocab.XSDElemSimpleType) != nil || n.firstXS(vocab.XSDElemComplexType) != nil {
		return false
	}
	return source.SubstitutionGroup.Lexical.Present
}

func (c *compiler) typeQNameKnown(q QName) bool {
	return c.simpleTypeQNameKnown(q) || c.complexTypeQNameKnown(q)
}

func (c *compiler) typeQNameMayBeUnavailable(q QName) bool {
	return c.rt.namespaceURI(q.Namespace) != vocab.XSDNamespaceURI
}

func (c *compiler) simpleTypeQNameKnown(q QName) bool {
	if c.rt.GlobalTypes[q].IsSimple() {
		return true
	}
	_, ok := c.simpleComponents[q]
	return ok
}

func (c *compiler) complexTypeQNameKnown(q QName) bool {
	if c.rt.GlobalTypes[q].IsComplex() {
		return true
	}
	_, ok := c.complexComponents[q]
	return ok
}
