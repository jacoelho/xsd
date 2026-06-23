package runtime

import (
	"strings"
	"testing"
)

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

			read := NewSimpleContentTypeReadForComplexType(tt.ct)
			if read.TypeID() != tt.want || read.HasSimpleContent() != tt.present {
				t.Fatalf("NewSimpleContentTypeReadForComplexType() = type %d present %v, want %d %v",
					read.TypeID(), read.HasSimpleContent(), tt.want, tt.present)
			}
		})
	}
}

func TestSimpleContentTypeReadProjectionTable(t *testing.T) {
	t.Parallel()

	complexTypes := []ComplexType{
		{TextType: 1, ContentKind: ContentSimple},
		{TextType: 2, ContentKind: ContentElementOnly},
		{TextType: 3, ContentKind: ContentSimpleMixed},
	}

	reads := NewSimpleContentTypeReadsForComplexTypes(complexTypes)
	if got, has, ok := SimpleContentTypeByType(4, reads, SimpleRef(2)); !ok || !has || got != 2 {
		t.Fatalf("SimpleContentTypeByType(simple) = %v, %v, %v; want 2, true, true", got, has, ok)
	}
	if got, has, ok := SimpleContentTypeByType(4, reads, ComplexRef(0)); !ok || !has || got != 1 {
		t.Fatalf("SimpleContentTypeByType(complex simple) = %v, %v, %v; want 1, true, true", got, has, ok)
	}
	if got, has, ok := SimpleContentTypeByType(4, reads, ComplexRef(1)); !ok || has || got != NoSimpleType {
		t.Fatalf("SimpleContentTypeByType(element-only) = %v, %v, %v; want no type, false, true", got, has, ok)
	}
	if got, has, ok := SimpleContentTypeByType(1, reads, SimpleRef(2)); ok || has || got != NoSimpleType {
		t.Fatalf("SimpleContentTypeByType(invalid simple) = %v, %v, %v; want no type, false, false", got, has, ok)
	}
	if got, has, ok := SimpleContentTypeByType(4, reads, ComplexRef(9)); ok || has || got != NoSimpleType {
		t.Fatalf("SimpleContentTypeByType(invalid complex) = %v, %v, %v; want no type, false, false", got, has, ok)
	}
	badReads := append([]SimpleContentTypeRead(nil), reads...)
	badReads[0] = NewSimpleContentTypeRead(SimpleContentTypeReadShape{Type: 9, Present: true})
	if got, has, ok := SimpleContentTypeByType(4, badReads, ComplexRef(0)); ok || has || got != NoSimpleType {
		t.Fatalf("SimpleContentTypeByType(invalid text type) = %v, %v, %v; want no type, false, false", got, has, ok)
	}
	if err := ValidateSimpleContentTypeReadProjectionTable(reads, complexTypes, 4); err != nil {
		t.Fatalf("ValidateSimpleContentTypeReadProjectionTable() error = %v", err)
	}
	if err := ValidateSimpleContentTypeReadProjectionTable(reads[:1], complexTypes, 4); err == nil ||
		!strings.Contains(err.Error(), "count does not match types") {
		t.Fatalf("ValidateSimpleContentTypeReadProjectionTable() error = %v, want count mismatch", err)
	}

	changed := append([]SimpleContentTypeRead(nil), reads...)
	changed[2] = NewSimpleContentTypeRead(SimpleContentTypeReadShape{Type: 1, Present: true})
	if err := ValidateSimpleContentTypeReadProjectionTable(changed, complexTypes, 4); err == nil ||
		!strings.Contains(err.Error(), "does not match complex type") {
		t.Fatalf("ValidateSimpleContentTypeReadProjectionTable() error = %v, want projection mismatch", err)
	}
}

func TestValidateSimpleContentTypeReadProjection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		read    SimpleContentTypeRead
		shape   SimpleContentTypeReadShape
		count   int
		wantErr string
	}{
		{
			name:  "present",
			read:  NewSimpleContentTypeRead(SimpleContentTypeReadShape{Type: 1, Present: true}),
			shape: SimpleContentTypeReadShape{Type: 1, Present: true},
			count: 2,
		},
		{
			name:  "absent",
			read:  NewSimpleContentTypeRead(SimpleContentTypeReadShape{Type: 1}),
			shape: SimpleContentTypeReadShape{Type: 1},
			count: 2,
		},
		{
			name:    "invalid present text type",
			read:    NewSimpleContentTypeRead(SimpleContentTypeReadShape{Type: 9, Present: true}),
			shape:   SimpleContentTypeReadShape{Type: 9, Present: true},
			count:   2,
			wantErr: "references invalid text type",
		},
		{
			name:    "present mismatch",
			read:    SimpleContentTypeRead{},
			shape:   SimpleContentTypeReadShape{Type: 1, Present: true},
			count:   2,
			wantErr: "does not match complex type",
		},
		{
			name:    "text type mismatch",
			read:    NewSimpleContentTypeRead(SimpleContentTypeReadShape{Type: 0, Present: true}),
			shape:   SimpleContentTypeReadShape{Type: 1, Present: true},
			count:   2,
			wantErr: "does not match complex type",
		},
		{
			name: "absent read stores text type",
			read: SimpleContentTypeRead{
				typ: 1,
			},
			shape:   SimpleContentTypeReadShape{Type: 1},
			count:   2,
			wantErr: "stores text type for non-simple content",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleContentTypeReadProjection(tt.read, tt.shape, tt.count)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleContentTypeReadProjection() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateSimpleContentTypeReadProjection() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSimpleContentTypeReadForComplexType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		read    SimpleContentTypeRead
		ct      ComplexType
		count   int
		wantErr string
	}{
		{
			name:  "present",
			read:  NewSimpleContentTypeReadForComplexType(ComplexType{TextType: 1, ContentKind: ContentSimple}),
			ct:    ComplexType{TextType: 1, ContentKind: ContentSimple},
			count: 2,
		},
		{
			name:  "absent",
			read:  NewSimpleContentTypeReadForComplexType(ComplexType{TextType: 1, ContentKind: ContentMixed}),
			ct:    ComplexType{TextType: 1, ContentKind: ContentMixed},
			count: 2,
		},
		{
			name:    "invalid present text type",
			read:    NewSimpleContentTypeReadForComplexType(ComplexType{TextType: 9, ContentKind: ContentSimple}),
			ct:      ComplexType{TextType: 9, ContentKind: ContentSimple},
			count:   2,
			wantErr: "references invalid text type",
		},
		{
			name:    "present mismatch",
			read:    SimpleContentTypeRead{},
			ct:      ComplexType{TextType: 1, ContentKind: ContentSimple},
			count:   2,
			wantErr: "does not match complex type",
		},
		{
			name:    "text type mismatch",
			read:    NewSimpleContentTypeReadForComplexType(ComplexType{TextType: 0, ContentKind: ContentSimple}),
			ct:      ComplexType{TextType: 1, ContentKind: ContentSimple},
			count:   2,
			wantErr: "does not match complex type",
		},
		{
			name: "absent read stores text type",
			read: SimpleContentTypeRead{
				typ: 1,
			},
			ct:      ComplexType{TextType: 1, ContentKind: ContentElementOnly},
			count:   2,
			wantErr: "stores text type for non-simple content",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleContentTypeReadForComplexType(tt.read, tt.ct, tt.count)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleContentTypeReadForComplexType() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateSimpleContentTypeReadForComplexType() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
