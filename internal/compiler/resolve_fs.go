package compiler

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
)

// FSResolver resolves schema documents from an fs.FS with strict path validation.
type FSResolver struct {
	fsys fs.FS
}

// NewFSResolver creates a resolver backed by the provided filesystem.
func NewFSResolver(fsys fs.FS) *FSResolver {
	return &FSResolver{fsys: fsys}
}

// Resolve implements SchemaResolver.
func (r *FSResolver) Resolve(req ResolveRequest) (io.ReadCloser, string, error) {
	if r == nil || r.fsys == nil {
		return nil, "", fmt.Errorf("no filesystem configured")
	}
	if req.SchemaLocation == "" {
		return nil, "", fs.ErrNotExist
	}
	systemID, err := ResolveSystemID(req.BaseSystemID, req.SchemaLocation)
	if err != nil {
		return nil, "", err
	}
	f, err := r.fsys.Open(systemID)
	if err != nil {
		return nil, "", err
	}
	return f, canonicalFSSystemID(r.fsys, systemID), nil
}

func canonicalFSSystemID(fsys fs.FS, systemID string) string {
	if fsys == nil || systemID == "" || systemID == "." {
		return systemID
	}
	parts := strings.Split(systemID, "/")
	out := make([]string, 0, len(parts))
	dir := "."
	for _, part := range parts {
		entries, err := fs.ReadDir(fsys, dir)
		if err != nil {
			return systemID
		}
		match, ok := canonicalFSSegment(entries, part)
		if !ok {
			return systemID
		}
		out = append(out, match)
		if dir == "." {
			dir = match
		} else {
			dir = path.Join(dir, match)
		}
	}
	return path.Join(out...)
}

func canonicalFSSegment(entries []fs.DirEntry, segment string) (string, bool) {
	for _, entry := range entries {
		if entry.Name() == segment {
			return segment, true
		}
	}
	match := ""
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), segment) {
			continue
		}
		if match != "" {
			return segment, true
		}
		match = entry.Name()
	}
	if match == "" {
		return "", false
	}
	return match, true
}
