package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/runtimetest"
)

func newRuntimeSchema(tb testing.TB) *runtime.Schema {
	tb.Helper()
	return runtimetest.EmptySchema(tb)
}

func setRuntimeRootPolicy(tb testing.TB, schema *runtime.Schema, v runtime.RootPolicy) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetRootPolicy(v)
	})
}

func setRuntimeTypes(tb testing.TB, schema *runtime.Schema, v []runtime.Type) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetTypes(v)
	})
}

func setRuntimeAncestors(tb testing.TB, schema *runtime.Schema, v runtime.TypeAncestors) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetAncestors(v)
	})
}

func setRuntimeComplexTypes(tb testing.TB, schema *runtime.Schema, v []runtime.ComplexType) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetComplexTypes(v)
	})
}

func setRuntimeElements(tb testing.TB, schema *runtime.Schema, v []runtime.Element) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetElements(v)
	})
}

func setRuntimeAttributes(tb testing.TB, schema *runtime.Schema, v []runtime.Attribute) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetAttributes(v)
	})
}

func setRuntimeAttrIndex(tb testing.TB, schema *runtime.Schema, v runtime.ComplexAttrIndex) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetAttrIndex(v)
	})
}

func setRuntimeValidators(tb testing.TB, schema *runtime.Schema, v runtime.ValidatorsBundle) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetValidators(v)
	})
}

func setRuntimeNotations(tb testing.TB, schema *runtime.Schema, v []runtime.SymbolID) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetNotations(v)
	})
}

func setRuntimePredefinedSymbols(tb testing.TB, schema *runtime.Schema, v runtime.PredefinedSymbols) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetPredefinedSymbols(v)
	})
}

func setRuntimePredefinedNamespaces(tb testing.TB, schema *runtime.Schema, v runtime.PredefinedNamespaces) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetPredefinedNamespaces(v)
	})
}

func setRuntimeModels(tb testing.TB, schema *runtime.Schema, v runtime.ModelsBundle) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetModels(v)
	})
}

func setRuntimeDFAModels(tb testing.TB, schema *runtime.Schema, v []runtime.DFAModel) {
	tb.Helper()
	models := schema.ModelBundle()
	models.DFA = v
	setRuntimeModels(tb, schema, models)
}

func setRuntimeNFAModels(tb testing.TB, schema *runtime.Schema, v []runtime.NFAModel) {
	tb.Helper()
	models := schema.ModelBundle()
	models.NFA = v
	setRuntimeModels(tb, schema, models)
}

func setRuntimeAllModels(tb testing.TB, schema *runtime.Schema, v []runtime.AllModel) {
	tb.Helper()
	models := schema.ModelBundle()
	models.All = v
	setRuntimeModels(tb, schema, models)
}

func setRuntimeAllSubstitutions(tb testing.TB, schema *runtime.Schema, v []runtime.ElemID) {
	tb.Helper()
	models := schema.ModelBundle()
	models.AllSubst = v
	setRuntimeModels(tb, schema, models)
}

func setRuntimeWildcards(tb testing.TB, schema *runtime.Schema, v []runtime.WildcardRule) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetWildcards(v)
	})
}

func setRuntimeIdentityConstraints(tb testing.TB, schema *runtime.Schema, v []runtime.IdentityConstraint) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetIdentityConstraints(v)
	})
}

func setRuntimeElementIdentityConstraints(tb testing.TB, schema *runtime.Schema, v []runtime.ICID) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetElementIdentityConstraints(v)
	})
}

func setRuntimeIdentitySelectors(tb testing.TB, schema *runtime.Schema, v []runtime.PathID) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetIdentitySelectors(v)
	})
}

func setRuntimeIdentityFields(tb testing.TB, schema *runtime.Schema, v []runtime.PathID) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetIdentityFields(v)
	})
}

func setRuntimePaths(tb testing.TB, schema *runtime.Schema, v []runtime.PathProgram) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetPaths(v)
	})
}

func setRuntimeGlobalTypes(tb testing.TB, schema *runtime.Schema, v []runtime.TypeID) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetGlobalTypes(v)
	})
}

func setRuntimeGlobalElements(tb testing.TB, schema *runtime.Schema, v []runtime.ElemID) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetGlobalElements(v)
	})
}

func setRuntimeGlobalAttributes(tb testing.TB, schema *runtime.Schema, v []runtime.AttrID) {
	tb.Helper()
	runtimetest.Mutate(tb, schema, func(a *runtime.Assembler) error {
		return a.SetGlobalAttributes(v)
	})
}
