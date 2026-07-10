package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/runtime"
)

// MaxRetainedBufferCapForTest exposes the retained byte-buffer cap to tests.
func MaxRetainedBufferCapForTest() int {
	return maxRetainedBufferCap
}

// MaxRetainedMapLenForTest exposes the retained map cap to benchmarks.
func MaxRetainedMapLenForTest() int {
	return maxRetainedMapLen
}

// MaxRetainedSliceCapForTest exposes the retained slice cap to benchmarks.
func MaxRetainedSliceCapForTest() int {
	return maxRetainedSliceCap
}

// SessionTextCapForTest returns the retained text buffer capacity.
func SessionTextCapForTest(s *Session) int {
	if s == nil {
		return 0
	}
	return cap(s.session.doc.text)
}

// IdentityRecorderForTest exposes identity recording hot-path helpers to benchmarks.
type IdentityRecorderForTest struct {
	session session
}

// NewIdentityRecorderForTest creates a benchmark identity recorder.
func NewIdentityRecorderForTest() *IdentityRecorderForTest {
	return &IdentityRecorderForTest{}
}

// PushPath appends a path segment.
func (r *IdentityRecorderForTest) PushPath(local string) {
	r.session.doc.CommitStart(xml.Name{Local: local}, local, false, frame{})
}

// PathString returns the current validation path.
func (r *IdentityRecorderForTest) PathString() string {
	return r.session.doc.PathString()
}

// ResetIdentity resets retained identity state.
func (r *IdentityRecorderForTest) ResetIdentity() {
	r.session.doc.identity.Reset(maxRetainedMapLen, maxRetainedSliceCap)
}

// RecordIdentityValue records one simple value identity payload.
func (r *IdentityRecorderForTest) RecordIdentityValue(value runtime.SimpleValue, line, col int) error {
	return r.session.recordIdentityValue(value, line, col)
}
