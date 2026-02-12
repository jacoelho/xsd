package validator

import "github.com/jacoelho/xsd/internal/runtime"

type rtIdentityFrame struct {
	captures []rtFieldCapture
	matches  []*rtSelectorMatch
	id       uint64
	depth    int
	sym      runtime.SymbolID
	ns       runtime.NamespaceID
	elem     runtime.ElemID
	typ      runtime.TypeID
	nilled   bool
}

type rtFieldNodeKind int

const (
	rtFieldNodeElement rtFieldNodeKind = iota
	rtFieldNodeAttribute
)

type identityAttrNameID uint32

type rtFieldNodeKey struct {
	kind       rtFieldNodeKind
	elemID     uint64
	attrSym    runtime.SymbolID
	attrNameID identityAttrNameID
}

type rtFieldCapture struct {
	match      *rtSelectorMatch
	fieldIndex int
}

type rtFieldState struct {
	nodes    map[rtFieldNodeKey]struct{}
	keyBytes []byte
	count    int
	keyKind  runtime.ValueKind
	multiple bool
	missing  bool
	invalid  bool
	hasValue bool
}

type rtSelectorMatch struct {
	constraint *rtConstraintState
	fields     []rtFieldState
	id         uint64
	depth      int
	invalid    bool
}

type rtConstraintState struct {
	matches    map[uint64]*rtSelectorMatch
	name       string
	selectors  []runtime.PathID
	fields     [][]runtime.PathID
	rows       []rtIdentityRow
	keyrefRows []rtIdentityRow
	violations []error
	id         runtime.ICID
	referenced runtime.ICID
	category   runtime.ICCategory
}

type rtIdentityRow struct {
	values []runtime.ValueKey
	hash   uint64
}

type rtIdentityScope struct {
	constraints []rtConstraintState
	rootID      uint64
	rootDepth   int
	rootElem    runtime.ElemID
}

type rtIdentityAttr struct {
	nsBytes  []byte
	local    []byte
	keyBytes []byte
	sym      runtime.SymbolID
	ns       runtime.NamespaceID
	keyKind  runtime.ValueKind
	nameID   identityAttrNameID
}
