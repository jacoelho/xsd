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

	read := NewElementTextContent(ElementTextContentShape{Simple: true})
	if !read.HasSimpleContent() || read.IsComplexType() ||
		read.AllowsMixedContent() || read.HasFixedElementValue() {
		t.Fatalf("NewElementTextContent() = simple %v complex %v mixed %v fixed %v, want simple=true complex=false mixed=false fixed=false",
			read.HasSimpleContent(), read.IsComplexType(), read.AllowsMixedContent(), read.HasFixedElementValue())
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

			if tt.read == read {
				t.Fatal("simple text content accepted mismatched flags")
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

			read := NewElementTextContent(ElementTextContentShape{
				Simple:  tt.ct.SimpleContent(),
				Complex: true,
				Mixed:   tt.ct.Mixed(),
				Fixed:   tt.fixed,
			})
			if !read.IsComplexType() || read.HasSimpleContent() != tt.simple ||
				read.AllowsMixedContent() != tt.mixed || read.HasFixedElementValue() != tt.hasFixed {
				t.Fatalf("NewElementTextContent() = complex %v simple %v mixed %v fixed %v, want true %v %v %v",
					read.IsComplexType(), read.HasSimpleContent(), read.AllowsMixedContent(), read.HasFixedElementValue(),
					tt.simple, tt.mixed, tt.hasFixed)
			}
			if read == NewElementTextContent(ElementTextContentShape{
				Simple: tt.ct.SimpleContent(), Complex: true, Mixed: tt.ct.Mixed(), Fixed: !tt.fixed,
			}) {
				t.Fatal("text content accepted wrong fixed flag")
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

			read := NewElementChildContent(ElementChildContentShape{Complex: true, Simple: tt.ct.SimpleContent()})
			if !read.IsComplexType() || read.HasSimpleContent() != tt.simple {
				t.Fatalf("NewElementChildContent() = complex %v simple %v, want true %v",
					read.IsComplexType(), read.HasSimpleContent(), tt.simple)
			}
			mismatch := tt.ct
			if tt.simple {
				mismatch.ContentKind = ContentElementOnly
			} else {
				mismatch.ContentKind = ContentSimple
			}
			if read == NewElementChildContent(ElementChildContentShape{Complex: true, Simple: mismatch.SimpleContent()}) {
				t.Fatal("child content accepted wrong content kind")
			}
		})
	}
}
