package runtime

import "testing"

func TestSimpleContentTypeRead(t *testing.T) {
	read := NewSimpleContentTypeRead(SimpleContentTypeReadShape{
		Type:    7,
		Present: true,
	})
	if !read.HasSimpleContent() || read.TypeID() != 7 {
		t.Fatalf("SimpleContentTypeRead = type %d present %v, want 7 true", read.TypeID(), read.HasSimpleContent())
	}

	absent := NewSimpleContentTypeRead(SimpleContentTypeReadShape{
		Type:    7,
		Present: false,
	})
	if absent.HasSimpleContent() || absent.TypeID() != NoSimpleType {
		t.Fatalf("absent SimpleContentTypeRead = type %d present %v, want no type false", absent.TypeID(), absent.HasSimpleContent())
	}
}

func TestSimpleContentTypeReadForComplexType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ct      ComplexType
		want    SimpleTypeID
		present bool
	}{
		{
			name: "simple",
			ct: ComplexType{
				TextType:    3,
				ContentKind: ContentSimple,
			},
			want:    3,
			present: true,
		},
		{
			name: "simple mixed",
			ct: ComplexType{
				TextType:    4,
				ContentKind: ContentSimpleMixed,
			},
			want:    4,
			present: true,
		},
		{
			name: "element only",
			ct: ComplexType{
				TextType:    5,
				ContentKind: ContentElementOnly,
			},
			want: NoSimpleType,
		},
		{
			name: "mixed",
			ct: ComplexType{
				TextType:    6,
				ContentKind: ContentMixed,
			},
			want: NoSimpleType,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			read := NewSimpleContentTypeRead(SimpleContentTypeReadShape{
				Type: tt.ct.TextType, Present: tt.ct.SimpleContent(),
			})
			if read.TypeID() != tt.want || read.HasSimpleContent() != tt.present {
				t.Fatalf("NewSimpleContentTypeRead() = type %d present %v, want %d %v",
					read.TypeID(), read.HasSimpleContent(), tt.want, tt.present)
			}
		})
	}
}
