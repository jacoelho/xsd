package compile

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateIdentityConstraintChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		children    []IdentityConstraintChild
		wantSyntax  IdentityConstraintSyntax
		wantChild   int
		wantNested  int
		wantCode    xsderrors.Code
		wantMessage string
	}{
		{
			name: "selector and field",
			children: []IdentityConstraintChild{
				{Local: selectorChild, HasXPath: true, XPath: "row"},
				{Local: fieldChild, HasXPath: true, XPath: "@id"},
			},
			wantSyntax: IdentityConstraintSyntax{Selector: 0, Fields: []int{1}},
		},
		{
			name: "annotation selector and fields",
			children: []IdentityConstraintChild{
				{Local: annotationChild},
				{Local: selectorChild, HasXPath: true, XPath: "row", Children: []string{annotationChild}},
				{Local: fieldChild, HasXPath: true, XPath: "@id"},
				{Local: fieldChild, HasXPath: true, XPath: "code"},
			},
			wantSyntax: IdentityConstraintSyntax{Selector: 1, Fields: []int{2, 3}},
		},
		{
			name: "duplicate annotation",
			children: []IdentityConstraintChild{
				{Local: annotationChild},
				{Local: annotationChild},
			},
			wantChild:   1,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "identity constraint can contain at most one annotation",
		},
		{
			name: "annotation after selector",
			children: []IdentityConstraintChild{
				{Local: selectorChild, HasXPath: true, XPath: "row"},
				{Local: annotationChild},
			},
			wantChild:   1,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "identity constraint annotation must be first",
		},
		{
			name: "duplicate selector",
			children: []IdentityConstraintChild{
				{Local: selectorChild, HasXPath: true, XPath: "row"},
				{Local: selectorChild, HasXPath: true, XPath: "row"},
			},
			wantChild:   1,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "identity constraint can contain at most one selector",
		},
		{
			name: "field before selector",
			children: []IdentityConstraintChild{
				{Local: fieldChild, HasXPath: true, XPath: "@id"},
			},
			wantChild:   0,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "identity constraint field requires selector",
		},
		{
			name: "invalid child",
			children: []IdentityConstraintChild{
				{Local: "element"},
			},
			wantChild:   0,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "invalid identity constraint child element",
		},
		{
			name: "missing selector",
			children: []IdentityConstraintChild{
				{Local: annotationChild},
			},
			wantChild:   -1,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaIdentity,
			wantMessage: "identity constraint missing selector",
		},
		{
			name: "missing fields",
			children: []IdentityConstraintChild{
				{Local: selectorChild, HasXPath: true, XPath: "row"},
			},
			wantChild:   -1,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaIdentity,
			wantMessage: "identity constraint missing fields",
		},
		{
			name: "selector missing xpath",
			children: []IdentityConstraintChild{
				{Local: selectorChild},
			},
			wantChild:   0,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaIdentity,
			wantMessage: "selector missing xpath",
		},
		{
			name: "field empty xpath",
			children: []IdentityConstraintChild{
				{Local: selectorChild, HasXPath: true, XPath: "row"},
				{Local: fieldChild, HasXPath: true},
			},
			wantChild:   1,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaIdentity,
			wantMessage: "field xpath is empty",
		},
		{
			name: "selector rejects non annotation child",
			children: []IdentityConstraintChild{
				{Local: selectorChild, HasXPath: true, XPath: "row", Children: []string{"field"}},
			},
			wantChild:   0,
			wantNested:  0,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "selector can contain only annotation",
		},
		{
			name: "field rejects duplicate annotation",
			children: []IdentityConstraintChild{
				{Local: selectorChild, HasXPath: true, XPath: "row"},
				{Local: fieldChild, HasXPath: true, XPath: "@id", Children: []string{annotationChild, annotationChild}},
			},
			wantChild:   1,
			wantNested:  1,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "field can contain at most one annotation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			syntax, err := ValidateIdentityConstraintChildren(tt.children)
			if tt.wantMessage == "" {
				if err != nil {
					t.Fatalf("ValidateIdentityConstraintChildren() error = %v", err)
				}
				if syntax.Selector != tt.wantSyntax.Selector || !slices.Equal(syntax.Fields, tt.wantSyntax.Fields) {
					t.Fatalf("syntax = {Selector:%d Fields:%v}, want {Selector:%d Fields:%v}", syntax.Selector, syntax.Fields, tt.wantSyntax.Selector, tt.wantSyntax.Fields)
				}
				return
			}
			issue, ok := errors.AsType[*IdentityConstraintSyntaxError](err)
			if !ok {
				t.Fatalf("ValidateIdentityConstraintChildren() error = %T %[1]v, want IdentityConstraintSyntaxError", err)
			}
			if issue.ChildIndex != tt.wantChild || issue.NestedChildIndex != tt.wantNested || issue.Code != tt.wantCode || issue.Message != tt.wantMessage {
				t.Fatalf("issue = {ChildIndex:%d NestedChildIndex:%d Code:%s Message:%q}, want {%d %d %s %q}", issue.ChildIndex, issue.NestedChildIndex, issue.Code, issue.Message, tt.wantChild, tt.wantNested, tt.wantCode, tt.wantMessage)
			}
		})
	}
}
