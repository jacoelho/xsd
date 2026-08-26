package compile_test

import "github.com/jacoelho/xsd/internal/runtime"

func unlimitedContentModelWork(int) error { return nil }

func publishSchema(build *runtime.SchemaBuild) (*runtime.Schema, error) {
	return runtime.PublishSchema(build, unlimitedContentModelWork)
}
