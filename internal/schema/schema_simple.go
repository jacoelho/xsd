package schema

// SimpleTypeCarriesID reports the value program's single ID-ness predicate
// (I-016) for a completed simple type.
func (rt *schemaBuild) SimpleTypeCarriesID(id SimpleTypeID) (carries, ok bool) {
	if _, found := SimpleTypeByID(rt.SimpleTypes, id); !found || rt.valueBuilder == nil {
		return false, false
	}
	requirements, ok := rt.valueBuilder.InputRequirements(id)
	return requirements.CarriesID, ok
}
