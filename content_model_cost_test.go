package xsd_test

import (
	"errors"
	"flag"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

var contentModelCostModels = flag.Int("content-model-cost.models", 500, "generated content models TestChoiceWithoutBranchesCostsNoMoreThanItsElementTwin compares")

// oracleCostCeiling bounds every limit the cost comparison searches, so a
// twin that needs more fails fast.
const oracleCostCeiling = 1 << 22

type oracleCostLimit struct {
	name, names string
	set         func(*xsd.CompileOptions, int)
}

var oracleCostLimits = []oracleCostLimit{
	{name: "MaxContentModelStates", names: "content model DFA state limit", set: func(o *xsd.CompileOptions, n int) { o.MaxContentModelStates = n }},
	{name: "MaxContentModelAnalysisSteps", names: "MaxContentModelAnalysisSteps", set: func(o *xsd.CompileOptions, n int) { o.MaxContentModelAnalysisSteps = n }},
	{name: "MaxSchemaDependencySteps", names: "MaxSchemaDependencySteps", set: func(o *xsd.CompileOptions, n int) { o.MaxSchemaDependencySteps = n }},
}

func TestChoiceWithoutBranchesCostsNoMoreThanItsElementTwin(t *testing.T) {
	t.Parallel()

	const seed = 0xc057
	t.Logf("seed %#x, %d models; model i is the first holding a choice without branches that rand.NewPCG(seed, i) generates", seed, *contentModelCostModels)
	for i := range *contentModelCostModels {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			rng := rand.New(rand.NewPCG(seed, uint64(i))) //nolint:gosec // Reproducible generated models, not security-sensitive randomness.
			generator := &oracleGenerator{rng: rng, costShapes: true}
			for {
				model := generator.model()
				if twin, ok := elementTwin(model); ok {
					checkCostNoMoreThanTwin(t, model.schema(), twin.schema())
					return
				}
			}
		})
	}
}

// elementTwin returns model with one element declaration no other particle
// names added to each choice without branches, and whether model holds such
// a choice.
func elementTwin(model *oracleModel) (*oracleModel, bool) {
	fresh := 0
	var twin func(*oracleParticle) *oracleParticle
	twin = func(p *oracleParticle) *oracleParticle {
		c := *p
		c.children = make([]*oracleParticle, 0, len(p.children)+1)
		for _, child := range p.children {
			c.children = append(c.children, twin(child))
		}
		if p.emptyChoice() {
			c.children = append(c.children, &oracleParticle{kind: oracleElement, namespace: oracleTargetNamespace, local: "y" + strconv.Itoa(fresh), min: 1, max: 1})
			fresh++
		}
		return &c
	}
	var extension *oracleParticle
	if model.extension != nil {
		extension = twin(model.extension)
	}
	base := twin(model.base)
	return newOracleModel(base, extension), fresh != 0
}

func checkCostNoMoreThanTwin(t *testing.T, empty, twin string) {
	t.Helper()
	ample := xsd.CompileOptions{MaxContentModelStates: oracleCostCeiling, MaxContentModelAnalysisSteps: oracleCostCeiling, MaxSchemaDependencySteps: oracleCostCeiling}
	if _, err := xsd.CompileWithOptions(ample, xsd.Bytes("twin.xsd", []byte(twin))); err != nil {
		t.Skipf("the twin does not compile within %d of each limit: %v", oracleCostCeiling, err)
	}
	for _, limit := range oracleCostLimits {
		least := leastLimit(t, twin, ample, limit)
		opts := ample
		limit.set(&opts, least)
		if _, err := xsd.CompileWithOptions(opts, xsd.Bytes("empty.xsd", []byte(empty))); err != nil {
			t.Fatalf("with %s %d, where the element twin compiles: %v\nschema: %s\ntwin: %s", limit.name, least, err, empty, twin)
		}
	}
}

// leastLimit returns the least value of limit at which schema compiles, with
// the other limits at ample. It doubles a bound until schema compiles, then
// bisects below it.
func leastLimit(t *testing.T, schema string, ample xsd.CompileOptions, limit oracleCostLimit) int {
	t.Helper()
	compiles := func(n int) bool {
		opts := ample
		limit.set(&opts, n)
		_, err := xsd.CompileWithOptions(opts, xsd.Bytes("twin.xsd", []byte(schema)))
		if err == nil {
			return true
		}
		if !isSchemaLimit(err) || !strings.Contains(err.Error(), limit.names) {
			t.Fatalf("Compile() with %s %d = %v, want nil or a limit error naming it\nschema: %s", limit.name, n, err, schema)
		}
		return false
	}
	high := 1
	for !compiles(high) {
		if high >= oracleCostCeiling {
			t.Fatalf("the twin needs more than %s %d\nschema: %s", limit.name, oracleCostCeiling, schema)
		}
		high *= 2
	}
	for low := high/2 + 1; low < high; {
		if mid := low + (high-low)/2; compiles(mid) {
			high = mid
		} else {
			low = mid + 1
		}
	}
	return high
}

func isSchemaLimit(err error) bool {
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	return ok && diagnostic.Code() == xsderrors.CodeSchemaLimit
}
