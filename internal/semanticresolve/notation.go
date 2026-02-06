package semanticresolve

import "github.com/jacoelho/xsd/internal/types"

func isDirectNotationType(typ types.Type) bool {
	if typ == nil || !typ.IsBuiltin() {
		return false
	}
	name := typ.Name()
	return name.Namespace == types.XSDNamespace && name.Local == string(types.TypeNameNOTATION)
}
