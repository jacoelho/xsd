package semanticcheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestDuplicateConstraintNameDirect(t *testing.T) {
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
		ElementDecls:    make(map[types.QName]*types.ElementDecl),
	}

	complexType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "PurchaseReportType",
		},
	}
	complexType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind:      types.Sequence,
			MinOccurs: types.OccursFromInt(1),
			MaxOccurs: types.OccursFromInt(1),
		},
	})
	complexTypeQName := types.QName{
		Namespace: "http://example.com",
		Local:     "PurchaseReportType",
	}
	schema.TypeDefs[complexTypeQName] = complexType

	elementDecl := &types.ElementDecl{
		Name: types.QName{
			Namespace: "http://example.com",
			Local:     "purchaseReport",
		},
		Type: complexType,
		Constraints: []*types.IdentityConstraint{
			{
				Name: "partKey",
				Type: types.KeyConstraint,
				Selector: types.Selector{
					XPath: "parts/part",
				},
				Fields: []types.Field{
					{XPath: "@number"},
				},
			},
			{
				Name: "partKey", // duplicate name
				Type: types.KeyConstraint,
				Selector: types.Selector{
					XPath: "parts/part",
				},
				Fields: []types.Field{
					{XPath: "@id"},
				},
			},
		},
	}

	elementQName := types.QName{
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
	elementDecl2 := &types.ElementDecl{
		Name: types.QName{
			Namespace: "http://example.com",
			Local:     "purchaseReport2",
		},
		Type: complexType,
		Constraints: []*types.IdentityConstraint{
			{
				Name: "partKey",
				Type: types.KeyConstraint,
				Selector: types.Selector{
					XPath: "parts/part",
				},
				Fields: []types.Field{
					{XPath: "@number"},
				},
			},
			{
				Name: "regionKey", // different name - should pass
				Type: types.UniqueConstraint,
				Selector: types.Selector{
					XPath: "regions/region",
				},
				Fields: []types.Field{
					{XPath: "@code"},
				},
			},
		},
	}

	elementQName2 := types.QName{
		Namespace: "http://example.com",
		Local:     "purchaseReport2",
	}

	err = validateElementDeclStructure(schema, elementQName2, elementDecl2)
	if err != nil {
		t.Errorf("validateElementDeclStructure should have passed for unique constraint names, got error: %v", err)
	}
}
