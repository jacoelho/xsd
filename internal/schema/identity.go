package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// identityConstraintKindForLocal maps identity constraint element names to
// runtime identity kinds.
func identityConstraintKindForLocal(local string) (IdentityKind, error) {
	switch local {
	case vocab.XSDElemKey:
		return IdentityKey, nil
	case vocab.XSDElemUnique:
		return IdentityUnique, nil
	case vocab.XSDElemKeyref:
		return IdentityKeyRef, nil
	default:
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity constraint "+diagnostictext.Quote(local), nil)
	}
}

// newDeclaredIdentityConstraint returns the placeholder of a declared
// identity constraint: unnamed until nameIdentityConstraint interns its name,
// and without paths until it compiles.
func newDeclaredIdentityConstraint() identityConstraintSpec {
	return identityConstraintSpec{Name: noQName(), Refer: NoIdentityConstraint}
}

// newIdentityConstraint builds an identity constraint over selector and
// fields without copying them, so the constraints compiled from one node in
// several document contexts share the paths compiler.identityPaths keeps for
// that node.
func newIdentityConstraint(kind IdentityKind, name QName, refer IdentityConstraintID, selector []identityPath, fields []identityField) identityConstraintSpec {
	if kind != IdentityKeyRef {
		refer = NoIdentityConstraint
	}
	return identityConstraintSpec{
		Selector: selector,
		Fields:   fields,
		Name:     name,
		Refer:    refer,
		Kind:     kind,
	}
}

// checkIdentityConstraintNameAvailable rejects name if an identity
// constraint is already declared with it.
func (c *compiler) checkIdentityConstraintNameAvailable(name ExpandedName) error {
	if _, exists := c.identityNames[name]; exists {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaDuplicate, "duplicate identity constraint "+diagnostictext.ExpandedName(name.Namespace, name.Local), nil)
	}
	return nil
}

// resolveIdentityConstraintRefer resolves the keyref refer q against every
// declared identity constraint, named or not.
func (c *compiler) resolveIdentityConstraintRefer(q QName) (IdentityConstraintID, error) {
	id, exists := c.identityNames[ExpandedName{Namespace: c.rt.Names.Namespace(q.Namespace), Local: c.rt.Names.Local(q.Local)}]
	if !exists {
		return NoIdentityConstraint, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unknown keyref refer "+c.rt.Names.Format(q), nil)
	}
	return id, nil
}

// validateKeyrefReference validates the compile-time reference shape of ic,
// one of identities: a keyref refers to a key or unique constraint with as
// many fields. Other kinds have no reference.
func validateKeyrefReference(identities []identityConstraintSpec, ic identityConstraintSpec) error {
	if ic.Kind != IdentityKeyRef {
		return nil
	}
	if !validUint32Index(uint32(ic.Refer), len(identities)) {
		return xsderrors.InternalInvariant("keyref references missing identity constraint", nil)
	}
	ref := identities[ic.Refer]
	if ref.Kind == IdentityKeyRef {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "keyref refer cannot be keyref", nil)
	}
	if len(ic.Fields) != len(ref.Fields) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "keyref field count does not match referenced key", nil)
	}
	return nil
}
