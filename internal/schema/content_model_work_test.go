package schema

import (
	"math"
	"slices"
)

func unlimitedContentModelWork(int) error { return nil }

func unlimitedContentModelAnalysis(rt *schemaBuild) *contentModelAnalysis {
	analysis, err := newContentModelAnalysis(rt, unlimitedContentModelWork)
	if err != nil {
		panic(err)
	}
	return analysis
}

// testProjectedModel projects model to its published form without indexing
// it, naming each element declaration by its build ID.
func testProjectedModel(model compiledModel) compiledModelRead {
	read, _ := testProjectedModelRun(model)
	return read
}

func testProjectedModelRun(model compiledModel) (compiledModelRead, []compiledModelEdgeRead) {
	read, run, err := newCompiledModelRead(model, unlimitedContentModelWork, identityElements(math.MaxInt32))
	if err != nil {
		panic(err)
	}
	return read, run
}

// newBuildNumberedDFARowIndexAnalysis returns an analysis of tables that name
// each declaration of rt by its build ID.
func newBuildNumberedDFARowIndexAnalysis(rt *schemaBuild, work contentModelWork, retained *workBudget) *dfaRowIndexAnalysis {
	identity := identityElements(len(rt.Elements))
	return newDFARowIndexAnalysis(rt, &identity, work, retained)
}

// testIndexedModels projects and indexes models as compilation does.
func testIndexedModels(rt *schemaBuild, models ...compiledModel) []compiledModelRead {
	analysis := newBuildNumberedDFARowIndexAnalysis(rt, unlimitedContentModelWork, testDependencyWork())
	reads := make([]compiledModelRead, len(models))
	for i, model := range models {
		read, run := testProjectedModelRun(model)
		reads[i] = read
		if err := analysis.indexModel(&reads[i], run); err != nil {
			panic(err)
		}
	}
	return reads
}

// testPublishedReads returns every model of a compiled table in published
// form, sharing its tables as publication does, without the dense indexes
// publication adds.
func testPublishedReads(models ...compiledModelRead) []compiledModelRead {
	return slices.Clone(models)
}

// testCompiledModelReads publishes every model of a compiled table, with the
// dense indexes publication adds.
func testCompiledModelReads(models []compiledModelRead) []compiledModelRead {
	reads := testPublishedReads(models...)
	budget := defaultDenseIndexBudget()
	addDenseDFAIndexes(reads, &budget)
	return reads
}
