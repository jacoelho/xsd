package compile

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestCheckOrderedChildren(t *testing.T) {
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
		name      string
		children  []string
		wantIndex int
		wantMsg   string
	}{
		{name: "valid", children: []string{"annotation", "head", "body"}},
		{name: "second annotation", children: []string{"annotation", "annotation"}, wantIndex: 1, wantMsg: "annotation must be first"},
		{name: "annotation after child", children: []string{"head", "annotation"}, wantIndex: 1, wantMsg: "annotation must be first"},
		{name: "unknown child", children: []string{"bogus"}, wantIndex: 0, wantMsg: "invalid bogus"},
		{name: "out of order", children: []string{"body", "head"}, wantIndex: 1, wantMsg: "head must precede body"},
		{name: "duplicate max one", children: []string{"head", "head"}, wantIndex: 1, wantMsg: "head duplicates"},
		{name: "forbidden", children: []string{"tail"}, wantIndex: 0, wantMsg: "tail is forbidden"},
		{name: "terminal", children: []string{"head", "tail", "body"}, wantIndex: 1, wantMsg: "tail is forbidden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := CheckOrderedChildren(tt.children, order)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("CheckOrderedChildren() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				t.Fatalf("CheckOrderedChildren() error = %T %[1]v, want ChildOrderError", err)
			}
			if issue.Index != tt.wantIndex || issue.Message != tt.wantMsg {
				t.Fatalf("ChildOrderError = {Index:%d Message:%q}, want {%d %q}", issue.Index, issue.Message, tt.wantIndex, tt.wantMsg)
			}
		})
	}
}

func TestValidateSimpleTypeChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		children  []string
		wantIndex int
		wantMsg   string
	}{
		{name: "restriction", children: []string{"restriction"}},
		{name: "list with annotation", children: []string{"annotation", "list"}},
		{name: "union", children: []string{"union"}},
		{name: "missing derivation", wantIndex: -1, wantMsg: "simpleType must contain one restriction, list, or union"},
		{name: "only annotation", children: []string{"annotation"}, wantIndex: -1, wantMsg: "simpleType must contain one restriction, list, or union"},
		{name: "duplicate derivation", children: []string{"restriction", "list"}, wantIndex: 1, wantMsg: "simpleType can contain one restriction, list, or union"},
		{name: "annotation after derivation", children: []string{"restriction", "annotation"}, wantIndex: 1, wantMsg: "simpleType annotation must be first"},
		{name: "unsupported child", children: []string{"sequence"}, wantIndex: 0, wantMsg: "unsupported simpleType child sequence"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSimpleTypeChildren(tt.children)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleTypeChildren() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				t.Fatalf("ValidateSimpleTypeChildren() error = %T %[1]v, want ChildOrderError", err)
			}
			if issue.Index != tt.wantIndex || issue.Message != tt.wantMsg {
				t.Fatalf("ChildOrderError = {Index:%d Message:%q}, want {%d %q}", issue.Index, issue.Message, tt.wantIndex, tt.wantMsg)
			}
		})
	}
}

func TestValidateSimpleDerivationChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		validate  func([]string) error
		children  []string
		wantIndex int
		wantMsg   string
	}{
		{
			name:     "restriction admits simple type before facets",
			validate: ValidateSimpleRestrictionChildren,
			children: []string{"annotation", "simpleType", "minLength", "pattern", "enumeration"},
		},
		{
			name:      "restriction simple type after facet",
			validate:  ValidateSimpleRestrictionChildren,
			children:  []string{"minLength", "simpleType"},
			wantIndex: 1,
			wantMsg:   "restriction simpleType must precede facets",
		},
		{
			name:      "restriction duplicate simple type",
			validate:  ValidateSimpleRestrictionChildren,
			children:  []string{"simpleType", "simpleType"},
			wantIndex: 1,
			wantMsg:   "restriction can contain one simpleType",
		},
		{
			name:      "list duplicate simple type",
			validate:  ValidateSimpleListChildren,
			children:  []string{"simpleType", "simpleType"},
			wantIndex: 1,
			wantMsg:   "list can contain one simpleType",
		},
		{
			name:      "list rejects direct facet",
			validate:  ValidateSimpleListChildren,
			children:  []string{"enumeration"},
			wantIndex: 0,
			wantMsg:   "invalid list child enumeration",
		},
		{
			name:     "union admits multiple simple types",
			validate: ValidateSimpleUnionChildren,
			children: []string{"annotation", "simpleType", "simpleType"},
		},
		{
			name:      "union rejects direct facet",
			validate:  ValidateSimpleUnionChildren,
			children:  []string{"enumeration"},
			wantIndex: 0,
			wantMsg:   "invalid union child enumeration",
		},
		{
			name:      "union annotation after content",
			validate:  ValidateSimpleUnionChildren,
			children:  []string{"simpleType", "annotation"},
			wantIndex: 1,
			wantMsg:   "union annotation must be first",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.validate(tt.children)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("validate() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				t.Fatalf("validate() error = %T %[1]v, want ChildOrderError", err)
			}
			if issue.Index != tt.wantIndex || issue.Message != tt.wantMsg {
				t.Fatalf("ChildOrderError = {Index:%d Message:%q}, want {%d %q}", issue.Index, issue.Message, tt.wantIndex, tt.wantMsg)
			}
		})
	}
}

func TestValidateComplexTypeChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		children  []string
		wantIndex int
		wantMsg   string
	}{
		{name: "empty"},
		{name: "model attributes and wildcard", children: []string{"annotation", "sequence", "attribute", "attributeGroup", "anyAttribute"}},
		{name: "simple content only", children: []string{"annotation", "simpleContent"}},
		{name: "annotation after model", children: []string{"sequence", "annotation"}, wantIndex: 1, wantMsg: "complexType annotation must be first"},
		{name: "model after attribute", children: []string{"attribute", "sequence"}, wantIndex: 1, wantMsg: complexTypeModelGroupOutOfOrder},
		{name: "duplicate model group", children: []string{"sequence", "choice"}, wantIndex: 1, wantMsg: complexTypeModelGroupOutOfOrder},
		{name: "attribute after wildcard", children: []string{"anyAttribute", "attribute"}, wantIndex: 1, wantMsg: "complexType attribute is out of order"},
		{name: "duplicate wildcard", children: []string{"anyAttribute", "anyAttribute"}, wantIndex: 1, wantMsg: "complexType can contain at most one anyAttribute"},
		{name: "content is terminal", children: []string{"simpleContent", "attribute"}, wantIndex: 1, wantMsg: "invalid complexType child attribute"},
		{name: "unsupported child", children: []string{"element"}, wantIndex: 0, wantMsg: "invalid complexType child element"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateComplexTypeChildren(tt.children)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateComplexTypeChildren() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				t.Fatalf("ValidateComplexTypeChildren() error = %T %[1]v, want ChildOrderError", err)
			}
			if issue.Index != tt.wantIndex || issue.Message != tt.wantMsg {
				t.Fatalf("ChildOrderError = {Index:%d Message:%q}, want {%d %q}", issue.Index, issue.Message, tt.wantIndex, tt.wantMsg)
			}
		})
	}
}

func TestValidateDerivationContainerChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		validate  func([]string) error
		children  []string
		wantIndex int
		wantMsg   string
	}{
		{name: "complexContent extension", validate: ValidateComplexContentChildren, children: []string{"annotation", "extension"}},
		{name: "simpleContent restriction", validate: ValidateSimpleContentChildren, children: []string{"restriction"}},
		{name: "complexContent missing derivation", validate: ValidateComplexContentChildren, wantIndex: -1, wantMsg: "complexContent missing extension or restriction"},
		{name: "simpleContent only annotation", validate: ValidateSimpleContentChildren, children: []string{"annotation"}, wantIndex: -1, wantMsg: "simpleContent missing extension or restriction"},
		{name: "complexContent duplicate derivation", validate: ValidateComplexContentChildren, children: []string{"extension", "restriction"}, wantIndex: 1, wantMsg: "complexContent can contain one derivation"},
		{name: "simpleContent annotation after derivation", validate: ValidateSimpleContentChildren, children: []string{"restriction", "annotation"}, wantIndex: 1, wantMsg: "simpleContent annotation must be first"},
		{name: "simpleContent invalid child", validate: ValidateSimpleContentChildren, children: []string{"sequence"}, wantIndex: 0, wantMsg: "invalid simpleContent child sequence"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.validate(tt.children)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("validate() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				t.Fatalf("validate() error = %T %[1]v, want ChildOrderError", err)
			}
			if issue.Index != tt.wantIndex || issue.Message != tt.wantMsg {
				t.Fatalf("ChildOrderError = {Index:%d Message:%q}, want {%d %q}", issue.Index, issue.Message, tt.wantIndex, tt.wantMsg)
			}
		})
	}
}

func TestValidateContentDerivationChildrenSyntax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		validate func([]string) (ContentDerivationSyntax, error)
		children []string
		want     ContentDerivationSyntax
	}{
		{
			name:     "complexContent extension after annotation",
			validate: ValidateComplexContentChildrenSyntax,
			children: []string{"annotation", "extension"},
			want:     ContentDerivationSyntax{Index: 1, Kind: ContentDerivationExtension},
		},
		{
			name:     "complexContent restriction",
			validate: ValidateComplexContentChildrenSyntax,
			children: []string{"restriction"},
			want:     ContentDerivationSyntax{Index: 0, Kind: ContentDerivationRestriction},
		},
		{
			name:     "simpleContent restriction after annotation",
			validate: ValidateSimpleContentChildrenSyntax,
			children: []string{"annotation", "restriction"},
			want:     ContentDerivationSyntax{Index: 1, Kind: ContentDerivationRestriction},
		},
		{
			name:     "simpleContent extension",
			validate: ValidateSimpleContentChildrenSyntax,
			children: []string{"extension"},
			want:     ContentDerivationSyntax{Index: 0, Kind: ContentDerivationExtension},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.validate(tt.children)
			if err != nil {
				t.Fatalf("validate() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ContentDerivationSyntax = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateContentDerivationBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		container  string
		derivation string
		hasBase    bool
		wantMsg    string
	}{
		{
			name:       "complex extension has base",
			container:  complexContent,
			derivation: extensionChild,
			hasBase:    true,
		},
		{
			name:       "complex restriction missing base",
			container:  complexContent,
			derivation: restrictionChild,
			wantMsg:    "complexContent restriction missing base",
		},
		{
			name:       "simple extension missing base",
			container:  simpleContent,
			derivation: extensionChild,
			wantMsg:    "simpleContent extension missing base",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateContentDerivationBase(tt.container, tt.derivation, tt.hasBase)
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
			if diag.Code != xsderrors.CodeSchemaReference || diag.Message != tt.wantMsg {
				t.Fatalf("diagnostic = (%s, %q), want (%s, %q)", diag.Code, diag.Message, xsderrors.CodeSchemaReference, tt.wantMsg)
			}
		})
	}
}

func TestValidateSimpleContentDerivationChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		validate  func([]string) error
		children  []string
		wantIndex int
		wantMsg   string
	}{
		{name: "restriction admits facets and attributes", validate: ValidateSimpleContentRestrictionChildren, children: []string{"annotation", "simpleType", "minLength", "attribute", "attributeGroup", "anyAttribute"}},
		{name: "extension admits attributes", validate: ValidateSimpleContentExtensionChildren, children: []string{"annotation", "attribute", "anyAttribute"}},
		{name: "extension rejects simple type", validate: ValidateSimpleContentExtensionChildren, children: []string{"simpleType"}, wantIndex: 0, wantMsg: simpleContentExtensionCannotContainSimpleType},
		{name: "restriction facet after attribute", validate: ValidateSimpleContentRestrictionChildren, children: []string{"attribute", "enumeration"}, wantIndex: 1, wantMsg: simpleContentFacetOutOfOrder},
		{name: "restriction duplicate simple type", validate: ValidateSimpleContentRestrictionChildren, children: []string{"simpleType", "simpleType"}, wantIndex: 1, wantMsg: simpleContentSimpleTypeOutOfOrder},
		{name: "restriction duplicate wildcard", validate: ValidateSimpleContentRestrictionChildren, children: []string{"anyAttribute", "anyAttribute"}, wantIndex: 1, wantMsg: "simpleContent" + oneAnyAttributeSuffix},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.validate(tt.children)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("validate() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				t.Fatalf("validate() error = %T %[1]v, want ChildOrderError", err)
			}
			if issue.Index != tt.wantIndex || issue.Message != tt.wantMsg {
				t.Fatalf("ChildOrderError = {Index:%d Message:%q}, want {%d %q}", issue.Index, issue.Message, tt.wantIndex, tt.wantMsg)
			}
		})
	}
}

func TestValidateComplexContentDerivationChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		validate  func([]string) error
		children  []string
		wantIndex int
		wantMsg   string
	}{
		{name: "extension model attributes and wildcard", validate: ValidateComplexContentExtensionChildren, children: []string{"annotation", "sequence", "attribute", "attributeGroup", "anyAttribute"}},
		{name: "restriction model after attribute", validate: ValidateComplexContentRestrictionChildren, children: []string{"attribute", "sequence"}, wantIndex: 1, wantMsg: "restriction" + modelGroupOutOfOrderSuffix},
		{name: "extension duplicate model", validate: ValidateComplexContentExtensionChildren, children: []string{"sequence", "choice"}, wantIndex: 1, wantMsg: "extension" + modelGroupOutOfOrderSuffix},
		{name: "extension duplicate wildcard", validate: ValidateComplexContentExtensionChildren, children: []string{"anyAttribute", "anyAttribute"}, wantIndex: 1, wantMsg: "extension" + oneAnyAttributeSuffix},
		{name: "restriction invalid child", validate: ValidateComplexContentRestrictionChildren, children: []string{"simpleType"}, wantIndex: 0, wantMsg: "invalid complexContent child simpleType"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.validate(tt.children)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("validate() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				t.Fatalf("validate() error = %T %[1]v, want ChildOrderError", err)
			}
			if issue.Index != tt.wantIndex || issue.Message != tt.wantMsg {
				t.Fatalf("ChildOrderError = {Index:%d Message:%q}, want {%d %q}", issue.Index, issue.Message, tt.wantIndex, tt.wantMsg)
			}
		})
	}
}

func TestValidateWildcardChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		validate  func([]string) error
		children  []string
		wantIndex int
		wantMsg   string
	}{
		{name: "any empty", validate: ValidateAnyParticleChildren},
		{name: "any duplicate annotation", validate: ValidateAnyParticleChildren, children: []string{"annotation", "annotation"}, wantIndex: 1, wantMsg: "any can contain at most one annotation"},
		{name: "any rejects child", validate: ValidateAnyParticleChildren, children: []string{"element"}, wantIndex: 0, wantMsg: "any can contain only annotation"},
		{name: "anyAttribute empty", validate: ValidateAnyAttributeChildren},
		{name: "anyAttribute duplicate annotation", validate: ValidateAnyAttributeChildren, children: []string{"annotation", "annotation"}, wantIndex: 1, wantMsg: "anyAttribute can contain at most one annotation"},
		{name: "anyAttribute rejects child", validate: ValidateAnyAttributeChildren, children: []string{"attribute"}, wantIndex: 0, wantMsg: "anyAttribute can contain only annotation"},
		{name: "element ref empty", validate: ValidateElementRefChildren},
		{name: "element ref annotation", validate: ValidateElementRefChildren, children: []string{"annotation"}},
		{name: "element ref rejects child", validate: ValidateElementRefChildren, children: []string{"simpleType"}, wantIndex: 0, wantMsg: "element ref can contain only annotation"},
		{name: "attribute ref empty", validate: ValidateAttributeRefChildren},
		{name: "attribute ref annotation", validate: ValidateAttributeRefChildren, children: []string{"annotation"}},
		{name: "attribute ref rejects child", validate: ValidateAttributeRefChildren, children: []string{"simpleType"}, wantIndex: 0, wantMsg: "attribute ref can contain only annotation"},
		{name: "attributeGroup use empty", validate: ValidateAttributeGroupUseChildren},
		{name: "attributeGroup use annotation", validate: ValidateAttributeGroupUseChildren, children: []string{"annotation"}},
		{name: "attributeGroup use rejects child", validate: ValidateAttributeGroupUseChildren, children: []string{"attribute"}, wantIndex: 0, wantMsg: "attributeGroup use can contain only annotation"},
		{name: "group use empty", validate: ValidateGroupUseChildren},
		{name: "group use annotation", validate: ValidateGroupUseChildren, children: []string{"annotation"}},
		{name: "group use rejects child", validate: ValidateGroupUseChildren, children: []string{"element"}, wantIndex: 0, wantMsg: "group use can contain only annotation"},
		{name: "group use rejects child after annotation", validate: ValidateGroupUseChildren, children: []string{"annotation", "sequence"}, wantIndex: 1, wantMsg: "group use can contain only annotation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.validate(tt.children)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("validate() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				t.Fatalf("validate() error = %T %[1]v, want ChildOrderError", err)
			}
			if issue.Index != tt.wantIndex || issue.Message != tt.wantMsg {
				t.Fatalf("ChildOrderError = {Index:%d Message:%q}, want {%d %q}", issue.Index, issue.Message, tt.wantIndex, tt.wantMsg)
			}
		})
	}
}

func TestValidateTopLevelGroupChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		children  []TopLevelGroupChild
		wantModel int
		wantIndex int
		wantCode  xsderrors.Code
		wantMsg   string
	}{
		{
			name:      "sequence",
			children:  []TopLevelGroupChild{{Local: sequenceChild}},
			wantModel: 0,
		},
		{
			name:      "annotation then all",
			children:  []TopLevelGroupChild{{Local: annotationChild}, {Local: allChild}},
			wantModel: 1,
		},
		{
			name:      "duplicate annotation",
			children:  []TopLevelGroupChild{{Local: annotationChild}, {Local: annotationChild}},
			wantIndex: 1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "top-level group can contain at most one annotation",
		},
		{
			name:      "annotation after model",
			children:  []TopLevelGroupChild{{Local: sequenceChild}, {Local: annotationChild}},
			wantIndex: 1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "top-level group annotation must be first",
		},
		{
			name:      "duplicate model",
			children:  []TopLevelGroupChild{{Local: sequenceChild}, {Local: choiceChild}},
			wantIndex: 1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "top-level group must contain exactly one model group",
		},
		{
			name:      "missing model",
			children:  []TopLevelGroupChild{{Local: annotationChild}},
			wantIndex: -1,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "top-level group must contain exactly one model group",
		},
		{
			name:      "group ref",
			children:  []TopLevelGroupChild{{Local: groupChild}},
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "top-level group cannot contain group ref",
		},
		{
			name:      "invalid child",
			children:  []TopLevelGroupChild{{Local: elementChild}},
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaContentModel,
			wantMsg:   "invalid top-level group child element",
		},
		{
			name:      "model min occurs",
			children:  []TopLevelGroupChild{{Local: sequenceChild, HasMinOccurs: true}},
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaOccurrence,
			wantMsg:   "top-level model group cannot have minOccurs",
		},
		{
			name:      "model max occurs",
			children:  []TopLevelGroupChild{{Local: choiceChild, HasMaxOccurs: true}},
			wantIndex: 0,
			wantCode:  xsderrors.CodeSchemaOccurrence,
			wantMsg:   "top-level model group cannot have maxOccurs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			syntax, err := ValidateTopLevelGroupChildren(tt.children)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateTopLevelGroupChildren() error = %v", err)
				}
				if syntax.Model != tt.wantModel {
					t.Fatalf("Model = %d, want %d", syntax.Model, tt.wantModel)
				}
				return
			}
			issue, ok := errors.AsType[*TopLevelGroupSyntaxError](err)
			if !ok {
				t.Fatalf("ValidateTopLevelGroupChildren() error = %T %[1]v, want TopLevelGroupSyntaxError", err)
			}
			if issue.Index != tt.wantIndex || issue.Code != tt.wantCode || issue.Message != tt.wantMsg {
				t.Fatalf("TopLevelGroupSyntaxError = {Index:%d Code:%s Message:%q}, want {%d %s %q}", issue.Index, issue.Code, issue.Message, tt.wantIndex, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

func TestValidateModelGroupChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		parent    string
		children  []string
		wantIndex int
		wantMsg   string
	}{
		{name: "empty sequence", parent: sequenceChild},
		{name: "annotation then particles", parent: sequenceChild, children: []string{annotationChild, elementChild, choiceChild, anyChild}},
		{name: "choice annotation after particle", parent: choiceChild, children: []string{elementChild, annotationChild}, wantIndex: 1, wantMsg: "choice annotation must be first"},
		{name: "all invalid child", parent: allChild, children: []string{attributeChild}, wantIndex: 0, wantMsg: "invalid model group child attribute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateModelGroupChildren(tt.parent, tt.children)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateModelGroupChildren() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				t.Fatalf("ValidateModelGroupChildren() error = %T %[1]v, want ChildOrderError", err)
			}
			if issue.Index != tt.wantIndex || issue.Message != tt.wantMsg {
				t.Fatalf("ChildOrderError = {Index:%d Message:%q}, want {%d %q}", issue.Index, issue.Message, tt.wantIndex, tt.wantMsg)
			}
		})
	}
}

func TestValidateAttributeDeclarationChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		children  []string
		wantIndex int
		wantMsg   string
	}{
		{name: "empty"},
		{name: "annotation only", children: []string{"annotation"}},
		{name: "simple type after annotation", children: []string{"annotation", "simpleType"}},
		{name: "annotation after simple type", children: []string{"simpleType", "annotation"}, wantIndex: 1, wantMsg: "attribute annotation must precede simpleType"},
		{name: "duplicate simple type", children: []string{"simpleType", "simpleType"}, wantIndex: 1, wantMsg: "attribute can contain at most one simpleType"},
		{name: "unsupported child", children: []string{"complexType"}, wantIndex: 0, wantMsg: "invalid attribute child complexType"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributeDeclarationChildren(tt.children)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeDeclarationChildren() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				t.Fatalf("ValidateAttributeDeclarationChildren() error = %T %[1]v, want ChildOrderError", err)
			}
			if issue.Index != tt.wantIndex || issue.Message != tt.wantMsg {
				t.Fatalf("ChildOrderError = {Index:%d Message:%q}, want {%d %q}", issue.Index, issue.Message, tt.wantIndex, tt.wantMsg)
			}
		})
	}
}

func TestValidateAttributeGroupDeclarationChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		children  []string
		wantIndex int
		wantMsg   string
	}{
		{name: "empty"},
		{name: "attribute uses", children: []string{annotationChild, attributeChild, attributeGroup, anyAttribute}},
		{name: "annotation after attribute", children: []string{attributeChild, annotationChild}, wantIndex: 1, wantMsg: "attributeGroup annotation must be first"},
		{name: "attribute after wildcard", children: []string{anyAttribute, attributeChild}, wantIndex: 1, wantMsg: "attributeGroup attribute is out of order"},
		{name: "duplicate wildcard", children: []string{anyAttribute, anyAttribute}, wantIndex: 1, wantMsg: "attributeGroup can contain at most one anyAttribute"},
		{name: "invalid child", children: []string{elementChild}, wantIndex: 0, wantMsg: "invalid attribute use child element"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributeGroupDeclarationChildren(tt.children)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeGroupDeclarationChildren() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				t.Fatalf("ValidateAttributeGroupDeclarationChildren() error = %T %[1]v, want ChildOrderError", err)
			}
			if issue.Index != tt.wantIndex || issue.Message != tt.wantMsg {
				t.Fatalf("ChildOrderError = {Index:%d Message:%q}, want {%d %q}", issue.Index, issue.Message, tt.wantIndex, tt.wantMsg)
			}
		})
	}
}

func TestValidateElementDeclarationChildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		children    []string
		hasTypeAttr bool
		wantIndex   int
		wantMsg     string
	}{
		{name: "empty"},
		{name: "annotation anonymous type identities", children: []string{"annotation", "complexType", "unique", "key", "keyref"}},
		{name: "type attribute with identities", children: []string{"key"}, hasTypeAttr: true},
		{name: "annotation after identity", children: []string{"key", "annotation"}, wantIndex: 1, wantMsg: "element annotation must be first"},
		{name: "duplicate anonymous type", children: []string{"simpleType", "complexType"}, wantIndex: 1, wantMsg: "element can contain at most one anonymous type"},
		{name: "anonymous type after identity", children: []string{"unique", "simpleType"}, wantIndex: 1, wantMsg: "element anonymous type must precede identity constraints"},
		{name: "unsupported child", children: []string{"sequence"}, wantIndex: 0, wantMsg: "invalid element child sequence"},
		{name: "type attribute with anonymous type", children: []string{"complexType"}, hasTypeAttr: true, wantIndex: -1, wantMsg: "element cannot have both type and anonymous type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateElementDeclarationChildren(tt.children, tt.hasTypeAttr)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatalf("ValidateElementDeclarationChildren() error = %v", err)
				}
				return
			}
			issue, ok := errors.AsType[*ChildOrderError](err)
			if !ok {
				t.Fatalf("ValidateElementDeclarationChildren() error = %T %[1]v, want ChildOrderError", err)
			}
			if issue.Index != tt.wantIndex || issue.Message != tt.wantMsg {
				t.Fatalf("ChildOrderError = {Index:%d Message:%q}, want {%d %q}", issue.Index, issue.Message, tt.wantIndex, tt.wantMsg)
			}
		})
	}
}

func matchTestChildren(want ...string) func(string) bool {
	return func(local string) bool {
		return slices.Contains(want, local)
	}
}
