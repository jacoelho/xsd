package schema

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestCheckOrderedXSDChildren(t *testing.T) {
	t.Parallel()

	order := ChildOrder{
		AnnotationFirstMsg: "annotation must be first",
		SingleAnnotation:   true,
		Rules: []ChildRule{
			{
				Match:    matchTestChildren("head"),
				Level:    0,
				MaxOne:   true,
				OrderMsg: "head must precede body",
				DupMsg:   "head duplicates",
			},
			{
				Match:    matchTestChildren("body"),
				Level:    1,
				OrderMsg: "body is out of order",
			},
			{
				Match:        matchTestChildren("tail"),
				Level:        2,
				Terminal:     true,
				ForbiddenMsg: "tail is forbidden",
			},
		},
		InvalidMsg: func(local string) string { return "invalid " + local },
	}
	tests := []struct {
		name     string
		children []string
		wantCode xsderrors.Code
		wantLine int
		wantCol  int
		wantMsg  string
	}{
		{name: "valid", children: []string{"annotation", "head", "body"}},
		{name: "second annotation", children: []string{"annotation", "annotation"}, wantMsg: "annotation must be first", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "annotation after child", children: []string{"head", "annotation"}, wantMsg: "annotation must be first", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "unknown child", children: []string{"bogus"}, wantMsg: "invalid bogus", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
		{name: "out of order", children: []string{"body", "head"}, wantMsg: "head must precede body", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "duplicate max one", children: []string{"head", "head"}, wantMsg: "head duplicates", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "forbidden", children: []string{"tail"}, wantMsg: "tail is forbidden", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
		{name: "terminal", children: []string{"head", "tail", "body"}, wantMsg: "tail is forbidden", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "component", tt.children)
			err := checkOrderedXSDChildren(parent, order)
			assertChildOrderDiagnostic(t, err, tt.wantCode, tt.wantMsg, tt.wantLine, tt.wantCol)
		})
	}
}

func TestValidateSimpleTypeChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		children []string
		wantCode xsderrors.Code
		wantLine int
		wantCol  int
		wantMsg  string
	}{
		{name: "restriction", children: []string{"restriction"}},
		{name: "list with annotation", children: []string{"annotation", "list"}},
		{name: "union", children: []string{"union"}},
		{name: "missing derivation", wantMsg: "simpleType must contain one restriction, list, or union", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 2, wantCol: 3},
		{name: "only annotation", children: []string{"annotation"}, wantMsg: "simpleType must contain one restriction, list, or union", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 2, wantCol: 3},
		{name: "duplicate derivation", children: []string{"restriction", "list"}, wantMsg: "simpleType can contain one restriction, list, or union", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "annotation after derivation", children: []string{"restriction", "annotation"}, wantMsg: "simpleType annotation must be first", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "unsupported child", children: []string{"sequence"}, wantMsg: "unsupported simpleType child sequence", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "simpleType", tt.children)
			err := validateSimpleTypeChildren(parent)
			assertChildOrderDiagnostic(t, err, tt.wantCode, tt.wantMsg, tt.wantLine, tt.wantCol)
		})
	}
}

func TestValidateSimpleDerivationChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		order    ChildOrder
		children []string
		wantCode xsderrors.Code
		wantLine int
		wantCol  int
		wantMsg  string
	}{
		{
			name:     "restriction admits simple type before facets",
			order:    simpleRestrictionChildOrder,
			children: []string{"annotation", "simpleType", "minLength", "pattern", "enumeration"},
		},
		{
			name:     "restriction simple type after facet",
			order:    simpleRestrictionChildOrder,
			children: []string{"minLength", "simpleType"},
			wantMsg:  "restriction simpleType must precede facets",
			wantCode: xsderrors.CodeSchemaContentModel,
			wantLine: 6,
			wantCol:  5,
		},
		{
			name:     "restriction duplicate simple type",
			order:    simpleRestrictionChildOrder,
			children: []string{"simpleType", "simpleType"},
			wantMsg:  "restriction can contain one simpleType",
			wantCode: xsderrors.CodeSchemaContentModel,
			wantLine: 6,
			wantCol:  5,
		},
		{
			name:     "list duplicate simple type",
			order:    simpleListChildOrder,
			children: []string{"simpleType", "simpleType"},
			wantMsg:  "list can contain one simpleType",
			wantCode: xsderrors.CodeSchemaContentModel,
			wantLine: 6,
			wantCol:  5,
		},
		{
			name:     "list rejects direct facet",
			order:    simpleListChildOrder,
			children: []string{"enumeration"},
			wantMsg:  "invalid list child enumeration",
			wantCode: xsderrors.CodeSchemaContentModel,
			wantLine: 4,
			wantCol:  5,
		},
		{
			name:     "union admits multiple simple types",
			order:    simpleUnionChildOrder,
			children: []string{"annotation", "simpleType", "simpleType"},
		},
		{
			name:     "union rejects direct facet",
			order:    simpleUnionChildOrder,
			children: []string{"enumeration"},
			wantMsg:  "invalid union child enumeration",
			wantCode: xsderrors.CodeSchemaContentModel,
			wantLine: 4,
			wantCol:  5,
		},
		{
			name:     "union annotation after content",
			order:    simpleUnionChildOrder,
			children: []string{"simpleType", "annotation"},
			wantMsg:  "union annotation must be first",
			wantCode: xsderrors.CodeSchemaContentModel,
			wantLine: 6,
			wantCol:  5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "component", tt.children)
			err := checkOrderedXSDChildren(parent, tt.order)
			assertChildOrderDiagnostic(t, err, tt.wantCode, tt.wantMsg, tt.wantLine, tt.wantCol)
		})
	}
}

func TestValidateComplexTypeChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		children []string
		wantCode xsderrors.Code
		wantLine int
		wantCol  int
		wantMsg  string
	}{
		{name: "empty"},
		{name: "model attributes and wildcard", children: []string{"annotation", "sequence", "attribute", "attributeGroup", "anyAttribute"}},
		{name: "simple content only", children: []string{"annotation", "simpleContent"}},
		{name: "annotation after model", children: []string{"sequence", "annotation"}, wantMsg: "complexType annotation must be first", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "model after attribute", children: []string{"attribute", "sequence"}, wantMsg: "complexType model group is out of order", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "duplicate model group", children: []string{"sequence", "choice"}, wantMsg: "complexType model group is out of order", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "attribute after wildcard", children: []string{"anyAttribute", "attribute"}, wantMsg: "complexType attribute is out of order", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "duplicate wildcard", children: []string{"anyAttribute", "anyAttribute"}, wantMsg: "complexType can contain at most one anyAttribute", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "content is terminal", children: []string{"simpleContent", "attribute"}, wantMsg: "invalid complexType child attribute", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "unsupported child", children: []string{"element"}, wantMsg: "invalid complexType child element", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "complexType", tt.children)
			err := checkComplexTypeChildren(parent)
			assertChildOrderDiagnostic(t, err, tt.wantCode, tt.wantMsg, tt.wantLine, tt.wantCol)
		})
	}
}

func TestValidateDerivationContainerChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		validate func(*schemaNode) (contentDerivationSource, error)
		children []string
		wantCode xsderrors.Code
		wantLine int
		wantCol  int
		wantMsg  string
	}{
		{name: "complexContent extension", validate: checkComplexContentSyntax, children: []string{"annotation", "extension"}},
		{name: "complexContent invalid child after derivation", validate: checkComplexContentSyntax, children: []string{"extension", "sequence"}, wantMsg: "invalid complexContent child sequence", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "simpleContent restriction", validate: checkSimpleContentSyntax, children: []string{"restriction"}},
		{name: "complexContent missing derivation", validate: checkComplexContentSyntax, wantMsg: "complexContent missing extension or restriction", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 2, wantCol: 3},
		{name: "simpleContent only annotation", validate: checkSimpleContentSyntax, children: []string{"annotation"}, wantMsg: "simpleContent missing extension or restriction", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 2, wantCol: 3},
		{name: "complexContent duplicate derivation", validate: checkComplexContentSyntax, children: []string{"extension", "restriction"}, wantMsg: "complexContent can contain one derivation", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "simpleContent annotation after derivation", validate: checkSimpleContentSyntax, children: []string{"restriction", "annotation"}, wantMsg: "simpleContent annotation must be first", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "simpleContent invalid child", validate: checkSimpleContentSyntax, children: []string{"sequence"}, wantMsg: "invalid simpleContent child sequence", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "component", tt.children)
			_, err := tt.validate(parent)
			assertChildOrderDiagnostic(t, err, tt.wantCode, tt.wantMsg, tt.wantLine, tt.wantCol)
		})
	}
}

func TestCheckContentDerivationSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		validate  func(*schemaNode) (contentDerivationSource, error)
		children  []string
		wantIndex int
		wantKind  ContentDerivationKind
	}{
		{
			name:      "complexContent extension after annotation",
			validate:  checkComplexContentSyntax,
			children:  []string{"annotation", "extension"},
			wantIndex: 1, wantKind: ContentDerivationExtension,
		},
		{
			name:      "complexContent restriction",
			validate:  checkComplexContentSyntax,
			children:  []string{"restriction"},
			wantIndex: 0, wantKind: ContentDerivationRestriction,
		},
		{
			name:      "simpleContent restriction after annotation",
			validate:  checkSimpleContentSyntax,
			children:  []string{"annotation", "restriction"},
			wantIndex: 1, wantKind: ContentDerivationRestriction,
		},
		{
			name:      "simpleContent extension",
			validate:  checkSimpleContentSyntax,
			children:  []string{"extension"},
			wantIndex: 0, wantKind: ContentDerivationExtension,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "component", tt.children)
			got, err := tt.validate(parent)
			if err != nil {
				t.Fatalf("content derivation check error = %v", err)
			}
			wantNode := parent.doc.node(parent.children[tt.wantIndex])
			if got.node != wantNode || got.kind != tt.wantKind {
				t.Fatalf("derivation = {%p %s}, want {%p %s}", got.node, got.kind, wantNode, tt.wantKind)
			}
		})
	}
}

func TestValidateContentDerivationBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		base    ContentDerivationBase
		wantMsg string
	}{
		{
			name: "complex extension has base",
			base: ContentDerivationBase{
				Container:  complexContent,
				Derivation: extensionChild,
				Lexical:    "tns:base",
				Present:    true,
			},
		},
		{
			name: "complex restriction missing base",
			base: ContentDerivationBase{
				Container:  complexContent,
				Derivation: restrictionChild,
			},
			wantMsg: "complexContent restriction missing base",
		},
		{
			name: "simple extension missing base",
			base: ContentDerivationBase{
				Container:  simpleContent,
				Derivation: extensionChild,
			},
			wantMsg: "simpleContent extension missing base",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateContentDerivationBase(tt.base)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateContentDerivationBase() error = %v", err)
				}
				return
			}
			diag, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("ValidateContentDerivationBase() error = %T %[1]v, want xsderrors.Error", err)
			}
			if diag.Code() != xsderrors.CodeSchemaReference || diag.Message() != tt.wantMsg {
				t.Fatalf("diagnostic = (%s, %q), want (%s, %q)", diag.Code(), diag.Message(), xsderrors.CodeSchemaReference, tt.wantMsg)
			}
		})
	}
}

func TestValidateSimpleContentDerivationChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		validate func(*schemaNode) error
		children []string
		wantCode xsderrors.Code
		wantLine int
		wantCol  int
		wantMsg  string
	}{
		{name: "restriction admits facets and attributes", validate: checkSimpleContentRestrictionChildren, children: []string{"annotation", "simpleType", "minLength", "attribute", "attributeGroup", "anyAttribute"}},
		{name: "extension admits attributes", validate: checkSimpleContentExtensionChildren, children: []string{"annotation", "attribute", "anyAttribute"}},
		{name: "extension rejects simple type", validate: checkSimpleContentExtensionChildren, children: []string{"simpleType"}, wantMsg: "simpleContent extension cannot contain simpleType", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
		{name: "restriction simple type after facet", validate: checkSimpleContentRestrictionChildren, children: []string{"minLength", "simpleType"}, wantMsg: "simpleContent simpleType is out of order", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "restriction facet after attribute", validate: checkSimpleContentRestrictionChildren, children: []string{"attribute", "enumeration"}, wantMsg: "simpleContent facet is out of order", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "restriction duplicate simple type", validate: checkSimpleContentRestrictionChildren, children: []string{"simpleType", "simpleType"}, wantMsg: "simpleContent simpleType is out of order", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "restriction duplicate wildcard", validate: checkSimpleContentRestrictionChildren, children: []string{"anyAttribute", "anyAttribute"}, wantMsg: "simpleContent can contain at most one anyAttribute", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "component", tt.children)
			err := tt.validate(parent)
			assertChildOrderDiagnostic(t, err, tt.wantCode, tt.wantMsg, tt.wantLine, tt.wantCol)
		})
	}
}

func TestValidateComplexContentDerivationChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		validate func(*schemaNode) error
		children []string
		wantCode xsderrors.Code
		wantLine int
		wantCol  int
		wantMsg  string
	}{
		{name: "extension model attributes and wildcard", validate: checkComplexContentExtensionChildren, children: []string{"annotation", "sequence", "attribute", "attributeGroup", "anyAttribute"}},
		{name: "restriction model after attribute", validate: checkComplexContentRestrictionChildren, children: []string{"attribute", "sequence"}, wantMsg: "restriction model group is out of order", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "extension duplicate model", validate: checkComplexContentExtensionChildren, children: []string{"sequence", "choice"}, wantMsg: "extension model group is out of order", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "extension duplicate wildcard", validate: checkComplexContentExtensionChildren, children: []string{"anyAttribute", "anyAttribute"}, wantMsg: "extension can contain at most one anyAttribute", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "restriction invalid child", validate: checkComplexContentRestrictionChildren, children: []string{"simpleType"}, wantMsg: "invalid complexContent child simpleType", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "component", tt.children)
			err := tt.validate(parent)
			assertChildOrderDiagnostic(t, err, tt.wantCode, tt.wantMsg, tt.wantLine, tt.wantCol)
		})
	}
}

func TestValidateWildcardChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		order    ChildOrder
		children []string
		wantCode xsderrors.Code
		wantLine int
		wantCol  int
		wantMsg  string
	}{
		{name: "any empty", order: anyParticleChildOrder},
		{name: "any duplicate annotation", order: anyParticleChildOrder, children: []string{"annotation", "annotation"}, wantMsg: "any can contain at most one annotation", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "any rejects child", order: anyParticleChildOrder, children: []string{"element"}, wantMsg: "any can contain only annotation", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
		{name: "anyAttribute empty", order: anyAttributeChildOrder},
		{name: "anyAttribute duplicate annotation", order: anyAttributeChildOrder, children: []string{"annotation", "annotation"}, wantMsg: "anyAttribute can contain at most one annotation", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "anyAttribute rejects child", order: anyAttributeChildOrder, children: []string{"attribute"}, wantMsg: "anyAttribute can contain only annotation", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
		{name: "element ref empty", order: elementRefChildOrder},
		{name: "element ref annotation", order: elementRefChildOrder, children: []string{"annotation"}},
		{name: "element ref rejects child", order: elementRefChildOrder, children: []string{"simpleType"}, wantMsg: "element ref can contain only annotation", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
		{name: "attribute ref empty", order: attributeRefChildOrder},
		{name: "attribute ref annotation", order: attributeRefChildOrder, children: []string{"annotation"}},
		{name: "attribute ref rejects child", order: attributeRefChildOrder, children: []string{"simpleType"}, wantMsg: "attribute ref can contain only annotation", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
		{name: "attributeGroup use empty", order: attributeGroupUseChildOrder},
		{name: "attributeGroup use annotation", order: attributeGroupUseChildOrder, children: []string{"annotation"}},
		{name: "attributeGroup use rejects child", order: attributeGroupUseChildOrder, children: []string{"attribute"}, wantMsg: "attributeGroup use can contain only annotation", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
		{name: "group use empty", order: groupUseChildOrder},
		{name: "group use annotation", order: groupUseChildOrder, children: []string{"annotation"}},
		{name: "group use rejects child", order: groupUseChildOrder, children: []string{"element"}, wantMsg: "group use can contain only annotation", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
		{name: "group use rejects child after annotation", order: groupUseChildOrder, children: []string{"annotation", "sequence"}, wantMsg: "group use can contain only annotation", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "component", tt.children)
			err := checkOrderedXSDChildren(parent, tt.order)
			assertChildOrderDiagnostic(t, err, tt.wantCode, tt.wantMsg, tt.wantLine, tt.wantCol)
		})
	}
}

func TestCheckTopLevelGroupChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		children  []groupChildFixture
		wantModel int
		wantIndex int
		wantCode  xsderrors.Code
		wantMsg   string
	}{
		{
			name:      "sequence",
			children:  []groupChildFixture{{local: sequenceChild}},
			wantModel: 0,
		},
		{
			name:      "annotation then all",
			children:  []groupChildFixture{{local: annotationChild}, {local: allChild}},
			wantModel: 1,
		},
		{
			name:      "duplicate annotation",
			children:  []groupChildFixture{{local: annotationChild}, {local: annotationChild}},
			wantIndex: 1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "top-level group can contain at most one annotation",
		},
		{
			name:      "annotation after model",
			children:  []groupChildFixture{{local: sequenceChild}, {local: annotationChild}},
			wantIndex: 1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "top-level group annotation must be first",
		},
		{
			name:      "duplicate model",
			children:  []groupChildFixture{{local: sequenceChild}, {local: choiceChild}},
			wantIndex: 1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "top-level group must contain exactly one model group",
		},
		{
			name:      "missing model",
			children:  []groupChildFixture{{local: annotationChild}},
			wantIndex: -1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "top-level group must contain exactly one model group",
		},
		{
			name:      "group ref",
			children:  []groupChildFixture{{local: groupChild}},
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "top-level group cannot contain group ref",
		},
		{
			name:      "invalid child",
			children:  []groupChildFixture{{local: elementChild}},
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "invalid top-level group child element",
		},
		{
			name:      "model min occurs",
			children:  []groupChildFixture{{local: sequenceChild, hasMinOccurs: true}},
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaOccurrence,
			wantMsg:   "top-level model group cannot have minOccurs",
		},
		{
			name:      "model max occurs",
			children:  []groupChildFixture{{local: choiceChild, hasMaxOccurs: true}},
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaOccurrence,
			wantMsg:   "top-level model group cannot have maxOccurs",
		},
		{
			name:      "later invalid child before occurrence",
			children:  []groupChildFixture{{local: sequenceChild, hasMinOccurs: true}, {local: elementChild}},
			wantIndex: 1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "invalid top-level group child element",
		},
		{
			name:      "min occurrence before max",
			children:  []groupChildFixture{{local: sequenceChild, hasMinOccurs: true, hasMaxOccurs: true}},
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaOccurrence,
			wantMsg:   "top-level model group cannot have minOccurs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := topLevelGroupGrammarFixture(t, tt.children)
			model, err := checkTopLevelGroupChildren(parent)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("checkTopLevelGroupChildren() error = %v", err)
				}
				if want := parent.doc.node(parent.children[tt.wantModel]); model != want {
					t.Fatalf("model = %p, want child %d (%p)", model, tt.wantModel, want)
				}
				return
			}
			issue, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("checkTopLevelGroupChildren() error = %T %[1]v, want structured diagnostic", err)
			}
			target := parent
			if tt.wantIndex >= 0 {
				target = parent.doc.node(parent.children[tt.wantIndex])
			}
			if issue.Category() != xsderrors.CategorySchemaCompile || issue.Code() != tt.wantCode || issue.Message() != tt.wantMsg {
				t.Fatalf("diagnostic = {%s %s %q}, want {%s %s %q}", issue.Category(), issue.Code(), issue.Message(), xsderrors.CategorySchemaCompile, tt.wantCode, tt.wantMsg)
			}
			if issue.Path() != "group.xsd" || issue.Line() != target.line || issue.Column() != target.column {
				t.Fatalf("location = {%q %d %d}, want {%q %d %d}", issue.Path(), issue.Line(), issue.Column(), "group.xsd", target.line, target.column)
			}
		})
	}
}

type groupChildFixture struct {
	local        string
	hasMinOccurs bool
	hasMaxOccurs bool
}

func topLevelGroupGrammarFixture(t *testing.T, children []groupChildFixture) *schemaNode {
	t.Helper()
	doc := &schemaDocument{name: "group.xsd"}
	parent := &schemaNode{doc: doc, local: groupChild, line: 1, column: 3}
	doc.arena = append(doc.arena, parent)
	for i, child := range children {
		id, err := schemaNodeIDFor(len(doc.arena))
		if err != nil {
			t.Fatal(err)
		}
		node := &schemaNode{doc: doc, id: id, local: child.local, line: 100 + i, column: 7}
		switch child.local {
		case sequenceChild, choiceChild, allChild:
			kind := ModelSequence
			switch child.local {
			case choiceChild:
				kind = ModelChoice
			case allChild:
				kind = ModelAll
			}
			particle := schemaParticleSource{
				MinOccurs: LexicalAttribute{Present: child.hasMinOccurs},
				MaxOccurs: LexicalAttribute{Present: child.hasMaxOccurs},
			}
			node.semantic.kind = &schemaModelSource{schemaParticleSource: particle, Kind: kind}
		}
		doc.arena = append(doc.arena, node)
		parent.children = append(parent.children, node.id)
	}
	return parent
}

func TestValidateModelGroupChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		parent   string
		children []string
		wantCode xsderrors.Code
		wantLine int
		wantCol  int
		wantMsg  string
	}{
		{name: "empty sequence", parent: sequenceChild},
		{name: "annotation then particles", parent: sequenceChild, children: []string{annotationChild, elementChild, choiceChild, anyChild}},
		{name: "choice annotation after particle", parent: choiceChild, children: []string{elementChild, annotationChild}, wantMsg: "choice annotation must be first", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "all invalid child", parent: allChild, children: []string{attributeChild}, wantMsg: "invalid model group child attribute", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "component", tt.children)
			err := checkOrderedXSDChildren(parent, modelGroupChildOrder(tt.parent))
			assertChildOrderDiagnostic(t, err, tt.wantCode, tt.wantMsg, tt.wantLine, tt.wantCol)
		})
	}
}

func TestValidateAttributeDeclarationChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		children []string
		wantCode xsderrors.Code
		wantLine int
		wantCol  int
		wantMsg  string
	}{
		{name: "empty"},
		{name: "annotation only", children: []string{"annotation"}},
		{name: "simple type after annotation", children: []string{"annotation", "simpleType"}},
		{name: "annotation after simple type", children: []string{"simpleType", "annotation"}, wantMsg: "attribute annotation must precede simpleType", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "duplicate simple type", children: []string{"simpleType", "simpleType"}, wantMsg: "attribute can contain at most one simpleType", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "unsupported child", children: []string{"complexType"}, wantMsg: "invalid attribute child complexType", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "attribute", tt.children)
			err := checkAttributeDeclarationChildren(parent)
			assertChildOrderDiagnostic(t, err, tt.wantCode, tt.wantMsg, tt.wantLine, tt.wantCol)
		})
	}
}

func TestValidateAttributeGroupDeclarationChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		children []string
		wantCode xsderrors.Code
		wantLine int
		wantCol  int
		wantMsg  string
	}{
		{name: "empty"},
		{name: "attribute uses", children: []string{annotationChild, attributeChild, attributeGroup, anyAttribute}},
		{name: "annotation after attribute", children: []string{attributeChild, annotationChild}, wantMsg: "attributeGroup annotation must be first", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "attribute after wildcard", children: []string{anyAttribute, attributeChild}, wantMsg: "attributeGroup attribute is out of order", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "duplicate wildcard", children: []string{anyAttribute, anyAttribute}, wantMsg: "attributeGroup can contain at most one anyAttribute", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "invalid child", children: []string{elementChild}, wantMsg: "invalid attribute use child element", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "attributeGroup", tt.children)
			err := checkAttributeGroupDeclarationChildren(parent)
			assertChildOrderDiagnostic(t, err, tt.wantCode, tt.wantMsg, tt.wantLine, tt.wantCol)
		})
	}
}

func TestValidateElementDeclarationChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		children    []string
		hasTypeAttr bool
		wantCode    xsderrors.Code
		wantLine    int
		wantCol     int
		wantMsg     string
	}{
		{name: "empty"},
		{name: "annotation anonymous type identities", children: []string{"annotation", "complexType", "unique", "key", "keyref"}},
		{name: "type attribute with identities", children: []string{"key"}, hasTypeAttr: true},
		{name: "annotation after identity", children: []string{"key", "annotation"}, wantMsg: "element annotation must be first", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "duplicate anonymous type", children: []string{"simpleType", "complexType"}, wantMsg: "element can contain at most one anonymous type", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "anonymous type after identity", children: []string{"unique", "simpleType"}, wantMsg: "element anonymous type must precede identity constraints", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
		{name: "unsupported child", children: []string{"sequence"}, wantMsg: "invalid element child sequence", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 4, wantCol: 5},
		{name: "type attribute with anonymous type", children: []string{"complexType"}, hasTypeAttr: true, wantMsg: "element cannot have both type and anonymous type", wantCode: xsderrors.CodeSchemaInvalidAttribute, wantLine: 2, wantCol: 3},
		{name: "child grammar precedes type conflict", children: []string{"complexType", "sequence"}, hasTypeAttr: true, wantMsg: "invalid element child sequence", wantCode: xsderrors.CodeSchemaContentModel, wantLine: 6, wantCol: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parent := childOrderGrammarFixture(t, "element", tt.children)
			parent.semantic.kind = &schemaElementSource{Type: schemaQNameAttribute{Lexical: LexicalAttribute{Present: tt.hasTypeAttr}}}
			err := checkElementDeclarationChildren(parent)
			assertChildOrderDiagnostic(t, err, tt.wantCode, tt.wantMsg, tt.wantLine, tt.wantCol)
		})
	}
}

func matchTestChildren(want ...string) func(string) bool {
	return func(local string) bool {
		return slices.Contains(want, local)
	}
}

func childOrderGrammarFixture(t *testing.T, local string, children []string) *schemaNode {
	t.Helper()
	doc := &schemaDocument{name: "child-order.xsd"}
	parent := &schemaNode{doc: doc, local: local, line: 2, column: 3}
	doc.arena = append(doc.arena, parent)
	for i, child := range children {
		id, err := schemaNodeIDFor(len(doc.arena))
		if err != nil {
			t.Fatal(err)
		}
		node := &schemaNode{doc: doc, id: id, local: child, line: 4 + 2*i, column: 5}
		doc.arena = append(doc.arena, node)
		parent.children = append(parent.children, id)
	}
	return parent
}

func assertChildOrderDiagnostic(t *testing.T, err error, code xsderrors.Code, message string, line, column int) {
	t.Helper()
	if message == "" {
		if err != nil {
			t.Fatalf("child grammar error = %v", err)
		}
		return
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("child grammar error = %T %[1]v, want structured diagnostic", err)
	}
	if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != code || diagnostic.Message() != message {
		t.Fatalf("diagnostic = {%s %s %q}, want {%s %s %q}", diagnostic.Category(), diagnostic.Code(), diagnostic.Message(), xsderrors.CategorySchemaCompile, code, message)
	}
	if diagnostic.Path() != "child-order.xsd" || diagnostic.Line() != line || diagnostic.Column() != column {
		t.Fatalf("location = {%q %d %d}, want {child-order.xsd %d %d}", diagnostic.Path(), diagnostic.Line(), diagnostic.Column(), line, column)
	}
}
