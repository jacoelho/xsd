package schema

func unlimitedContentModelWork(int) error { return nil }

func unlimitedContentModelAnalysis(rt *schemaBuild) *ContentModelAnalysis {
	analysis, err := NewContentModelAnalysis(rt, unlimitedContentModelWork)
	if err != nil {
		panic(err)
	}
	return analysis
}

func testCompiledModelReads(models []CompiledModel) []compiledModelRead {
	reads, err := newCompiledModelReads(models, unlimitedContentModelWork)
	if err != nil {
		panic(err)
	}
	return reads
}
