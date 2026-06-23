package validate

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

// Runtime is the complete read boundary required by validation
// sessions. Splitting it by caller would only move the same runtime value
// between narrower interfaces without reducing the phase surface.
type Runtime interface { //nolint:interfacebloat // This is the session read boundary over existing narrow validation APIs.
	StartRuntime
	NameRuntime
	ContentRuntime
	runtime.ContentFrameRuntime
	CharacterDataRuntime[runtime.ElementTextContent]
	AttributeRuntime
	EndIdentityRuntime
	IdentityRuntime
	SimpleValueIdentityRuntime
	IdentityConstraintRuntime

	HasIdentityConstraints() bool
	SimpleValueNeedsQNameResolver(id runtime.SimpleTypeID) bool
	AttributeUseSetForType(typ runtime.TypeID) (runtime.AttributeUseSetRead, bool, bool)
	AttributeDecl(id runtime.AttributeID) (runtime.AttributeDeclRead, bool)
	SimpleContentType(typ runtime.TypeID) (runtime.SimpleTypeID, bool, bool)
	ElementValueConstraints(id runtime.ElementID) (runtime.ElementValueConstraints, bool, bool)
	SimpleIdentity(id runtime.SimpleTypeID) runtime.SimpleIdentityKind
	ValidateRawSimpleValue(id runtime.SimpleTypeID, raw []byte) (bool, error)
	ValidateSimpleValue(id runtime.SimpleTypeID, lexical string, resolve runtime.ResolveQNameParts, needs runtime.SimpleValueNeed) (runtime.SimpleValue, error)
}
