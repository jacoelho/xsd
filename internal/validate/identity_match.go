package validate

import (
	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/xsderrors"
)

// Identity path programs and immutable dispatch programs are schema-owned.
// Validation retains only document-local active scope membership and scratch.
type identityConstraintProgram = xsdSchema.IdentityConstraintProgramRead

// internalIdentityMetadataError keeps malformed published metadata in the
// internal-invariant error category.
func internalIdentityMetadataError(message string) error {
	return xsderrors.InternalInvariant(message)
}
