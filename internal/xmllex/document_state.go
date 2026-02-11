package xmllex

// DocumentState tracks document-boundary lexical state shared by XML token loops.
type DocumentState struct {
	allowBOM   bool
	rootSeen   bool
	rootClosed bool
}

// NewDocumentState returns an initialized document-boundary state.
func NewDocumentState() DocumentState {
	return DocumentState{allowBOM: true}
}

// RootSeen reports whether a root start element has been seen.
func (s *DocumentState) RootSeen() bool {
	return s != nil && s.rootSeen
}

// RootClosed reports whether the root element has been closed.
func (s *DocumentState) RootClosed() bool {
	return s != nil && s.rootClosed
}

// StartElementAllowed reports whether a start element may appear at this point.
func (s *DocumentState) StartElementAllowed() bool {
	return s == nil || !s.rootClosed
}

// OnStartElement advances state for a start-element token.
func (s *DocumentState) OnStartElement() {
	if s == nil {
		return
	}
	s.rootSeen = true
	s.allowBOM = false
}

// OnEndElement advances state for an end-element token.
// closeRoot should be true only when this token closes the document root element.
func (s *DocumentState) OnEndElement(closeRoot bool) {
	if s == nil {
		return
	}
	if closeRoot {
		s.rootClosed = true
	}
	s.allowBOM = false
}

// ValidateOutsideCharData reports whether character data outside root is ignorable.
func (s *DocumentState) ValidateOutsideCharData(data []byte) bool {
	if s == nil {
		return IsIgnorableOutsideRoot(data, true)
	}
	ok := IsIgnorableOutsideRoot(data, s.allowBOM)
	if ok {
		s.allowBOM = false
	}
	return ok
}

// OnOutsideMarkup advances state for comments/PI/directives outside root.
func (s *DocumentState) OnOutsideMarkup() {
	if s == nil {
		return
	}
	s.allowBOM = false
}
