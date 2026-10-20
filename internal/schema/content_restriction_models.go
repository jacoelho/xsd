package schema

// restrictionModelView borrows semantic input and owns append-only comparison
// nodes. Readers borrow its slices without mutation. Temporary IDs never enter
// the build or its analysis caches.
type restrictionModelView struct {
	original []ContentModel
	added    []ContentModel
}

func (m *restrictionModelView) model(id ContentModelID) (ContentModel, bool) {
	if uint64(id) < uint64(len(m.original)) {
		return m.original[id], true
	}
	index := uint64(id) - uint64(len(m.original))
	if index >= uint64(len(m.added)) {
		return ContentModel{}, false
	}
	return m.added[index], true
}
