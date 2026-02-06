package source

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// ResolveKind identifies the kind of schema resolution request.
type ResolveKind uint8

const (
	ResolveInclude ResolveKind = iota
	ResolveImport
)

// ResolveRequest describes a schema resolution request.
type ResolveRequest struct {
	BaseSystemID   string
	SchemaLocation string
	ImportNS       []byte
	Kind           ResolveKind
}

// Resolver resolves schema documents into readers and canonical system IDs.
type Resolver interface {
	Resolve(req ResolveRequest) (doc io.ReadCloser, systemID string, err error)
}

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

func resolveSystemID(baseSystemID, schemaLocation string) (string, error) {
	if strings.Contains(schemaLocation, "\\") {
		return "", fmt.Errorf("schema location contains backslash: %q", schemaLocation)
	}
	if strings.HasPrefix(schemaLocation, "/") {
		return "", fmt.Errorf("schema location must be relative: %q", schemaLocation)
	}
	if schemaLocation == "" {
		return "", fmt.Errorf("schema location is empty")
	}
	if baseSystemID != "" && strings.Contains(baseSystemID, "\\") {
		return "", fmt.Errorf("base system ID contains backslash: %q", baseSystemID)
	}
	segments := strings.Split(schemaLocation, "/")
	if len(segments) == 0 {
		return "", fmt.Errorf("schema location is empty")
	}
	if slices.Contains(segments, "") {
		return "", fmt.Errorf("invalid schema location segment: %q", schemaLocation)
	}
	canonical := path.Clean(schemaLocation)
	if canonical == "." {
		return "", fmt.Errorf("schema location is empty")
	}
	if baseSystemID == "" {
		if strings.HasPrefix(canonical, "../") || canonical == ".." {
			return "", fmt.Errorf("schema location escapes root: %q", schemaLocation)
		}
		return canonical, nil
	}
	baseDir := baseDirSystemID(baseSystemID)
	if baseDir == "" {
		if strings.HasPrefix(canonical, "../") || canonical == ".." {
			return "", fmt.Errorf("schema location escapes root: %q", schemaLocation)
		}
		return canonical, nil
	}
	joined := path.Clean(baseDir + "/" + schemaLocation)
	if joined == "." {
		return "", fmt.Errorf("schema location is empty")
	}
	if strings.HasPrefix(joined, "../") || joined == ".." {
		return "", fmt.Errorf("schema location escapes root: %q", schemaLocation)
	}
	return joined, nil
}

func baseDirSystemID(systemID string) string {
	if systemID == "" {
		return ""
	}
	if strings.Contains(systemID, "\\") {
		return ""
	}
	idx := strings.LastIndex(systemID, "/")
	if idx == -1 {
		return ""
	}
	return systemID[:idx]
}
