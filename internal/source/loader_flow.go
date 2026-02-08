package source

import (
	"io"

	"github.com/jacoelho/xsd/internal/parser"
)

// Load loads and merges a schema graph from location.
// It is fail-stop and requires a configured resolver for root resolution.
func (l *SchemaLoader) Load(location string) (*parser.Schema, error) {
	if err := l.beginLocationLoad(); err != nil {
		return nil, err
	}
	sch, err := l.loadRoot(location)
	if err != nil {
		l.markFailed(err)
		return nil, err
	}
	return sch, nil
}

// loadRoot loads the root schema by resolving the provided location.
func (l *SchemaLoader) loadRoot(location string) (*parser.Schema, error) {
	doc, systemID, err := l.resolve(ResolveRequest{
		BaseSystemID:   "",
		SchemaLocation: location,
		Kind:           ResolveInclude,
	})
	if err != nil {
		return nil, err
	}
	result, err := parseSchemaDocument(doc, systemID, l.config.SchemaParseOptions...)
	if err != nil {
		return nil, err
	}
	key := l.loadKey(systemID, result.Schema.TargetNamespace)
	return l.loadParsed(result, systemID, key)
}

// loadResolved loads a schema from an already-resolved reader and systemID.
func (l *SchemaLoader) loadResolved(doc io.ReadCloser, systemID string, key loadKey) (*parser.Schema, error) {
	session := newLoadSession(l, systemID, key, doc)

	if sch, ok := l.state.loadedSchema(key); ok {
		if closeErr := closeSchemaDoc(doc, systemID); closeErr != nil {
			return nil, closeErr
		}
		return sch, nil
	}

	loadedSchema, err := session.handleCircularLoad()
	if err != nil || loadedSchema != nil {
		closeErr := closeSchemaDoc(doc, systemID)
		if combined := joinWithClose(err, closeErr); combined != nil {
			return nil, combined
		}
		return loadedSchema, err
	}

	result, err := session.parseSchema()
	if err != nil {
		return nil, err
	}
	return l.loadParsed(result, systemID, key)
}
