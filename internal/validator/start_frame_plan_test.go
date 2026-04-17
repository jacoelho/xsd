package validator

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestPlanFrameClonesUncachedNameBytes(t *testing.T) {
	t.Parallel()

	local := []byte("local")
	ns := []byte("urn:test")
	plan, err := planStartFrame(
		startNameInput{Local: local, NS: ns},
		StartResult{Type: 7},
		runtime.Type{Kind: runtime.TypeSimple},
		nil,
	)
	if err != nil {
		t.Fatalf("planStartFrame() error = %v", err)
	}

	local[0] = 'L'
	ns[0] = 'U'
	if !bytes.Equal(plan.Local, []byte("local")) {
		t.Fatalf("planStartFrame() local = %q, want cloned bytes", plan.Local)
	}
	if !bytes.Equal(plan.NS, []byte("urn:test")) {
		t.Fatalf("planStartFrame() ns = %q, want cloned bytes", plan.NS)
	}
	if plan.Content != runtime.ContentSimple {
		t.Fatalf("planStartFrame() content = %v, want %v", plan.Content, runtime.ContentSimple)
	}
}

func TestPlanFrameComplexType(t *testing.T) {
	t.Parallel()

	plan, err := planStartFrame(
		startNameInput{Cached: true},
		StartResult{Type: 3},
		runtime.Type{Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 1}},
		[]runtime.ComplexType{
			{},
			{
				Content: runtime.ContentMixed,
				Mixed:   true,
				Model:   runtime.ModelRef{Kind: runtime.ModelDFA, ID: 9},
			},
		},
	)
	if err != nil {
		t.Fatalf("planStartFrame() error = %v", err)
	}
	if plan.Content != runtime.ContentMixed || !plan.Mixed || plan.Model.ID != 9 {
		t.Fatalf("planStartFrame() = %+v, want mixed model plan", plan)
	}
}

func TestPlanFrameMissingComplexType(t *testing.T) {
	t.Parallel()

	_, err := planStartFrame(
		startNameInput{Cached: true},
		StartResult{Type: 4},
		runtime.Type{Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 1}},
		nil,
	)
	if err == nil || err.Error() != "complex type 4 missing" {
		t.Fatalf("planStartFrame() error = %v, want missing complex type", err)
	}
}
