package validator

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func TestClassifyAttrsClasses(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)

	inputAttrs := []Start{
		{
			Sym:     schema.Predef.XsiType,
			NS:      schema.PredefNS.Xsi,
			NSBytes: []byte("http://www.w3.org/2001/XMLSchema-instance"),
			Local:   []byte("type"),
			Value:   []byte("t:Derived"),
		},
		{
			NS:      schema.PredefNS.Xsi,
			NSBytes: []byte("http://www.w3.org/2001/XMLSchema-instance"),
			Local:   []byte("unknown"),
			Value:   []byte("1"),
		},
		{
			Sym:     schema.Predef.XMLLang,
			NS:      schema.PredefNS.XML,
			NSBytes: []byte("http://www.w3.org/XML/1998/namespace"),
			Local:   []byte("lang"),
			Value:   []byte("en"),
		},
		{
			Sym:     ids.attrSymDefault,
			NS:      ids.nsID,
			NSBytes: []byte("urn:test"),
			Local:   []byte("default"),
			Value:   []byte("ok"),
		},
	}

	classified, err := sess.classifyAttrs(inputAttrs, true)
	if err != nil {
		t.Fatalf("classifyAttrs: %v", err)
	}
	if classified.DuplicateErr != nil {
		t.Fatalf("unexpected duplicate error: %v", classified.DuplicateErr)
	}
	if len(classified.Classes) != len(inputAttrs) {
		t.Fatalf("classes length = %d, want %d", len(classified.Classes), len(inputAttrs))
	}
	if got := classified.Classes[0]; got != ClassXSIKnown {
		t.Fatalf("class[0] = %d, want %d", got, ClassXSIKnown)
	}
	if got := classified.Classes[1]; got != ClassXSIUnknown {
		t.Fatalf("class[1] = %d, want %d", got, ClassXSIUnknown)
	}
	if got := classified.Classes[2]; got != ClassXML {
		t.Fatalf("class[2] = %d, want %d", got, ClassXML)
	}
	if got := classified.Classes[3]; got != ClassOther {
		t.Fatalf("class[3] = %d, want %d", got, ClassOther)
	}
	if got := string(classified.XSIType); got != "t:Derived" {
		t.Fatalf("xsiType = %q, want %q", got, "t:Derived")
	}
	if len(classified.XSINil) != 0 {
		t.Fatalf("xsiNil = %q, want empty", string(classified.XSINil))
	}
}

func TestClassifyAttrsDuplicateAttributeSmallAndLarge(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)

	t.Run("small", func(t *testing.T) {
		inputAttrs := []Start{
			{Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")},
			{Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")},
		}
		classified, err := sess.classifyAttrs(inputAttrs, true)
		if err != nil {
			t.Fatalf("classifyAttrs: %v", err)
		}
		if classified.DuplicateErr == nil {
			t.Fatalf("expected duplicate attribute error")
		}
		code, ok := validationErrorInfo(classified.DuplicateErr)
		if !ok || code != xsderrors.ErrXMLParse {
			t.Fatalf("duplicate error code = %v, want %v", code, xsderrors.ErrXMLParse)
		}
	})

	t.Run("large", func(t *testing.T) {
		inputAttrs := make([]Start, SmallDuplicateThreshold+2)
		for i := range inputAttrs {
			inputAttrs[i] = Start{
				NS:      ids.nsID,
				NSBytes: []byte("urn:test"),
				Local:   []byte{byte('a' + i)},
			}
		}
		inputAttrs[len(inputAttrs)-1].Local = inputAttrs[0].Local

		classified, err := sess.classifyAttrs(inputAttrs, true)
		if err != nil {
			t.Fatalf("classifyAttrs: %v", err)
		}
		if classified.DuplicateErr == nil {
			t.Fatalf("expected duplicate attribute error")
		}
		code, ok := validationErrorInfo(classified.DuplicateErr)
		if !ok || code != xsderrors.ErrXMLParse {
			t.Fatalf("duplicate error code = %v, want %v", code, xsderrors.ErrXMLParse)
		}
	})
}

func TestClassifyAttrsDuplicateXsiTypeAndNil(t *testing.T) {
	schema, _ := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)

	t.Run("xsiType", func(t *testing.T) {
		inputAttrs := []Start{
			{Sym: schema.Predef.XsiType, Value: []byte("t:Derived")},
			{
				NS:      schema.PredefNS.Xsi,
				NSBytes: []byte("http://www.w3.org/2001/XMLSchema-instance"),
				Local:   []byte("type"),
				Value:   []byte("t:Derived"),
			},
		}
		_, err := sess.classifyAttrs(inputAttrs, true)
		if err == nil {
			t.Fatalf("expected duplicate xsi:type error")
		}
		code, ok := validationErrorInfo(err)
		if !ok || code != xsderrors.ErrDatatypeInvalid {
			t.Fatalf("error code = %v, want %v", code, xsderrors.ErrDatatypeInvalid)
		}
	})

	t.Run("xsiNil", func(t *testing.T) {
		inputAttrs := []Start{
			{Sym: schema.Predef.XsiNil, Value: []byte("true")},
			{
				NS:      schema.PredefNS.Xsi,
				NSBytes: []byte("http://www.w3.org/2001/XMLSchema-instance"),
				Local:   []byte("nil"),
				Value:   []byte("true"),
			},
		}
		_, err := sess.classifyAttrs(inputAttrs, true)
		if err == nil {
			t.Fatalf("expected duplicate xsi:nil error")
		}
		code, ok := validationErrorInfo(err)
		if !ok || code != xsderrors.ErrDatatypeInvalid {
			t.Fatalf("error code = %v, want %v", code, xsderrors.ErrDatatypeInvalid)
		}
	})
}
