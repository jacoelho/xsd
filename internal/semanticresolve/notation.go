package semanticresolve

import model "github.com/jacoelho/xsd/internal/types"

func isDirectNotationType(typ model.Type) bool {
	if typ == nil || !typ.IsBuiltin() {
		return false
	}
	name := typ.Name()
	return name.Namespace == model.XSDNamespace && name.Local == string(model.TypeNameNOTATION)
}
