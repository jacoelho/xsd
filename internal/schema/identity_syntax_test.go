package schema

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestCheckIdentityConstraintChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		children     []identityChildFixture
		wantSelector int
		wantFields   []int
		wantChild    int
		wantNested   int
		wantCode     xsderrors.Code
		wantMessage  string
	}{
		{
			name: "selector and field",
			children: []identityChildFixture{
				{local: selectorChild, xpath: LexicalAttribute{Value: "row", Present: true}},
				{local: fieldChild, xpath: LexicalAttribute{Value: "@id", Present: true}},
			},
			wantSelector: 0, wantFields: []int{1},
		},
		{
			name: "annotation selector and fields",
			children: []identityChildFixture{
				{local: annotationChild},
				{local: selectorChild, xpath: LexicalAttribute{Value: "row", Present: true}, children: []string{annotationChild}},
				{local: fieldChild, xpath: LexicalAttribute{Value: "@id", Present: true}},
				{local: fieldChild, xpath: LexicalAttribute{Value: "code", Present: true}},
			},
			wantSelector: 1, wantFields: []int{2, 3},
		},
		{
			name: "duplicate annotation",
			children: []identityChildFixture{
				{local: annotationChild},
				{local: annotationChild},
			},
			wantChild:   1,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "identity constraint can contain at most one annotation",
		},
		{
			name: "annotation after selector",
			children: []identityChildFixture{
				{local: selectorChild, xpath: LexicalAttribute{Value: "row", Present: true}},
				{local: annotationChild},
			},
			wantChild:   1,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "identity constraint annotation must be first",
		},
		{
			name: "duplicate selector",
			children: []identityChildFixture{
				{local: selectorChild, xpath: LexicalAttribute{Value: "row", Present: true}},
				{local: selectorChild, xpath: LexicalAttribute{Value: "row", Present: true}},
			},
			wantChild:   1,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "identity constraint can contain at most one selector",
		},
		{
			name: "field before selector",
			children: []identityChildFixture{
				{local: fieldChild, xpath: LexicalAttribute{Value: "@id", Present: true}},
			},
			wantChild:   0,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "identity constraint field requires selector",
		},
		{
			name: "invalid child",
			children: []identityChildFixture{
				{local: "element"},
			},
			wantChild:   0,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "invalid identity constraint child element",
		},
		{
			name: "missing selector",
			children: []identityChildFixture{
				{local: annotationChild},
			},
			wantChild:   -1,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaIdentity,
			wantMessage: "identity constraint missing selector",
		},
		{
			name: "missing fields",
			children: []identityChildFixture{
				{local: selectorChild, xpath: LexicalAttribute{Value: "row", Present: true}},
			},
			wantChild:   -1,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaIdentity,
			wantMessage: "identity constraint missing fields",
		},
		{
			name: "selector missing xpath",
			children: []identityChildFixture{
				{local: selectorChild},
			},
			wantChild:   0,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaIdentity,
			wantMessage: "selector missing xpath",
		},
		{
			name: "field empty xpath",
			children: []identityChildFixture{
				{local: selectorChild, xpath: LexicalAttribute{Value: "row", Present: true}},
				{local: fieldChild, xpath: LexicalAttribute{Present: true}},
			},
			wantChild:   1,
			wantNested:  -1,
			wantCode:    xsderrors.CodeSchemaIdentity,
			wantMessage: "field xpath is empty",
		},
		{
			name: "selector rejects non annotation child",
			children: []identityChildFixture{
				{local: selectorChild, xpath: LexicalAttribute{Value: "row", Present: true}, children: []string{"field"}},
			},
			wantChild:   0,
			wantNested:  0,
			wantCode:    xsderrors.CodeSchemaContentModel,
			wantMessage: "selector can contain only annotation",
		},
		{
			name: "field rejects duplicate annotation",
			children: []identityChildFixture{
				{local: selectorChild, xpath: LexicalAttribute{Value: "row", Present: true}},
				{local: fieldChild, xpath: LexicalAttribute{Value: "@id", Present: true}, children: []string{annotationChild, annotationChild}},
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

			parent := identityGrammarFixture(t, tt.children)
			children := slices.Collect(parent.xsdChildren())
			syntax, err := checkIdentityConstraintChildren(parent)
			if tt.wantMessage == "" {
				if err != nil {
					t.Fatalf("checkIdentityConstraintChildren() error = %v", err)
				}
				if syntax.selector != children[tt.wantSelector] || len(syntax.fields) != len(tt.wantFields) {
					t.Fatalf("selector or field count differs from expected selection")
				}
				for i, index := range tt.wantFields {
					if syntax.fields[i] != children[index] {
						t.Fatalf("field %d = %p, want child %d (%p)", i, syntax.fields[i], index, children[index])
					}
				}
				return
			}
			issue, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("checkIdentityConstraintChildren() error = %T %[1]v, want structured diagnostic", err)
			}
			target := parent
			if tt.wantChild >= 0 {
				target = children[tt.wantChild]
				if tt.wantNested >= 0 {
					target = parent.doc.node(target.children[tt.wantNested])
				}
			}
			if issue.Category() != xsderrors.CategorySchemaCompile || issue.Code() != tt.wantCode || issue.Message() != tt.wantMessage {
				t.Fatalf("diagnostic = {%s %s %q}, want {%s %s %q}", issue.Category(), issue.Code(), issue.Message(), xsderrors.CategorySchemaCompile, tt.wantCode, tt.wantMessage)
			}
			if issue.Path() != "identity.xsd" || issue.Line() != target.line || issue.Column() != target.column {
				t.Fatalf("location = {%q %d %d}, want {%q %d %d}", issue.Path(), issue.Line(), issue.Column(), "identity.xsd", target.line, target.column)
			}
		})
	}
}

type identityChildFixture struct {
	local    string
	xpath    LexicalAttribute
	children []string
}

func identityGrammarFixture(t *testing.T, children []identityChildFixture) *schemaNode {
	t.Helper()
	doc := &schemaDocument{name: "identity.xsd"}
	appendNode := func(parent *schemaNode, local string, line int) *schemaNode {
		id, err := schemaNodeIDFor(len(doc.arena))
		if err != nil {
			t.Fatal(err)
		}
		node := &schemaNode{doc: doc, id: id, local: local, line: line, column: 7}
		doc.arena = append(doc.arena, node)
		if parent != nil {
			parent.children = append(parent.children, node.id)
		}
		return node
	}
	parent := appendNode(nil, "key", 1)
	for i, child := range children {
		node := appendNode(parent, child.local, 100+i)
		if child.local == selectorChild || child.local == fieldChild {
			node.semantic.kind = &schemaIdentityXPathSource{XPath: child.xpath}
		}
		for j, local := range child.children {
			appendNode(node, local, 1000+i*100+j)
		}
	}
	return parent
}
