package xsd

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/jacoelho/xsd/internal/preprocessor"
)

type sourceEntry struct {
	fsys     fs.FS
	resolver preprocessor.SchemaResolver
	location string
}

func newSourceEntry(fsys fs.FS, location string) (sourceEntry, error) {
	if fsys == nil {
		return sourceEntry{}, fmt.Errorf("source set: nil fs")
	}
	location = strings.TrimSpace(location)
	if location == "" {
		return sourceEntry{}, fmt.Errorf("source set: empty location")
	}
	return sourceEntry{fsys: fsys, location: location}, nil
}
