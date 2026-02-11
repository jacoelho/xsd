package validator

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
)

func TestClassifyAttrsClasses(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)

	attrs := []StartAttr{
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
			Sym:     schema.Predef.XmlLang,
			NS:      schema.PredefNS.Xml,
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

	classified, err := sess.classifyAttrs(attrs, true)
	if err != nil {
		t.Fatalf("classifyAttrs: %v", err)
	}
	if classified.duplicateErr != nil {
		t.Fatalf("unexpected duplicate error: %v", classified.duplicateErr)
	}
	if len(classified.classes) != len(attrs) {
		t.Fatalf("classes length = %d, want %d", len(classified.classes), len(attrs))
	}
	if got := classified.classes[0]; got != attrClassXsiKnown {
		t.Fatalf("class[0] = %d, want %d", got, attrClassXsiKnown)
	}
	if got := classified.classes[1]; got != attrClassXsiUnknown {
		t.Fatalf("class[1] = %d, want %d", got, attrClassXsiUnknown)
	}
	if got := classified.classes[2]; got != attrClassXML {
		t.Fatalf("class[2] = %d, want %d", got, attrClassXML)
	}
	if got := classified.classes[3]; got != attrClassOther {
		t.Fatalf("class[3] = %d, want %d", got, attrClassOther)
	}
	if got := string(classified.xsiType); got != "t:Derived" {
		t.Fatalf("xsiType = %q, want %q", got, "t:Derived")
	}
	if len(classified.xsiNil) != 0 {
		t.Fatalf("xsiNil = %q, want empty", string(classified.xsiNil))
	}
}

func TestClassifyAttrsDuplicateAttributeSmallAndLarge(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)

	t.Run("small", func(t *testing.T) {
		attrs := []StartAttr{
			{Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")},
			{Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")},
		}
		classified, err := sess.classifyAttrs(attrs, true)
		if err != nil {
			t.Fatalf("classifyAttrs: %v", err)
		}
		if classified.duplicateErr == nil {
			t.Fatalf("expected duplicate attribute error")
		}
		code, _, ok := validationErrorInfo(classified.duplicateErr)
		if !ok || code != xsderrors.ErrXMLParse {
			t.Fatalf("duplicate error code = %v, want %v", code, xsderrors.ErrXMLParse)
		}
	})

	t.Run("large", func(t *testing.T) {
		attrs := make([]StartAttr, smallAttrDupThreshold+2)
		for i := range attrs {
			attrs[i] = StartAttr{
				NS:      ids.nsID,
				NSBytes: []byte("urn:test"),
				Local:   []byte{byte('a' + i)},
			}
		}
		attrs[len(attrs)-1].Local = attrs[0].Local

		classified, err := sess.classifyAttrs(attrs, true)
		if err != nil {
			t.Fatalf("classifyAttrs: %v", err)
		}
		if classified.duplicateErr == nil {
			t.Fatalf("expected duplicate attribute error")
		}
		code, _, ok := validationErrorInfo(classified.duplicateErr)
		if !ok || code != xsderrors.ErrXMLParse {
			t.Fatalf("duplicate error code = %v, want %v", code, xsderrors.ErrXMLParse)
		}
	})
}

func TestClassifyAttrsDuplicateXsiTypeAndNil(t *testing.T) {
	schema, _ := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)

	t.Run("xsiType", func(t *testing.T) {
		attrs := []StartAttr{
			{Sym: schema.Predef.XsiType, Value: []byte("t:Derived")},
			{
				NS:      schema.PredefNS.Xsi,
				NSBytes: []byte("http://www.w3.org/2001/XMLSchema-instance"),
				Local:   []byte("type"),
				Value:   []byte("t:Derived"),
			},
		}
		_, err := sess.classifyAttrs(attrs, true)
		if err == nil {
			t.Fatalf("expected duplicate xsi:type error")
		}
		code, _, ok := validationErrorInfo(err)
		if !ok || code != xsderrors.ErrDatatypeInvalid {
			t.Fatalf("error code = %v, want %v", code, xsderrors.ErrDatatypeInvalid)
		}
	})

	t.Run("xsiNil", func(t *testing.T) {
		attrs := []StartAttr{
			{Sym: schema.Predef.XsiNil, Value: []byte("true")},
			{
				NS:      schema.PredefNS.Xsi,
				NSBytes: []byte("http://www.w3.org/2001/XMLSchema-instance"),
				Local:   []byte("nil"),
				Value:   []byte("true"),
			},
		}
		_, err := sess.classifyAttrs(attrs, true)
		if err == nil {
			t.Fatalf("expected duplicate xsi:nil error")
		}
		code, _, ok := validationErrorInfo(err)
		if !ok || code != xsderrors.ErrDatatypeInvalid {
			t.Fatalf("error code = %v, want %v", code, xsderrors.ErrDatatypeInvalid)
		}
	})
}
