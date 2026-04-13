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
		return compiler.Root{}, fmt.Errorf("source set: nil fs")
	}
	location = strings.TrimSpace(location)
	if location == "" {
		return compiler.Root{}, fmt.Errorf("source set: empty location")
	}
	return compiler.Root{FS: fsys, Location: location}, nil
}

func newCompileRequest(roots []compiler.Root, opts []CompileOption) (compileRequest, error) {
	source, build, err := resolveCompileOptions(opts)
	if err != nil {
		return compileRequest{}, err
	}
	return compileRequest{
		roots:            cloneCompileRoots(roots),
		source:           source,
		build:            build,
		validateDefaults: defaultResolvedValidateOptions(),
	}, nil
}

func newSourceCompileRequest(roots []compiler.Root, sourceCfg sourceConfig) (compileRequest, error) {
	if len(roots) == 0 {
		return compileRequest{}, fmt.Errorf("no schema roots added")
	}
	source, err := sourceCfg.withDefaults()
	if err != nil {
		return compileRequest{}, err
	}
	return compileRequest{
		roots:            cloneCompileRoots(roots),
		source:           source,
		validateDefaults: defaultResolvedValidateOptions(),
	}, nil
}

func newBuildCompileRequest(opts []BuildOption) compileRequest {
	return compileRequest{
		build:            resolveBuildOptions(opts),
		validateDefaults: defaultResolvedValidateOptions(),
	}
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

func newValidateRequest(opts []ValidateOption) (validateRequest, error) {
	resolved, err := resolveValidateOptions(opts)
	if err != nil {
		return validateRequest{}, err
	}
	return validateRequest{options: resolved}, nil
}

func (r validateRequest) newValidator(rt *runtime.Schema) *Validator {
	return newValidator(rt, r.options)
}
