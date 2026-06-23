package compile

import (
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestParseWildcard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		attrs      WildcardAttrs
		wantMode   runtime.WildcardMode
		wantNS     []string
		wantOther  string
		wantProc   runtime.ProcessContents
		wantCode   xsderrors.Code
		wantNoName bool
	}{
		{
			name:       "defaults to any strict",
			wantMode:   runtime.WildcardAny,
			wantProc:   runtime.ProcessStrict,
			wantNoName: true,
		},
		{
			name: "other uses target namespace",
			attrs: WildcardAttrs{
				Namespace:       "##other",
				TargetNamespace: "urn:t",
				HasNamespace:    true,
			},
			wantMode:  runtime.WildcardOther,
			wantOther: "urn:t",
			wantProc:  runtime.ProcessStrict,
		},
		{
			name: "local",
			attrs: WildcardAttrs{
				Namespace:    "##local",
				HasNamespace: true,
			},
			wantMode:   runtime.WildcardLocal,
			wantProc:   runtime.ProcessStrict,
			wantNoName: true,
		},
		{
			name: "target namespace",
			attrs: WildcardAttrs{
				Namespace:       "##targetNamespace",
				TargetNamespace: "urn:t",
				HasNamespace:    true,
			},
			wantMode: runtime.WildcardTargetNamespace,
			wantNS:   []string{"urn:t"},
			wantProc: runtime.ProcessStrict,
		},
		{
			name: "list normalizes XML whitespace and duplicates",
			attrs: WildcardAttrs{
				Namespace:          "urn:b\t##local\nurn:a\rurn:b ##targetNamespace",
				ProcessContents:    "lax",
				TargetNamespace:    "urn:t",
				HasNamespace:       true,
				HasProcessContents: true,
			},
			wantMode: runtime.WildcardList,
			wantNS:   []string{"", "urn:b", "urn:a", "urn:t"},
			wantProc: runtime.ProcessLax,
		},
		{
			name: "invalid reserved namespace token",
			attrs: WildcardAttrs{
				Namespace:    "urn:a ##bogus",
				HasNamespace: true,
			},
			wantCode: xsderrors.CodeSchemaInvalidAttribute,
		},
		{
			name: "invalid process contents",
			attrs: WildcardAttrs{
				ProcessContents:    "open",
				HasProcessContents: true,
			},
			wantCode:   xsderrors.CodeSchemaInvalidAttribute,
			wantNoName: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			names := newWildcardNamespaceInterner()
			var interner NamespaceInterner = names
			if tt.wantNoName {
				interner = nil
			}
			got, err := ParseWildcard(interner, tt.attrs)
			if tt.wantCode != "" {
				expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, tt.wantCode)
				return
			}
			if err != nil {
				t.Fatalf("ParseWildcard() error = %v", err)
			}
			if got.Mode != tt.wantMode || got.Process != tt.wantProc {
				t.Fatalf("ParseWildcard() = %#v, want mode %d process %d", got, tt.wantMode, tt.wantProc)
			}
			if tt.wantOther != "" && got.OtherThan != names.mustID(tt.wantOther) {
				t.Fatalf("OtherThan = %d, want namespace %q", got.OtherThan, tt.wantOther)
			}
			if tt.wantNS != nil {
				var want []runtime.NamespaceID
				for _, uri := range tt.wantNS {
					want = append(want, names.mustID(uri))
				}
				if !slices.Equal(got.Namespaces, want) {
					t.Fatalf("Namespaces = %v, want %v", got.Namespaces, want)
				}
			}
		})
	}
}

func TestParseWildcardRequiresNamespaceInternerWhenInterning(t *testing.T) {
	t.Parallel()

	_, err := ParseWildcard(nil, WildcardAttrs{Namespace: "##other", HasNamespace: true})
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

type wildcardNamespaceInterner struct {
	ids map[string]runtime.NamespaceID
}

func newWildcardNamespaceInterner() *wildcardNamespaceInterner {
	return &wildcardNamespaceInterner{
		ids: map[string]runtime.NamespaceID{"": runtime.EmptyNamespaceID},
	}
}

func (n *wildcardNamespaceInterner) InternNamespace(uri string) (runtime.NamespaceID, error) {
	if id, ok := n.ids[uri]; ok {
		return id, nil
	}
	next, err := checkedUint32(len(n.ids), "namespace limit exceeded")
	if err != nil {
		return 0, err
	}
	id := runtime.NamespaceID(next)
	n.ids[uri] = id
	return id, nil
}

func (n *wildcardNamespaceInterner) mustID(uri string) runtime.NamespaceID {
	id, ok := n.ids[uri]
	if !ok {
		panic("namespace was not interned: " + uri)
	}
	return id
}
