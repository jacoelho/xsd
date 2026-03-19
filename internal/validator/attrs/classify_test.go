package attrs

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
)

func TestTrackerClassifyCapturesClassesAndXSIValues(t *testing.T) {
	rt := &runtime.Schema{
		Predef: runtime.PredefinedSymbols{
			XsiType: 1,
			XsiNil:  2,
			XMLLang: 3,
		},
		PredefNS: runtime.PredefinedNamespaces{
			Xsi: 1,
			XML: 2,
		},
	}
	input := []Start{
		{Sym: rt.Predef.XsiType, NS: rt.PredefNS.Xsi, Local: []byte("type"), Value: []byte("t:Derived")},
		{NS: rt.PredefNS.Xsi, Local: []byte("unknown"), Value: []byte("1")},
		{Sym: rt.Predef.XMLLang, NS: rt.PredefNS.XML, Local: []byte("lang"), Value: []byte("en")},
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
	classified, err := tracker.Classify(&runtime.Schema{}, input, true)
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	code, ok := diag.Info(classified.DuplicateErr)
	if !ok || code != xsderrors.ErrXMLParse {
		t.Fatalf("Classify() duplicate code = %v, want %v", code, xsderrors.ErrXMLParse)
	}
}
