package xsd

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/runtime"
)

type compileRequest struct {
	roots            []compiler.Root
	source           resolvedSourceOptions
	build            resolvedBuildOptions
	validateDefaults resolvedValidateOptions
}

func newCompileRoot(fsys fs.FS, location string) (compiler.Root, error) {
	if fsys == nil {
		return compiler.Root{}, fmt.Errorf("nil fs")
	}
	location = strings.TrimSpace(location)
	if location == "" {
		return compiler.Root{}, fmt.Errorf("empty location")
	}
	return compiler.Root{FS: fsys, Location: location}, nil
}

func newCompileRequest(roots []compiler.Root, config CompileConfig) (compileRequest, error) {
	if len(roots) == 0 {
		return compileRequest{}, fmt.Errorf("no schema roots")
	}
	source, build, validate, err := config.withDefaults()
	if err != nil {
		return compileRequest{}, err
	}
	return compileRequest{
		roots:            cloneCompileRoots(roots),
		source:           source,
		build:            build,
		validateDefaults: validate,
	}, nil
}

func (r compileRequest) prepare() (*compiler.Prepared, error) {
	return compiler.PrepareRoots(compiler.LoadConfig{
		Roots:                       cloneCompileRoots(r.roots),
		AllowMissingImportLocations: r.source.allowMissingImportLocations,
		SchemaParseOptions:          r.source.schemaLimits.options(),
	})
}

func (r compileRequest) compile() (*Schema, error) {
	prepared, err := r.prepare()
	if err != nil {
		return nil, err
	}
	return r.buildPrepared(prepared)
}

func (r compileRequest) buildPrepared(prepared *compiler.Prepared) (*Schema, error) {
	return buildSchema(prepared, r.build, r.validateDefaults)
}

func cloneCompileRoots(roots []compiler.Root) []compiler.Root {
	return slices.Clone(roots)
}

type validateRequest struct {
	options resolvedValidateOptions
}

func newValidateRequest(config ValidateConfig) (validateRequest, error) {
	resolved, err := config.withDefaults()
	if err != nil {
		return validateRequest{}, err
	}
	return validateRequest{options: resolved}, nil
}

func (r validateRequest) newValidator(rt *runtime.Schema) *Validator {
	return newValidator(rt, r.options)
}
