package compiler

import (
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/parser"
)

// DirectiveLoadConfig describes the root-owned callbacks needed to resolve one directive target.
type DirectiveLoadConfig[K comparable] struct {
	Resolve       func() (io.ReadCloser, string, error)
	IsNotFound    func(error) bool
	Key           func(string) K
	AlreadyMerged func(K) bool
	IsLoading     func(K) bool
	OnLoading     func(K)
	Load          func(io.ReadCloser, string, K) (*parser.Schema, error)
	Close         func(io.Closer, string) error
	AllowMissing  bool
}

// Load resolves one directive target and classifies it as loaded, deferred, or skipped.
func Load[K comparable](cfg DirectiveLoadConfig[K]) (LoadResult[K], error) {
	if cfg.Resolve == nil {
		return LoadResult[K]{}, fmt.Errorf("missing resolve callback")
	}
	if cfg.Key == nil {
		return LoadResult[K]{}, fmt.Errorf("missing key callback")
	}
	if cfg.Load == nil {
		return LoadResult[K]{}, fmt.Errorf("missing load callback")
	}
	if cfg.Close == nil {
		return LoadResult[K]{}, fmt.Errorf("missing close callback")
	}

	doc, systemID, err := cfg.Resolve()
	if err != nil {
		if cfg.AllowMissing && cfg.IsNotFound != nil && cfg.IsNotFound(err) {
			return LoadResult[K]{Status: StatusSkippedMissing}, nil
		}
		return LoadResult[K]{}, err
	}

	target := cfg.Key(systemID)
	if cfg.AlreadyMerged != nil && cfg.AlreadyMerged(target) {
		if closeErr := cfg.Close(doc, systemID); closeErr != nil {
			return LoadResult[K]{}, closeErr
		}
		return LoadResult[K]{
			Target: target,
			Status: StatusDeferred,
		}, nil
	}
	if cfg.IsLoading != nil && cfg.IsLoading(target) {
		if closeErr := cfg.Close(doc, systemID); closeErr != nil {
			return LoadResult[K]{}, closeErr
		}
		if cfg.OnLoading != nil {
			cfg.OnLoading(target)
		}
		return LoadResult[K]{
			Target: target,
			Status: StatusDeferred,
		}, nil
	}

	schema, err := cfg.Load(doc, systemID, target)
	if err != nil {
		return LoadResult[K]{}, err
	}
	return LoadResult[K]{
		Schema: schema,
		Target: target,
		Status: StatusLoaded,
	}, nil
}
