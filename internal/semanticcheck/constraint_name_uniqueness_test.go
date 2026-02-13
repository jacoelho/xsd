package semanticcheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestDuplicateConstraintNameDirect(t *testing.T) {
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
					XPath: "parts/part",
				},
				Fields: []model.Field{
					{XPath: "@number"},
				},
			},
			{
				Name: "partKey", // duplicate name
				Type: model.KeyConstraint,
				Selector: model.Selector{
					XPath: "parts/part",
				},
				Fields: []model.Field{
					{XPath: "@id"},
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
		t.Error("validateElementDeclStructure should have failed for duplicate constraint names")
	} else {
		if !strings.Contains(err.Error(), "duplicate") {
			t.Errorf("Expected error to mention 'duplicate', got: %v", err)
		}
		if !strings.Contains(err.Error(), "partKey") {
			t.Errorf("Expected error to mention constraint name 'partKey', got: %v", err)
		}
	}

	// test with unique constraint names (should pass)
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
					XPath: "parts/part",
				},
				Fields: []model.Field{
					{XPath: "@number"},
				},
			},
			{
				Name: "regionKey", // different name - should pass
				Type: model.UniqueConstraint,
				Selector: model.Selector{
					XPath: "regions/region",
				},
				Fields: []model.Field{
					{XPath: "@code"},
				},
			},
		},
	}

	elementQName2 := model.QName{
		Namespace: "http://example.com",
		Local:     "purchaseReport2",
	}

	err = validateElementDeclStructure(schema, elementQName2, elementDecl2)
	if err != nil {
		t.Errorf("validateElementDeclStructure should have passed for unique constraint names, got error: %v", err)
	}
}
