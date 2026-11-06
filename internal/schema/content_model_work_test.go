package schema

func unlimitedContentModelWork(int) error { return nil }

func unlimitedContentModelAnalysis(rt *schemaBuild) *contentModelAnalysis {
	analysis, err := newContentModelAnalysis(rt, unlimitedContentModelWork)
	if err != nil {
		panic(err)
	}
	return analysis
}

// allContentModelIDs enters every model, so the published table keeps source
// IDs.
func allContentModelIDs(n int) []contentModelID {
	ids := make([]contentModelID, n)
	for i := range ids {
		ids[i] = contentModelID(i)
	}
	return ids
}

func testCompiledModelReads(models []compiledModel) []compiledModelRead {
	reads, mayHaveDenseRows, err := newCompiledModelReads(models, allContentModelIDs(len(models)), unlimitedContentModelWork)
	if err != nil {
		panic(err)
	}
	if mayHaveDenseRows {
		budget := defaultDenseIndexBudget()
		addDenseDFAIndexes(reads, &budget)
	}
	return reads
}
