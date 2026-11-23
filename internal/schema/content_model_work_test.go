package schema

import "slices"

func unlimitedContentModelWork(int) error { return nil }

func unlimitedContentModelAnalysis(rt *schemaBuild) *contentModelAnalysis {
	analysis, err := newContentModelAnalysis(rt, unlimitedContentModelWork)
	if err != nil {
		panic(err)
	}
	return analysis
}

// testProjectedModel projects model to its published form without indexing
// it.
func testProjectedModel(model compiledModel) compiledModelRead {
	read, _ := testProjectedModelRun(model)
	return read
}

func testProjectedModelRun(model compiledModel) (compiledModelRead, []compiledModelEdgeRead) {
	read, run, err := newCompiledModelRead(model, unlimitedContentModelWork)
	if err != nil {
		panic(err)
	}
	return read, run
}

// testIndexedModels projects and indexes models as compilation does.
func testIndexedModels(rt *schemaBuild, models ...compiledModel) []compiledModelRead {
	analysis := newDFARowIndexAnalysis(rt, unlimitedContentModelWork, testDependencyWork())
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
// form, sharing its tables as publication does when every element
// declaration keeps its ID, without the dense indexes publication adds.
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
