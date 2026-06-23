package runtime

import "testing"

func TestElementTextContentRead(t *testing.T) {
	t.Parallel()

	content := NewElementTextContent(ElementTextContentShape{
		Simple:  true,
		Complex: true,
		Mixed:   true,
		Fixed:   true,
	})
	if !content.HasSimpleContent() {
		t.Fatal("HasSimpleContent() = false, want true")
	}
	if !content.IsComplexType() {
		t.Fatal("IsComplexType() = false, want true")
	}
	if !content.AllowsMixedContent() {
		t.Fatal("AllowsMixedContent() = false, want true")
	}
	if !content.HasFixedElementValue() {
		t.Fatal("HasFixedElementValue() = false, want true")
	}

	var zero ElementTextContent
	if zero.HasSimpleContent() || zero.IsComplexType() || zero.AllowsMixedContent() || zero.HasFixedElementValue() {
		t.Fatalf("zero ElementTextContent = %+v, want no flags", zero)
	}
}

func TestElementTextContentForSimpleType(t *testing.T) {
	t.Parallel()

	read := NewElementTextContentForSimpleType()
	if !read.HasSimpleContent() || read.IsComplexType() ||
		read.AllowsMixedContent() || read.HasFixedElementValue() {
		t.Fatalf("NewElementTextContentForSimpleType() = simple %v complex %v mixed %v fixed %v, want simple=true complex=false mixed=false fixed=false",
			read.HasSimpleContent(), read.IsComplexType(), read.AllowsMixedContent(), read.HasFixedElementValue())
	}
	if !EqualElementTextContentForSimpleType(read) {
		t.Fatal("EqualElementTextContentForSimpleType() = false, want true")
	}

	tests := []struct {
		name string
		read ElementTextContent
	}{
		{
			name: "missing simple",
			read: NewElementTextContent(ElementTextContentShape{}),
		},
		{
			name: "complex",
			read: NewElementTextContent(ElementTextContentShape{
				Simple:  true,
				Complex: true,
			}),
		},
		{
			name: "mixed",
			read: NewElementTextContent(ElementTextContentShape{
				Simple: true,
				Mixed:  true,
			}),
		},
		{
			name: "fixed",
			read: NewElementTextContent(ElementTextContentShape{
				Simple: true,
				Fixed:  true,
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if EqualElementTextContentForSimpleType(tt.read) {
				t.Fatal("EqualElementTextContentForSimpleType() = true, want false")
			}
		})
	}
}

func TestElementTextContentForComplexType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ct       ComplexType
		fixed    bool
		simple   bool
		mixed    bool
		hasFixed bool
	}{
		{
			name: "element only",
			ct: ComplexType{
				ContentKind: ContentElementOnly,
			},
		},
		{
			name: "mixed",
			ct: ComplexType{
				ContentKind: ContentMixed,
			},
			mixed: true,
		},
		{
			name: "simple",
			ct: ComplexType{
				ContentKind: ContentSimple,
			},
			simple: true,
		},
		{
			name: "simple mixed",
			ct: ComplexType{
				ContentKind: ContentSimpleMixed,
			},
			simple: true,
			mixed:  true,
		},
		{
			name: "fixed mixed",
			ct: ComplexType{
				ContentKind: ContentMixed,
			},
			fixed:    true,
			mixed:    true,
			hasFixed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			read := NewElementTextContentForComplexType(tt.ct, tt.fixed)
			if !read.IsComplexType() || read.HasSimpleContent() != tt.simple ||
				read.AllowsMixedContent() != tt.mixed || read.HasFixedElementValue() != tt.hasFixed {
				t.Fatalf("NewElementTextContentForComplexType() = complex %v simple %v mixed %v fixed %v, want true %v %v %v",
					read.IsComplexType(), read.HasSimpleContent(), read.AllowsMixedContent(), read.HasFixedElementValue(),
					tt.simple, tt.mixed, tt.hasFixed)
			}
			if !EqualElementTextContentForComplexType(read, tt.ct, tt.fixed) {
				t.Fatal("EqualElementTextContentForComplexType() = false, want true")
			}
			if EqualElementTextContentForComplexType(read, tt.ct, !tt.fixed) {
				t.Fatal("EqualElementTextContentForComplexType() accepted wrong fixed flag")
			}
		})
	}
}

func TestElementTextContentProjection(t *testing.T) {
	t.Parallel()

	complexTypes := []ComplexType{
		{ContentKind: ContentElementOnly},
		{ContentKind: ContentMixed},
		{ContentKind: ContentSimple},
	}

	reads := NewElementTextContentsForComplexTypes(complexTypes, true)
	if !EqualElementTextContentProjection(reads, complexTypes, true) {
		t.Fatal("EqualElementTextContentProjection() rejected matching projection")
	}
	normalReads := NewElementTextContentsForComplexTypes(complexTypes, false)
	simpleRead := NewElementTextContentForSimpleType()
	elementValues := []ElementValueConstraints{
		NewElementValueConstraints(SimpleRef(1), ValueConstraintRead{}, true, ValueConstraintRead{}, false),
	}
	if got, ok := ElementTextContentByType(2, normalReads, reads, elementValues, simpleRead, SimpleRef(1), NoElement); !ok || !got.HasSimpleContent() || got.IsComplexType() {
		t.Fatalf("ElementTextContentByType(simple) = %+v, %v; want simple read, true", got, ok)
	}
	if got, ok := ElementTextContentByType(2, normalReads, reads, elementValues, simpleRead, ComplexRef(1), NoElement); !ok || !got.IsComplexType() || !got.AllowsMixedContent() || got.HasFixedElementValue() {
		t.Fatalf("ElementTextContentByType(complex mixed) = %+v, %v; want mixed non-fixed read, true", got, ok)
	}
	if got, ok := ElementTextContentByType(2, normalReads, reads, elementValues, simpleRead, ComplexRef(1), 0); !ok || !got.IsComplexType() || !got.AllowsMixedContent() || !got.HasFixedElementValue() {
		t.Fatalf("ElementTextContentByType(fixed element) = %+v, %v; want fixed mixed read, true", got, ok)
	}
	if got, ok := ElementTextContentByType(1, normalReads, reads, elementValues, simpleRead, SimpleRef(1), NoElement); ok || got != (ElementTextContent{}) {
		t.Fatalf("ElementTextContentByType(invalid simple) = %+v, %v; want zero, false", got, ok)
	}
	if got, ok := ElementTextContentByType(2, normalReads, reads, elementValues, simpleRead, ComplexRef(9), NoElement); ok || got != (ElementTextContent{}) {
		t.Fatalf("ElementTextContentByType(invalid complex) = %+v, %v; want zero, false", got, ok)
	}
	if got, ok := ElementTextContentByType(2, normalReads, reads, elementValues, simpleRead, ComplexRef(1), ElementID(99)); ok || got != (ElementTextContent{}) {
		t.Fatalf("ElementTextContentByType(invalid element) = %+v, %v; want zero, false", got, ok)
	}
	if got, ok := ElementTextContentByType(2, normalReads, reads[:1], elementValues, simpleRead, ComplexRef(1), 0); ok || got != (ElementTextContent{}) {
		t.Fatalf("ElementTextContentByType(invalid fixed table) = %+v, %v; want zero, false", got, ok)
	}
	if got, ok := ElementTextContentByType(2, normalReads, reads, elementValues, ElementTextContent{}, SimpleRef(1), NoElement); ok || got != (ElementTextContent{}) {
		t.Fatalf("ElementTextContentByType(invalid simple read) = %+v, %v; want zero, false", got, ok)
	}
	if has, ok := ElementHasSimpleContentByType(2, normalReads, reads, elementValues, simpleRead, SimpleRef(1), NoElement); !ok || !has {
		t.Fatalf("ElementHasSimpleContentByType(simple) = %v, %v; want true, true", has, ok)
	}
	if has, ok := ElementHasSimpleContentByType(2, normalReads, reads, elementValues, simpleRead, ComplexRef(0), NoElement); !ok || has {
		t.Fatalf("ElementHasSimpleContentByType(element-only) = %v, %v; want false, true", has, ok)
	}
	if has, ok := ElementHasSimpleContentByType(2, normalReads, reads, elementValues, simpleRead, ComplexRef(9), NoElement); ok || has {
		t.Fatalf("ElementHasSimpleContentByType(invalid) = %v, %v; want false, false", has, ok)
	}
	if EqualElementTextContentProjection(reads[:1], complexTypes, true) {
		t.Fatal("EqualElementTextContentProjection() accepted mismatched table length")
	}
	if EqualElementTextContentProjection(reads, complexTypes, false) {
		t.Fatal("EqualElementTextContentProjection() accepted wrong fixed flag")
	}

	changed := append([]ElementTextContent(nil), reads...)
	changed[1] = NewElementTextContent(ElementTextContentShape{Complex: true})
	if EqualElementTextContentProjection(changed, complexTypes, true) {
		t.Fatal("EqualElementTextContentProjection() accepted mismatched projection")
	}
}

func TestValidateElementTextContentProjection(t *testing.T) {
	t.Parallel()

	complexTypes := []ComplexType{
		{ContentKind: ContentElementOnly},
		{ContentKind: ContentSimple},
	}
	reads := NewElementTextContentsForComplexTypes(complexTypes, false)
	if err := ValidateElementTextContentProjection(reads, complexTypes, false); err != nil {
		t.Fatalf("ValidateElementTextContentProjection() error = %v", err)
	}
	if err := ValidateElementTextContentProjection(reads[:1], complexTypes, false); err == nil || err.Error() != "complex text content read projection count does not match types" {
		t.Fatalf("ValidateElementTextContentProjection(short) error = %v, want count invariant", err)
	}

	changed := append([]ElementTextContent(nil), reads...)
	changed[1] = NewElementTextContent(ElementTextContentShape{Complex: true})
	if err := ValidateElementTextContentProjection(changed, complexTypes, false); err == nil || err.Error() != "complex text content read projection does not match type" {
		t.Fatalf("ValidateElementTextContentProjection(changed) error = %v, want mismatch invariant", err)
	}

	fixedReads := NewElementTextContentsForComplexTypes(complexTypes, true)
	if err := ValidateElementTextContentProjection(fixedReads[:1], complexTypes, true); err == nil || err.Error() != "fixed complex text content read projection count does not match types" {
		t.Fatalf("ValidateElementTextContentProjection(fixed short) error = %v, want fixed count invariant", err)
	}
	if err := ValidateElementTextContentProjection(reads, complexTypes, true); err == nil || err.Error() != "fixed complex text content read projection does not match type" {
		t.Fatalf("ValidateElementTextContentProjection(fixed mismatch) error = %v, want fixed mismatch invariant", err)
	}
}

func TestValidateElementTextContentForSimpleType(t *testing.T) {
	t.Parallel()

	if err := ValidateElementTextContentForSimpleType(NewElementTextContentForSimpleType()); err != nil {
		t.Fatalf("ValidateElementTextContentForSimpleType() error = %v", err)
	}
	if err := ValidateElementTextContentForSimpleType(NewElementTextContent(ElementTextContentShape{Complex: true})); err == nil || err.Error() != "simple text content read projection does not match simple type" {
		t.Fatalf("ValidateElementTextContentForSimpleType(mismatch) error = %v, want mismatch invariant", err)
	}
}

func TestEqualElementTextContent(t *testing.T) {
	t.Parallel()

	content := NewElementTextContent(ElementTextContentShape{
		Simple:  true,
		Complex: true,
		Mixed:   true,
		Fixed:   true,
	})
	tests := []struct {
		name string
		a    ElementTextContent
		b    ElementTextContent
		want bool
	}{
		{
			name: "equal",
			a:    content,
			b: NewElementTextContent(ElementTextContentShape{
				Simple:  true,
				Complex: true,
				Mixed:   true,
				Fixed:   true,
			}),
			want: true,
		},
		{
			name: "simple differs",
			a:    content,
			b: NewElementTextContent(ElementTextContentShape{
				Complex: true,
				Mixed:   true,
				Fixed:   true,
			}),
		},
		{
			name: "complex differs",
			a:    content,
			b: NewElementTextContent(ElementTextContentShape{
				Simple: true,
				Mixed:  true,
				Fixed:  true,
			}),
		},
		{
			name: "mixed differs",
			a:    content,
			b: NewElementTextContent(ElementTextContentShape{
				Simple:  true,
				Complex: true,
				Fixed:   true,
			}),
		},
		{
			name: "fixed differs",
			a:    content,
			b: NewElementTextContent(ElementTextContentShape{
				Simple:  true,
				Complex: true,
				Mixed:   true,
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := EqualElementTextContent(tt.a, tt.b); got != tt.want {
				t.Fatalf("EqualElementTextContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElementChildContentRead(t *testing.T) {
	t.Parallel()

	content := NewElementChildContent(ElementChildContentShape{
		Complex: true,
		Simple:  true,
	})
	if !content.IsComplexType() {
		t.Fatal("IsComplexType() = false, want true")
	}
	if !content.HasSimpleContent() {
		t.Fatal("HasSimpleContent() = false, want true")
	}

	var zero ElementChildContent
	if zero.IsComplexType() || zero.HasSimpleContent() {
		t.Fatalf("zero ElementChildContent = %+v, want no flags", zero)
	}
}

func TestNewChildContentInfoForElementChildContent(t *testing.T) {
	t.Parallel()

	content := NewElementChildContent(ElementChildContentShape{
		Complex: true,
		Simple:  true,
	})
	info := NewChildContentInfoForElementChildContent(content)
	if !info.Complex || !info.Simple {
		t.Fatalf("NewChildContentInfoForElementChildContent() = %+v, want both flags", info)
	}

	var zero ElementChildContent
	info = NewChildContentInfoForElementChildContent(zero)
	if info.Complex || info.Simple {
		t.Fatalf("NewChildContentInfoForElementChildContent(zero) = %+v, want no flags", info)
	}
}

func TestElementChildContentForComplexType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		ct     ComplexType
		simple bool
	}{
		{
			name: "element only",
			ct: ComplexType{
				ContentKind: ContentElementOnly,
			},
		},
		{
			name: "mixed",
			ct: ComplexType{
				ContentKind: ContentMixed,
			},
		},
		{
			name: "simple",
			ct: ComplexType{
				ContentKind: ContentSimple,
			},
			simple: true,
		},
		{
			name: "simple mixed",
			ct: ComplexType{
				ContentKind: ContentSimpleMixed,
			},
			simple: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			read := NewElementChildContentForComplexType(tt.ct)
			if !read.IsComplexType() || read.HasSimpleContent() != tt.simple {
				t.Fatalf("NewElementChildContentForComplexType() = complex %v simple %v, want true %v",
					read.IsComplexType(), read.HasSimpleContent(), tt.simple)
			}
			if !EqualElementChildContentForComplexType(read, tt.ct) {
				t.Fatal("EqualElementChildContentForComplexType() = false, want true")
			}
			mismatch := tt.ct
			if tt.simple {
				mismatch.ContentKind = ContentElementOnly
			} else {
				mismatch.ContentKind = ContentSimple
			}
			if EqualElementChildContentForComplexType(read, mismatch) {
				t.Fatal("EqualElementChildContentForComplexType() accepted wrong content kind")
			}
		})
	}
}

func TestElementChildContentProjection(t *testing.T) {
	t.Parallel()

	complexTypes := []ComplexType{
		{ContentKind: ContentElementOnly},
		{ContentKind: ContentSimple},
		{ContentKind: ContentSimpleMixed},
	}

	reads := NewElementChildContentsForComplexTypes(complexTypes)
	if !EqualElementChildContentProjection(reads, complexTypes) {
		t.Fatal("EqualElementChildContentProjection() rejected matching projection")
	}
	if got, ok := ElementChildContentByType(2, reads, SimpleRef(1)); !ok || got != (ElementChildContent{}) {
		t.Fatalf("ElementChildContentByType(simple) = %+v, %v; want zero, true", got, ok)
	}
	if got, ok := ElementChildContentByType(2, reads, ComplexRef(1)); !ok || !got.IsComplexType() || !got.HasSimpleContent() {
		t.Fatalf("ElementChildContentByType(complex simple) = %+v, %v; want complex simple, true", got, ok)
	}
	if got, ok := ElementChildContentByType(1, reads, SimpleRef(1)); ok || got != (ElementChildContent{}) {
		t.Fatalf("ElementChildContentByType(invalid simple) = %+v, %v; want zero, false", got, ok)
	}
	if got, ok := ElementChildContentByType(2, reads, ComplexRef(9)); ok || got != (ElementChildContent{}) {
		t.Fatalf("ElementChildContentByType(invalid complex) = %+v, %v; want zero, false", got, ok)
	}
	if EqualElementChildContentProjection(reads[:1], complexTypes) {
		t.Fatal("EqualElementChildContentProjection() accepted mismatched table length")
	}

	changed := append([]ElementChildContent(nil), reads...)
	changed[1] = NewElementChildContent(ElementChildContentShape{Complex: true})
	if EqualElementChildContentProjection(changed, complexTypes) {
		t.Fatal("EqualElementChildContentProjection() accepted mismatched projection")
	}
}

func TestValidateElementChildContentProjection(t *testing.T) {
	t.Parallel()

	complexTypes := []ComplexType{
		{ContentKind: ContentElementOnly},
		{ContentKind: ContentSimple},
	}
	reads := NewElementChildContentsForComplexTypes(complexTypes)
	if err := ValidateElementChildContentProjection(reads, complexTypes); err != nil {
		t.Fatalf("ValidateElementChildContentProjection() error = %v", err)
	}
	if err := ValidateElementChildContentProjection(reads[:1], complexTypes); err == nil || err.Error() != "complex child content read projection count does not match types" {
		t.Fatalf("ValidateElementChildContentProjection(short) error = %v, want count invariant", err)
	}

	changed := append([]ElementChildContent(nil), reads...)
	changed[1] = NewElementChildContent(ElementChildContentShape{Complex: true})
	if err := ValidateElementChildContentProjection(changed, complexTypes); err == nil || err.Error() != "complex child content read projection does not match type" {
		t.Fatalf("ValidateElementChildContentProjection(changed) error = %v, want mismatch invariant", err)
	}
}

func TestEqualElementChildContent(t *testing.T) {
	t.Parallel()

	content := NewElementChildContent(ElementChildContentShape{Complex: true, Simple: true})
	tests := []struct {
		name string
		a    ElementChildContent
		b    ElementChildContent
		want bool
	}{
		{
			name: "equal",
			a:    content,
			b:    NewElementChildContent(ElementChildContentShape{Complex: true, Simple: true}),
			want: true,
		},
		{
			name: "complex differs",
			a:    content,
			b:    NewElementChildContent(ElementChildContentShape{Simple: true}),
		},
		{
			name: "simple differs",
			a:    content,
			b:    NewElementChildContent(ElementChildContentShape{Complex: true}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := EqualElementChildContent(tt.a, tt.b); got != tt.want {
				t.Fatalf("EqualElementChildContent() = %v, want %v", got, tt.want)
			}
		})
	}
}
