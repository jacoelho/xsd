package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func TestTrackerClassifyCapturesClassesAndXSIValues(t *testing.T) {
	rt := newRuntimeSchema(t)
	setRuntimePredefinedSymbols(t, rt, runtime.PredefinedSymbols{
		XsiType: 1,
		XsiNil:  2,
		XMLLang: 3,
	})
	setRuntimePredefinedNamespaces(t, rt, runtime.PredefinedNamespaces{
		Xsi: 1,
		XML: 2,
	})
	input := []Start{
		{Sym: rt.KnownSymbols().XsiType, NS: rt.KnownNamespaces().Xsi, Local: []byte("type"), Value: []byte("t:Derived")},
		{NS: rt.KnownNamespaces().Xsi, Local: []byte("unknown"), Value: []byte("1")},
		{Sym: rt.KnownSymbols().XMLLang, NS: rt.KnownNamespaces().XML, Local: []byte("lang"), Value: []byte("en")},
		{NSBytes: []byte("urn:test"), Local: []byte("default"), Value: []byte("ok")},
	}

	var tracker Tracker
	classified, err := tracker.Classify(rt, input, true)
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if classified.DuplicateErr != nil {
		t.Fatalf("Classify() duplicate error = %v, want nil", classified.DuplicateErr)
	}
	want := []Class{ClassXSIKnown, ClassXSIUnknown, ClassXML, ClassOther}
	for i, got := range classified.Classes {
		if got != want[i] {
			t.Fatalf("Classify() class[%d] = %v, want %v", i, got, want[i])
		}
	}
	if got := string(classified.XSIType); got != "t:Derived" {
		t.Fatalf("Classify() XSIType = %q, want %q", got, "t:Derived")
	}
	if len(classified.XSINil) != 0 {
		t.Fatalf("Classify() XSINil = %q, want empty", string(classified.XSINil))
	}
}

func TestTrackerClassifyReportsDuplicateAttribute(t *testing.T) {
	input := []Start{
		{NSBytes: []byte("urn:test"), Local: []byte("dup")},
		{NSBytes: []byte("urn:test"), Local: []byte("dup")},
	}

	var tracker Tracker
	classified, err := tracker.Classify(newRuntimeSchema(t), input, true)
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	code, ok := xsderrors.Info(classified.DuplicateErr)
	if !ok || code != xsderrors.ErrXMLParse {
		t.Fatalf("Classify() duplicate code = %v, want %v", code, xsderrors.ErrXMLParse)
	}
}
