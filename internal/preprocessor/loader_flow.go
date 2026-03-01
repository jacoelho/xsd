package preprocessor

import (
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/parser"
)

// Load loads and merges a schema graph from location.
// It requires a configured resolver for root resolution and is retryable after failures.
func (l *Loader) Load(location string) (*parser.Schema, error) {
	if err := l.beginLocationLoad(); err != nil {
		return nil, err
	}
	l.state = newLoadState()
	l.imports = newImportTracker()
	return l.loadRoot(location)
}

// loadRoot loads the root schema by resolving the provided location.
func (l *Loader) loadRoot(location string) (*parser.Schema, error) {
	if l == nil || l.resolver == nil {
		return nil, fmt.Errorf("no resolver configured")
	}
	doc, systemID, err := l.resolver.Resolve(ResolveRequest{
		BaseSystemID:   "",
		SchemaLocation: location,
		Kind:           ResolveInclude,
	})
	if err != nil {
		return nil, err
	}
	result, err := parseSchemaDocument(doc, systemID, l.config.DocumentPool, l.config.SchemaParseOptions...)
	if err != nil {
		return nil, err
	}
	key := l.loadKey(systemID, result.Schema.TargetNamespace)
	return l.loadParsed(result, systemID, key)
}

func (l *Loader) loadResolvedWithJournal(
	doc io.ReadCloser,
	systemID string,
	key loadKey,
	parentJournal *stateJournal,
) (*parser.Schema, error) {
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

	result, err := parseSchemaDocument(session.doc, session.systemID, session.loader.config.DocumentPool, session.loader.config.SchemaParseOptions...)
	if err != nil {
		return nil, err
	}
	return l.loadParsedWithJournal(result, systemID, key, parentJournal)
}
