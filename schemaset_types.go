package xsd

import (
	"fmt"
	"io/fs"
	"strings"
)

type schemaSetEntry struct {
	fsys     fs.FS
	location string
}

// SchemaSet owns schema sources and compiles them into a runtime schema.
type SchemaSet struct {
	entries  []schemaSetEntry
	loadOpts LoadOptions
}

// NewSchemaSet creates an empty schema set.
func NewSchemaSet() *SchemaSet {
	return &SchemaSet{loadOpts: NewLoadOptions()}
}

// WithLoadOptions replaces schema-set load options.
func (s *SchemaSet) WithLoadOptions(opts LoadOptions) *SchemaSet {
	if s == nil {
		return nil
	}
	s.loadOpts = opts
	return s
}

// AddFS adds one schema root location from fsys.
func (s *SchemaSet) AddFS(fsys fs.FS, location string) error {
	if s == nil {
		return fmt.Errorf("schema set: nil set")
	}
	entry, err := newSchemaSetEntry(fsys, location)
	if err != nil {
		return err
	}
	s.entries = append(s.entries, entry)
	return nil
}

func newSchemaSetEntry(fsys fs.FS, location string) (schemaSetEntry, error) {
	if fsys == nil {
		return schemaSetEntry{}, fmt.Errorf("schema set: nil fs")
	}
	location = strings.TrimSpace(location)
	if location == "" {
		return schemaSetEntry{}, fmt.Errorf("schema set: empty location")
	}
	return schemaSetEntry{fsys: fsys, location: location}, nil
}
