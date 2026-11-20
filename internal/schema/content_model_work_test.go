package schema

func unlimitedContentModelWork(int) error { return nil }

func unlimitedContentModelAnalysis(rt *schemaBuild) *contentModelAnalysis {
	analysis, err := newContentModelAnalysis(rt, unlimitedContentModelWork)
	if err != nil {
		panic(err)
	}
	return analysis
}

// allCompiledModels enters every model of an n-model compiled table, so the
// published table keeps compiled-table positions.
func allCompiledModels(n int) []int {
	positions := make([]int, n)
	for i := range positions {
		positions[i] = i
	}
	return positions
}

func testCompiledModelReads(models []compiledModel) []compiledModelRead {
	reads, mayHaveDenseRows, err := newCompiledModelReads(models, allCompiledModels(len(models)), unlimitedContentModelWork)
	if err != nil {
		panic(err)
	}
	if mayHaveDenseRows {
		budget := defaultDenseIndexBudget()
		addDenseDFAIndexes(reads, &budget)
	}
	return reads
}
