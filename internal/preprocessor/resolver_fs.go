package preprocessor

import (
	"fmt"
	"io"
	"io/fs"
)

// FSResolver resolves schema documents from an fs.FS with strict path validation.
type FSResolver struct {
	fsys fs.FS
}

// NewFSResolver creates a resolver backed by the provided filesystem.
func NewFSResolver(fsys fs.FS) *FSResolver {
	return &FSResolver{fsys: fsys}
}

// Resolve implements Resolver.
func (r *FSResolver) Resolve(req ResolveRequest) (io.ReadCloser, string, error) {
	if r == nil || r.fsys == nil {
		return nil, "", fmt.Errorf("no filesystem configured")
	}
	if req.SchemaLocation == "" {
		return nil, "", fs.ErrNotExist
	}
	systemID, err := resolveSystemID(req.BaseSystemID, req.SchemaLocation)
	if err != nil {
		return nil, "", err
	}
	f, err := r.fsys.Open(systemID)
	if err != nil {
		return nil, "", err
	}
	return f, systemID, nil
}
