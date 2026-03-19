package preprocessor

import (
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/preprocessor/resolve"
)

// Load loads and merges a schema graph from location.
// It requires a configured resolver for root resolution and is retryable after failures.
func (l *Loader) Load(location string) (*parser.Schema, error) {
	if err := l.beginLocationLoad(); err != nil {
		return nil, err
	}
	l.state = newLoadState()
	l.imports = NewTracker[loadKey]()
	return l.loadRoot(location)
}

// loadRoot loads the root schema by resolving the provided location.
func (l *Loader) loadRoot(location string) (*parser.Schema, error) {
	if l == nil || l.resolver == nil {
		return nil, fmt.Errorf("no resolver configured")
	}
	doc, systemID, err := l.resolver.Resolve(resolve.Request{
		BaseSystemID:   "",
		SchemaLocation: location,
		Kind:           resolve.Include,
	})
	if err != nil {
		return nil, err
	}
	result, err := Parse(doc, systemID, l.config.DocumentPool, l.config.SchemaParseOptions...)
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
	parentJournal *Journal[loadKey],
) (*parser.Schema, error) {
	session := newLoadSession(l, systemID, key, doc)
	return LoadResolved(doc, systemID, LoadCallbacks{
		Loaded: func() (*parser.Schema, bool) {
			return l.state.loadedSchema(key)
		},
		Circular: session.handleCircularLoad,
		Close:    Close,
		Parse: func(doc io.ReadCloser, systemID string) (*parser.ParseResult, error) {
			return Parse(doc, systemID, l.config.DocumentPool, l.config.SchemaParseOptions...)
		},
		ApplyParsed: func(result *parser.ParseResult, systemID string) (*parser.Schema, error) {
			return l.loadParsedWithJournal(result, systemID, key, parentJournal)
		},
	})
}
