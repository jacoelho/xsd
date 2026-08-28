package validate

import (
	"encoding/xml"
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

var testSchemaLocationHintLimits = schemaLocationHintLimits{Namespaces: 32, NamespaceBytes: 1 << 16}

func TestSchemaLocationHintsRecordNamespacePairs(t *testing.T) {
	t.Parallel()

	var hints SchemaLocationHints
	if err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrSchemaLocation), "urn:a a.xsd\turn:b b.xsd", testSchemaLocationHintLimits, StartContext{Path: "/root", Line: 2, Column: 3}); err != nil {
		t.Fatalf("RecordAttribute() error = %v", err)
	}
	if !hints.Has("urn:a") || !hints.Has("urn:b") {
		t.Fatalf("RecordAttribute() did not retain schema-location namespaces")
	}
	if hints.Has("a.xsd") || hints.Has("b.xsd") {
		t.Fatalf("RecordAttribute() retained schema-location documents as namespaces")
	}
}

func TestSchemaLocationHintsRecordNoNamespace(t *testing.T) {
	t.Parallel()

	var hints SchemaLocationHints
	if err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrNoNamespaceSchemaLocation), "\tno-ns.xsd\n", testSchemaLocationHintLimits, StartContext{Path: "/root", Line: 2, Column: 3}); err != nil {
		t.Fatalf("RecordAttribute() error = %v", err)
	}
	if !hints.Has("") {
		t.Fatalf("RecordAttribute() did not retain no-namespace schema-location hint")
	}
}

func TestSchemaLocationHintsRecordEmptyNoNamespace(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", " \t\n\r"} {
		var hints SchemaLocationHints
		if err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrNoNamespaceSchemaLocation), value, testSchemaLocationHintLimits, StartContext{Path: "/root", Line: 2, Column: 3}); err != nil {
			t.Fatalf("RecordAttribute(%q) error = %v", value, err)
		}
		if !hints.Has("") {
			t.Fatalf("RecordAttribute(%q) did not retain no-namespace hint", value)
		}
	}
}

func TestSchemaLocationHintsRejectMalformedHints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		local   string
		value   string
		message string
	}{
		{
			name:    "schemaLocation odd field count",
			local:   vocab.XSIAttrSchemaLocation,
			value:   "urn:t",
			message: "xsi:schemaLocation must contain namespace/location pairs",
		},
		{
			name:    "schemaLocation invalid URI",
			local:   vocab.XSIAttrSchemaLocation,
			value:   "urn:t %zz",
			message: "invalid xsi:schemaLocation URI %zz",
		},
		{
			name:    "schemaLocation non XML whitespace",
			local:   vocab.XSIAttrSchemaLocation,
			value:   "urn:t\u00a0hinted.xsd",
			message: "xsi:schemaLocation must contain namespace/location pairs",
		},
		{
			name:    "noNamespace invalid URI",
			local:   vocab.XSIAttrNoNamespaceSchemaLocation,
			value:   "%zz",
			message: "invalid xsi:noNamespaceSchemaLocation URI %zz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var hints SchemaLocationHints
			err := hints.RecordAttribute(xsiHintName(tc.local), tc.value, testSchemaLocationHintLimits, StartContext{Path: "/root", Line: 2, Column: 3})
			expectXSDCode(t, err, xsderrors.CodeValidationAttribute)
			expectXSDMessage(t, err, tc.message)
		})
	}
}

func TestSchemaLocationHintsRejectMalformedAtomically(t *testing.T) {
	t.Parallel()

	var hints SchemaLocationHints
	err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrSchemaLocation), "urn:t %zz", testSchemaLocationHintLimits, StartContext{Path: "/root", Line: 2, Column: 3})
	expectXSDCode(t, err, xsderrors.CodeValidationAttribute)
	if hints.Has("urn:t") {
		t.Fatalf("RecordAttribute() retained namespace from malformed schema-location hint")
	}
}

func TestSchemaLocationHintsEnforceAtomicCountAndByteLimits(t *testing.T) {
	t.Parallel()

	ctx := StartContext{Path: "/root", Line: 2, Column: 3}
	t.Run("count", func(t *testing.T) {
		t.Parallel()
		var hints SchemaLocationHints
		limits := schemaLocationHintLimits{Namespaces: 2, NamespaceBytes: 64}
		if err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrSchemaLocation), "urn:a a.xsd urn:b b.xsd", limits, ctx); err != nil {
			t.Fatal(err)
		}
		err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrSchemaLocation), "urn:c c.xsd urn:d d.xsd", limits, ctx)
		expectXSDCode(t, err, xsderrors.CodeValidationLimit)
		if hints.Has("urn:c") || hints.Has("urn:d") {
			t.Fatal("over-limit attribute mutated hints")
		}
	})

	t.Run("UTF-8 byte count", func(t *testing.T) {
		t.Parallel()
		var hints SchemaLocationHints
		limits := schemaLocationHintLimits{Namespaces: 2, NamespaceBytes: 2}
		if err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrSchemaLocation), "é one.xsd", limits, ctx); err != nil {
			t.Fatal(err)
		}
		err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrSchemaLocation), "a two.xsd", limits, ctx)
		expectXSDCode(t, err, xsderrors.CodeValidationLimit)
		if hints.Has("a") {
			t.Fatal("byte-over-limit attribute mutated hints")
		}
	})
}

func TestSchemaLocationHintsDoNotChargeDuplicates(t *testing.T) {
	t.Parallel()

	var hints SchemaLocationHints
	limits := schemaLocationHintLimits{Namespaces: 1, NamespaceBytes: 5}
	ctx := StartContext{Path: "/root", Line: 2, Column: 3}
	for range 2 {
		if err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrSchemaLocation), "urn:a a.xsd urn:a b.xsd", limits, ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(hints.namespaces) != 1 || hints.namespaceBytes != 5 {
		t.Fatalf("duplicate accounting = %d namespaces, %d bytes", len(hints.namespaces), hints.namespaceBytes)
	}
}

func TestSchemaLocationHintSyntaxPrecedesLimits(t *testing.T) {
	t.Parallel()

	var hints SchemaLocationHints
	err := hints.RecordAttribute(
		xsiHintName(vocab.XSIAttrSchemaLocation),
		"urn:a a.xsd urn:b %zz",
		schemaLocationHintLimits{Namespaces: 1, NamespaceBytes: 1},
		StartContext{Path: "/root", Line: 2, Column: 3},
	)
	expectXSDCode(t, err, xsderrors.CodeValidationAttribute)
	if len(hints.namespaces) != 0 || hints.namespaceBytes != 0 {
		t.Fatal("malformed attribute mutated hints")
	}
}

func TestNoNamespaceSchemaLocationConsumesOneEntryAndNoBytes(t *testing.T) {
	t.Parallel()

	var hints SchemaLocationHints
	limits := schemaLocationHintLimits{Namespaces: 1, NamespaceBytes: 1}
	ctx := StartContext{Path: "/root", Line: 2, Column: 3}
	for range 2 {
		if err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrNoNamespaceSchemaLocation), "schema.xsd", limits, ctx); err != nil {
			t.Fatal(err)
		}
	}
	if !hints.Has("") || len(hints.namespaces) != 1 || hints.namespaceBytes != 0 {
		t.Fatalf("no-namespace accounting = %d namespaces, %d bytes", len(hints.namespaces), hints.namespaceBytes)
	}
}

func TestSchemaLocationHintsRecordAttributesFiltersHints(t *testing.T) {
	t.Parallel()

	var hints SchemaLocationHints
	err := hints.RecordAttributes(hintAttrs(
		hintStreamAttr("", "id", "ignored"),
		hintStreamAttr("urn:other", vocab.XSIAttrSchemaLocation, "ignored"),
		hintStreamAttr(vocab.XSINamespaceURI, vocab.XSIAttrSchemaLocation, "urn:a a.xsd"),
		hintStreamAttr(vocab.XSINamespaceURI, vocab.XSIAttrNoNamespaceSchemaLocation, "no-ns.xsd"),
	), nil, testSchemaLocationHintLimits, StartContext{Path: "/root", Line: 2, Column: 3})
	if err != nil {
		t.Fatalf("RecordAttributes() error = %v", err)
	}
	if !hints.Has("urn:a") || !hints.Has("") {
		t.Fatalf("RecordAttributes() did not retain expected schema-location hints")
	}
	if hints.Has("urn:other") {
		t.Fatalf("RecordAttributes() retained non-XSI schema-location-like attribute")
	}
}

func TestSchemaLocationHintsRecordAttributesIsAtomic(t *testing.T) {
	t.Parallel()

	hints := SchemaLocationHints{namespaces: map[string]struct{}{"urn:existing": {}}, namespaceBytes: 12}
	err := hints.RecordAttributes(hintAttrs(
		hintStreamAttr(vocab.XSINamespaceURI, vocab.XSIAttrSchemaLocation, "urn:new new.xsd"),
		hintStreamAttr(vocab.XSINamespaceURI, vocab.XSIAttrNoNamespaceSchemaLocation, "%zz"),
	), nil, testSchemaLocationHintLimits, StartContext{Path: "/root", Line: 2, Column: 3})
	expectXSDCode(t, err, xsderrors.CodeValidationAttribute)
	if !hints.Has("urn:existing") || hints.Has("urn:new") || hints.Has("") {
		t.Fatalf("failed RecordAttributes() changed hints: %+v", hints)
	}
}

func BenchmarkSchemaLocationHintsRecordDuplicateAttributes(b *testing.B) {
	for _, namespaceCount := range []int{1, 256} {
		b.Run(fmt.Sprintf("namespaces=%d", namespaceCount), func(b *testing.B) {
			namespaces := make(map[string]struct{}, namespaceCount)
			var namespaceBytes int64
			for i := range namespaceCount {
				ns := fmt.Sprintf("urn:%d", i)
				namespaces[ns] = struct{}{}
				namespaceBytes += int64(len(ns))
			}
			hints := SchemaLocationHints{namespaces: namespaces, namespaceBytes: namespaceBytes}
			attrs := hintAttrs(hintStreamAttr(vocab.XSINamespaceURI, vocab.XSIAttrSchemaLocation, "urn:0 schema.xsd"))
			limits := schemaLocationHintLimits{Namespaces: 256, NamespaceBytes: 1 << 20}
			ctx := StartContext{Path: "/root", Line: 1, Column: 1}
			b.ReportAllocs()
			for b.Loop() {
				if err := hints.RecordAttributes(attrs, nil, limits, ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestSchemaLocationHintsNoOpAttributesDoNotAllocate(t *testing.T) {
	namespaces := make(map[string]struct{}, 256)
	var namespaceBytes int64
	for i := range 256 {
		ns := fmt.Sprintf("urn:%d", i)
		namespaces[ns] = struct{}{}
		namespaceBytes += int64(len(ns))
	}
	limits := schemaLocationHintLimits{Namespaces: 256, NamespaceBytes: 1 << 20}
	ctx := StartContext{Path: "/root", Line: 1, Column: 1}
	test := func(t *testing.T, namespace, local, value string) {
		t.Helper()
		hints := SchemaLocationHints{namespaces: namespaces, namespaceBytes: namespaceBytes}
		allocs := testing.AllocsPerRun(100, func() {
			if err := recordHintAttributeForAllocationTest(&hints, namespace, local, value, limits, ctx); err != nil {
				panic(err)
			}
		})
		if allocs != 0 {
			t.Fatalf("RecordAttributes() allocations = %g, want 0", allocs)
		}
	}
	t.Run("no hint", func(t *testing.T) {
		test(t, "", "id", "ignored")
	})
	t.Run("duplicate hint", func(t *testing.T) {
		test(t, vocab.XSINamespaceURI, vocab.XSIAttrSchemaLocation, "urn:0 schema.xsd")
	})
}

func recordHintAttributeForAllocationTest(hints *SchemaLocationHints, namespace, local, value string, limits schemaLocationHintLimits, ctx StartContext) error {
	return hints.RecordAttributes(
		hintAttrs(hintStreamAttr(namespace, local, value)),
		nil,
		limits,
		ctx,
	)
}

func TestSchemaLocationHintsResetClearsAndDropsOversizedMaps(t *testing.T) {
	t.Parallel()

	hints := SchemaLocationHints{namespaces: map[string]struct{}{"urn:a": {}}, namespaceBytes: 5}
	hints.Reset(1)
	if hints.namespaces == nil {
		t.Fatalf("Reset() dropped bounded namespace map")
	}
	if hints.Has("urn:a") {
		t.Fatalf("Reset() retained stale namespace")
	}
	if hints.namespaceBytes != 0 {
		t.Fatal("Reset() retained namespace byte accounting")
	}

	hints.namespaces = map[string]struct{}{"urn:a": {}, "urn:b": {}}
	hints.Reset(1)
	if hints.namespaces != nil {
		t.Fatalf("Reset() retained oversized namespace map")
	}
}

func xsiHintName(local string) xml.Name {
	return xml.Name{Space: vocab.XSINamespaceURI, Local: local}
}

func hintStreamAttr(ns, local, value string) stream.Attr {
	return stream.OwnedAttr(xml.Name{Space: ns, Local: local}, value)
}

func hintAttrs(attrs ...stream.Attr) []stream.Attr {
	return stream.OwnedAttrs(attrs...)
}

func expectXSDMessage(t *testing.T, err error, message string) {
	t.Helper()
	var x *xsderrors.Error
	if !errors.As(err, &x) {
		t.Fatalf("error = %v, want *xsderrors.Error", err)
	}
	if x.Message() != message {
		t.Fatalf("error message = %q, want %q", x.Message(), message)
	}
}
