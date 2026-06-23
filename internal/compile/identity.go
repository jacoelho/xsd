package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// ValidateIdentityConstraintNameSource validates declaration-level identity
// constraint name source.
func ValidateIdentityConstraintNameSource(hasName bool) error {
	if !hasName {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "identity constraint missing name")
	}
	return nil
}

// ValidateIdentityConstraintReferSource validates declaration-level keyref
// reference source.
func ValidateIdentityConstraintReferSource(local string, hasRefer bool) error {
	if local == vocab.XSDElemKeyref && !hasRefer {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "keyref missing refer")
	}
	return nil
}

// IdentityConstraintKindForLocal maps identity constraint element names to
// runtime identity kinds.
func IdentityConstraintKindForLocal(local string) (runtime.IdentityKind, error) {
	switch local {
	case vocab.XSDElemKey:
		return runtime.IdentityKey, nil
	case vocab.XSDElemUnique:
		return runtime.IdentityUnique, nil
	case vocab.XSDElemKeyref:
		return runtime.IdentityKeyRef, nil
	default:
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "invalid identity constraint "+local)
	}
}

// CheckIdentityConstraintNameAvailable rejects duplicate global identity
// constraint names.
func CheckIdentityConstraintNameAvailable(
	identities map[runtime.QName]runtime.IdentityConstraintID,
	name runtime.QName,
	label string,
) error {
	if _, exists := identities[name]; exists {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaDuplicate, "duplicate identity constraint "+label)
	}
	return nil
}

// ResolveIdentityConstraintRefer resolves a keyref refer name against declared
// global identity constraints.
func ResolveIdentityConstraintRefer(
	identities map[runtime.QName]runtime.IdentityConstraintID,
	name runtime.QName,
	label string,
) (runtime.IdentityConstraintID, error) {
	id, exists := identities[name]
	if !exists {
		return runtime.NoIdentityConstraint, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "unknown keyref refer "+label)
	}
	return id, nil
}

// ValidateIdentityReferences validates compile-time keyref reference shape.
func ValidateIdentityReferences(identities []runtime.IdentityConstraint) error {
	for _, ic := range identities {
		if ic.Kind != runtime.IdentityKeyRef {
			continue
		}
		if !runtime.ValidUint32Index(uint32(ic.Refer), len(identities)) {
			return xsderrors.InternalInvariant("keyref references missing identity constraint")
		}
		ref := identities[ic.Refer]
		if ref.Kind == runtime.IdentityKeyRef {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "keyref refer cannot be keyref")
		}
		if len(ic.Fields) != len(ref.Fields) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaIdentity, "keyref field count does not match referenced key")
		}
	}
	return nil
}
