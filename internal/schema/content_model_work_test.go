package schema

func unlimitedContentModelWork(int) error { return nil }

func unlimitedContentModelAnalysis(rt *schemaBuild) *ContentModelAnalysis {
	analysis, err := NewContentModelAnalysis(rt, unlimitedContentModelWork)
	if err != nil {
		panic(err)
	}
	return analysis
}

// allContentModelIDs enters every model, so the published table keeps source
// IDs.
func allContentModelIDs(n int) []ContentModelID {
	ids := make([]ContentModelID, n)
	for i := range ids {
		ids[i] = ContentModelID(i)
	}
	return ids
}

func testCompiledModelReads(models []CompiledModel) []compiledModelRead {
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
