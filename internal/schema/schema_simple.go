package schema

import valuepkg "github.com/jacoelho/xsd/internal/value"

// SimpleTypeIdentity returns compiler-owned identity metadata.
func (rt *schemaBuild) SimpleTypeIdentity(id SimpleTypeID) (valuepkg.IdentityKind, bool) {
	st, ok := SimpleTypeByID(rt.SimpleTypes, id)
	if !ok {
		return valuepkg.IdentityNone, false
	}
	if rt.valueBuilder != nil {
		if requirements, complete := rt.valueBuilder.InputRequirements(id); complete {
			return requirements.Identity, true
		}
	}
	return st.ValueSpec.Identity, true
}
