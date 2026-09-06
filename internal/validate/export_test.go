package validate

import (
	"encoding/xml"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/value"
)

func newSessionForTest(rt *xsdSchema.Schema, opts Options) (*Session, error) {
	s, err := NewSession(rt, opts)
	if err != nil {
		return nil, err
	}
	return s, nil
}

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
	recorder := &IdentityRecorderForTest{}
	recorder.session.doc.identity.limits = identityLimits{
		Entries:    defaultMaxIdentityEntries,
		TupleBytes: defaultMaxIdentityTupleBytes,
	}
	return recorder
}

// PushPath appends a path segment and its active identity frame.
func (r *IdentityRecorderForTest) PushPath(local string) {
	start := preparedXMLStart{name: xml.Name{Local: local}}
	r.session.doc.CommitStart(start, frame{})
	if err := r.session.doc.identity.startElement(identityElementStart{
		Context: r.session.startContext(1, 1),
		Name:    xsdSchema.RuntimeName{Local: local},
		Element: xsdSchema.NoElement,
		Mode:    elementAssessed,
	}); err != nil {
		panic(err)
	}
}

// PathString returns the current validation path.
func (r *IdentityRecorderForTest) PathString() string {
	return r.session.doc.PathString()
}

// ResetIdentity resets retained identity state.
func (r *IdentityRecorderForTest) ResetIdentity() {
	// Keep the active document fixture in place while resetting only the
	// retained identity values measured by the benchmark.
	r.session.doc.identity.identityState.reset(maxRetainedMapLen, maxRetainedSliceCap)
}

// RecordIdentityValue records one simple value identity payload.
func (r *IdentityRecorderForTest) RecordIdentityValue(v value.Value, line, col int) error {
	return r.session.doc.identity.recordIdentityFields(v.IDs(), v.IDRefs(), r.session.startContext(line, col))
}
