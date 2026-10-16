package schema

import (
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestTargetReferencePlanningChargesOnlyResolvedEdges(t *testing.T) {
	t.Parallel()
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	const root = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:root">
<xs:import namespace="urn:unused:0"/>
<xs:import namespace="urn:imported" schemaLocation="urn:source:imported"/>
<xs:import namespace="urn:unused:1" schemaLocation="urn:source:missing"/>
<xs:include schemaLocation="urn:source:included"/>
<xs:import namespace="urn:unused:2"/>
</xs:schema>`
	resolver := source.Resolver(func(_, location string) (source.Source, error) {
		switch location {
		case "urn:source:imported":
			return source.Bytes("a-imported", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:imported"/>`)), nil
		case "urn:source:included":
			return source.Bytes("b-included", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:root"/>`)), nil
		case "urn:source:missing":
			return source.Source{}, xsderrors.ErrSchemaNotFound
		default:
			return source.Source{}, errors.New("unexpected reference: " + location)
		}
	})
	for _, steps := range []int{2, 1} {
		t.Run(strconv.Itoa(steps), func(t *testing.T) {
			t.Parallel()
			loading := newWorkBudget(dependencyWorkBudget, limits.MaxSchemaDependencySteps)
			graph, err := loadSchemaGraphOwned([]source.Source{
				source.Bytes("z-root", []byte(root)).WithResolver(resolver),
			}, limits, &loading)
			if err != nil {
				t.Fatal(err)
			}
			planning := newWorkBudget(dependencyWorkBudget, steps)
			graph.dependencyWork = &planning
			plan, err := graph.plan()
			if steps == 1 {
				xerr, ok := errors.AsType[*xsderrors.Error](err)
				if !ok || xerr.Category() != xsderrors.CategorySchemaCompile || xerr.Code() != xsderrors.CodeSchemaLimit || xerr.Message() != "compiler work exceeds MaxSchemaDependencySteps (limit 1)" {
					t.Fatalf("plan() error = %v, want schema dependency limit 1", err)
				}
				if len(plan.documents) != 0 {
					t.Fatal("failed planning returned a partial plan")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var names, targets []string
			for _, document := range plan.documents {
				names = append(names, document.doc.name)
				targets = append(targets, document.effectiveTargetNS)
			}
			if !slices.Equal(names, []string{"a-imported", "b-included", "z-root"}) || !slices.Equal(targets, []string{"urn:imported", "urn:root", "urn:root"}) {
				t.Fatalf("plan documents = %v in %v, want ordered imported/include/root contexts", names, targets)
			}
		})
	}
}
