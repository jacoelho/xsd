package semanticcheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
	"github.com/jacoelho/xsd/internal/parser"
)

// TestValidateSelectorXPathDirect tests validateSelectorXPath directly.
func TestValidateSelectorXPathDirect(t *testing.T) {
	tests := []struct {
		name      string
		xpath     string
		errorMsg  string
		shouldErr bool
	}{
		{
			name:      "empty xpath should fail",
			xpath:     "",
			shouldErr: true,
			errorMsg:  "selector xpath cannot be empty",
		},
		{
			name:      "whitespace only should fail",
			xpath:     "   ",
			shouldErr: true,
			errorMsg:  "selector xpath cannot be empty",
		},
		{
			name:      "attribute selection at start should fail",
			xpath:     "@number",
			shouldErr: true,
			errorMsg:  "selector xpath cannot select attributes",
		},
		{
			name:      "attribute selection in middle should fail",
			xpath:     "parts/part/@number",
			shouldErr: true,
			errorMsg:  "selector xpath cannot select attributes",
		},
		{
			name:      "text node selection should fail",
			xpath:     "child::text()",
			shouldErr: true,
			errorMsg:  "selector xpath cannot select text nodes",
		},
		{
			name:      "text() at end should fail",
			xpath:     "text()",
			shouldErr: true,
			errorMsg:  "selector xpath cannot select text nodes",
		},
		{
			name:      "text() in middle should fail",
			xpath:     "parts/part/text()",
			shouldErr: true,
			errorMsg:  "selector xpath cannot select text nodes",
		},
		{
			name:      "parent navigation with .. should fail",
			xpath:     "../part",
			shouldErr: true,
			errorMsg:  "selector xpath cannot use parent navigation",
		},
		{
			name:      "parent navigation with .. in middle should fail",
			xpath:     "parts/../part",
			shouldErr: true,
			errorMsg:  "selector xpath cannot use parent navigation",
		},
		{
			name:      "parent navigation with parent:: should fail",
			xpath:     "parent::part",
			shouldErr: true,
			errorMsg:  "selector xpath cannot use parent navigation",
		},
		{
			name:      "valid element selection should pass",
			xpath:     "parts/part",
			shouldErr: false,
		},
		{
			name:      "valid descendant-or-self selection should pass",
			xpath:     ".//part",
			shouldErr: false,
		},
		{
			name:      "valid child axis should pass",
			xpath:     "child::part",
			shouldErr: false,
		},
		{
			name:      "valid descendant-or-self prefix should pass with whitespace",
			xpath:     ".// part",
			shouldErr: false,
		},
		{
			name:      "valid with wildcard should pass",
			xpath:     "parts/*",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSelectorXPath(tt.xpath)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("validateSelectorXPath(%q) should have failed but succeeded", tt.xpath)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
				}
			} else if err != nil {
				t.Errorf("validateSelectorXPath(%q) should have succeeded but got error: %v", tt.xpath, err)
			}
		})
	}
}

// TestSelectorXPathInIdentityConstraint tests selector validation in identity constraints.
func TestSelectorXPathInIdentityConstraint(t *testing.T) {
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[model.QName]model.Type),
		ElementDecls:    make(map[model.QName]*model.ElementDecl),
	}

	complexType := &model.ComplexType{
		QName: model.QName{
			Namespace: "http://example.com",
			Local:     "PurchaseReportType",
		},
	}
	complexType.SetContent(&model.ElementContent{
		Particle: &model.ModelGroup{
			Kind:      model.Sequence,
			MinOccurs: occurs.OccursFromInt(1),
			MaxOccurs: occurs.OccursFromInt(1),
		},
	})
	complexTypeQName := model.QName{
		Namespace: "http://example.com",
		Local:     "PurchaseReportType",
	}
	schema.TypeDefs[complexTypeQName] = complexType

	// test invalid selector - attribute selection
	elementDecl := &model.ElementDecl{
		Name: model.QName{
			Namespace: "http://example.com",
			Local:     "purchaseReport",
		},
		Type: complexType,
		Constraints: []*model.IdentityConstraint{
			{
				Name: "partKey",
				Type: model.KeyConstraint,
				Selector: model.Selector{
					XPath: "@number", // invalid - selects attribute
				},
				Fields: []model.Field{
					{XPath: "@number"},
				},
			},
		},
	}

	elementQName := model.QName{
		Namespace: "http://example.com",
		Local:     "purchaseReport",
	}

	err := validateElementDeclStructure(schema, elementQName, elementDecl)
	if err == nil {
		t.Error("validateElementDeclStructure should have failed for attribute selector")
	} else if !strings.Contains(err.Error(), "selector xpath cannot select attributes") {
		t.Errorf("Expected error to mention attribute selection, got: %v", err)
	}

	// test invalid selector - text node selection
	elementDecl2 := &model.ElementDecl{
		Name: model.QName{
			Namespace: "http://example.com",
			Local:     "purchaseReport2",
		},
		Type: complexType,
		Constraints: []*model.IdentityConstraint{
			{
				Name: "partKey",
				Type: model.KeyConstraint,
				Selector: model.Selector{
					XPath: "child::text()", // invalid - selects text
				},
				Fields: []model.Field{
					{XPath: "@number"},
				},
			},
		},
	}

	elementQName2 := model.QName{
		Namespace: "http://example.com",
		Local:     "purchaseReport2",
	}

	err = validateElementDeclStructure(schema, elementQName2, elementDecl2)
	if err == nil {
		t.Error("validateElementDeclStructure should have failed for text node selector")
	} else if !strings.Contains(err.Error(), "selector xpath cannot select text nodes") {
		t.Errorf("Expected error to mention text node selection, got: %v", err)
	}

	// test valid selector - element selection
	elementDecl3 := &model.ElementDecl{
		Name: model.QName{
			Namespace: "http://example.com",
			Local:     "purchaseReport3",
		},
		Type: complexType,
		Constraints: []*model.IdentityConstraint{
			{
				Name: "partKey",
				Type: model.KeyConstraint,
				Selector: model.Selector{
					XPath: "parts/part", // valid - selects elements
				},
				Fields: []model.Field{
					{XPath: "@number"},
				},
			},
		},
	}

	elementQName3 := model.QName{
		Namespace: "http://example.com",
		Local:     "purchaseReport3",
	}

	err = validateElementDeclStructure(schema, elementQName3, elementDecl3)
	if err != nil {
		t.Errorf("validateElementDeclStructure should have passed for valid element selector, got error: %v", err)
	}
}

// TestValidateFieldXPathDirect tests validateFieldXPath directly.
func TestValidateFieldXPathDirect(t *testing.T) {
	tests := []struct {
		name      string
		xpath     string
		errorMsg  string
		shouldErr bool
	}{
		{
			name:      "empty xpath should fail",
			xpath:     "",
			shouldErr: true,
			errorMsg:  "field xpath cannot be empty",
		},
		{
			name:      "whitespace only should fail",
			xpath:     "   ",
			shouldErr: true,
			errorMsg:  "field xpath cannot be empty",
		},
		{
			name:      "wildcard alone should succeed",
			xpath:     "*",
			shouldErr: false,
		},
		{
			name:      "wildcard with child axis should succeed",
			xpath:     "child::*",
			shouldErr: false,
		},
		{
			name:      "wildcard with descendant-or-self prefix should succeed",
			xpath:     ".//*",
			shouldErr: false,
		},
		{
			name:      "wildcard with descendant-or-self prefix should succeed with whitespace",
			xpath:     ".// *",
			shouldErr: false,
		},
		{
			name:      "wildcard in path should succeed",
			xpath:     "part/*",
			shouldErr: false,
		},
		{
			name:      "wildcard at end should succeed",
			xpath:     "part/*",
			shouldErr: false,
		},
		{
			name:      "parent axis should fail",
			xpath:     "parent::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'parent::'",
		},
		{
			name:      "ancestor axis should fail",
			xpath:     "ancestor::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'ancestor::'",
		},
		{
			name:      "ancestor-or-self axis should fail",
			xpath:     "ancestor-or-self::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'ancestor-or-self::'",
		},
		{
			name:      "following axis should fail",
			xpath:     "following::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'following::'",
		},
		{
			name:      "following-sibling axis should fail",
			xpath:     "following-sibling::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'following-sibling::'",
		},
		{
			name:      "preceding axis should fail",
			xpath:     "preceding::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'preceding::'",
		},
		{
			name:      "preceding-sibling axis should fail",
			xpath:     "preceding-sibling::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'preceding-sibling::'",
		},
		{
			name:      "namespace axis should fail",
			xpath:     "namespace::prefix",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'namespace::'",
		},
		{
			name:      "valid attribute should pass",
			xpath:     "@number",
			shouldErr: false,
		},
		{
			name:      "valid child element should pass",
			xpath:     "part",
			shouldErr: false,
		},
		{
			name:      "valid child axis should pass",
			xpath:     "child::part",
			shouldErr: false,
		},
		{
			name:      "valid descendant-or-self prefix should pass",
			xpath:     ".//part",
			shouldErr: false,
		},
		{
			name:      "valid descendant-or-self prefix should pass with whitespace",
			xpath:     ".// part",
			shouldErr: false,
		},
		{
			name:      "valid attribute axis should pass",
			xpath:     "attribute::number",
			shouldErr: false,
		},
		{
			name:      "valid child element attribute should pass",
			xpath:     "part/@id",
			shouldErr: false,
		},
		{
			name:      "valid path with child element should pass",
			xpath:     "parts/part",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFieldXPath(tt.xpath)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("validateFieldXPath(%q) should have failed but succeeded", tt.xpath)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
				}
			} else if err != nil {
				t.Errorf("validateFieldXPath(%q) should have succeeded but got error: %v", tt.xpath, err)
			}
		})
	}
}
