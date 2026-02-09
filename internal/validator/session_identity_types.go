package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/state"
)

type identityState struct {
	arena  *Arena
	frames state.StateStack[rtIdentityFrame]
	scopes state.StateStack[rtIdentityScope]
	// uncommittedViolations are immediate identity processing failures tied to the active frame.
	uncommittedViolations []error
	// committedViolations are scope-finalized violations emitted on the next event boundary.
	committedViolations []error
	nextNodeID          uint64
	active              bool
}

type identitySnapshot struct {
	nextNodeID  uint64
	framesLen   int
	scopesLen   int
	uncommitted int
	committed   int
	active      bool
}

type identityStartInput struct {
	Attrs   []StartAttr
	Applied []AttrApplied
	Elem    runtime.ElemID
	Type    runtime.TypeID
	Sym     runtime.SymbolID
	NS      runtime.NamespaceID
	Nilled  bool
}

type identityEndInput struct {
	Text      []byte
	KeyBytes  []byte
	TextState TextState
	KeyKind   runtime.ValueKind
}

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

func (s *rtFieldState) addNode(key rtFieldNodeKey) bool {
	if s.nodes == nil {
		s.nodes = make(map[rtFieldNodeKey]struct{})
	}
	if _, ok := s.nodes[key]; ok {
		return false
	}
	s.nodes[key] = struct{}{}
	s.count++
	if s.count > 1 {
		s.multiple = true
	}
	return true
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

func (s *Session) identityStart(in identityStartInput) error {
	if s == nil {
		return nil
	}
	snapshot := s.icState.checkpoint()
	err := s.icState.start(s, in)
	if err != nil {
		s.icState.rollback(snapshot)
	}
	return err
}
