package schema

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
	read, err := newCompiledModelRead(model, unlimitedContentModelWork)
	if err != nil {
		panic(err)
	}
	return read
}

// testIndexedModels projects and indexes models as compilation does.
func testIndexedModels(rt *schemaBuild, models ...compiledModel) []compiledModelRead {
	analysis := newDFARowIndexAnalysis(rt, unlimitedContentModelWork, testDependencyWork())
	reads := make([]compiledModelRead, len(models))
	for i, model := range models {
		reads[i] = testProjectedModel(model)
		if err := analysis.indexModel(&reads[i]); err != nil {
			panic(err)
		}
	}
	return reads
}

// testPublishedReads publishes every model of a compiled table without the
// dense indexes publication adds.
func testPublishedReads(models ...compiledModelRead) []compiledModelRead {
	positions := make([]int, len(models))
	for i := range positions {
		positions[i] = i
	}
	return publishedContentModels(models, positions)
}

// testCompiledModelReads publishes every model of a compiled table, with the
// dense indexes publication adds.
func testCompiledModelReads(models []compiledModelRead) []compiledModelRead {
	reads := testPublishedReads(models...)
	budget := defaultDenseIndexBudget()
	addDenseDFAIndexes(reads, &budget)
	return reads
}
