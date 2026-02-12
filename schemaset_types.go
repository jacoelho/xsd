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
func NewSchemaSet(opts ...LoadOptions) *SchemaSet {
	loadOpts := NewLoadOptions()
	if len(opts) > 0 {
		loadOpts = opts[0]
	}
	return &SchemaSet{loadOpts: loadOpts}
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
	if fsys == nil {
		return fmt.Errorf("schema set: nil fs")
	}
	location = strings.TrimSpace(location)
	if location == "" {
		return fmt.Errorf("schema set: empty location")
	}
	s.entries = append(s.entries, schemaSetEntry{
		fsys:     fsys,
		location: location,
	})
	return nil
}
