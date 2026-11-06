package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// validateIdentityConstraintNameSource validates declaration-level identity
// constraint name source.
func validateIdentityConstraintNameSource(name lexicalAttribute) error {
	if !name.Present || name.Value == "" {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity constraint missing name")
	}
	return nil
}

// identityConstraintReferSource keeps the declaration kind and refer attribute
// together for keyref source admission.
type identityConstraintReferSource struct {
	Local string
	Refer lexicalAttribute
}

// validateIdentityConstraintReferSource validates declaration-level keyref
// reference source.
func validateIdentityConstraintReferSource(source identityConstraintReferSource) error {
	if source.Local == vocab.XSDElemKeyref && !source.Refer.Present {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "keyref missing refer")
	}
	return nil
}

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
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity constraint "+diagnostictext.Quote(local))
	}
}

// checkIdentityConstraintNameAvailable rejects duplicate global identity
// constraint names.
func checkIdentityConstraintNameAvailable(
	identities map[QName]IdentityConstraintID,
	name QName,
	label string,
) error {
	if _, exists := identities[name]; exists {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaDuplicate, "duplicate identity constraint "+label)
	}
	return nil
}

// resolveIdentityConstraintRefer resolves a keyref refer name against declared
// global identity constraints.
func resolveIdentityConstraintRefer(
	identities map[QName]IdentityConstraintID,
	name QName,
	label string,
) (IdentityConstraintID, error) {
	id, exists := identities[name]
	if !exists {
		return NoIdentityConstraint, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unknown keyref refer "+label)
	}
	return id, nil
}

// validateIdentityReferences validates compile-time keyref reference shape.
func validateIdentityReferences(identities []identityConstraintSpec) error {
	for _, ic := range identities {
		if ic.Kind != IdentityKeyRef {
			continue
		}
		if !validUint32Index(uint32(ic.Refer), len(identities)) {
			return xsderrors.InternalInvariant("keyref references missing identity constraint")
		}
		ref := identities[ic.Refer]
		if ref.Kind == IdentityKeyRef {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "keyref refer cannot be keyref")
		}
		if len(ic.Fields) != len(ref.Fields) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "keyref field count does not match referenced key")
		}
	}
	return nil
}
