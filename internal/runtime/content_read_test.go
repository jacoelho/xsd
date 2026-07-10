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
			if !EqualElementTextContent(read, NewElementTextContentForComplexType(tt.ct, tt.fixed)) {
				t.Fatal("text content does not match complex type projection")
			}
			if EqualElementTextContent(read, NewElementTextContentForComplexType(tt.ct, !tt.fixed)) {
				t.Fatal("text content accepted wrong fixed flag")
			}
		})
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
			if read != NewElementChildContentForComplexType(tt.ct) {
				t.Fatal("child content does not match complex type projection")
			}
			mismatch := tt.ct
			if tt.simple {
				mismatch.ContentKind = ContentElementOnly
			} else {
				mismatch.ContentKind = ContentSimple
			}
			if read == NewElementChildContentForComplexType(mismatch) {
				t.Fatal("child content accepted wrong content kind")
			}
		})
	}
}
