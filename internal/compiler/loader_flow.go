package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
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
	doc, systemID, err := l.resolver.Resolve(ResolveRequest{
		BaseSystemID:   "",
		SchemaLocation: location,
		Kind:           ResolveInclude,
	})
	if err != nil {
		return nil, err
	}
	result, err := Parse(doc, systemID, l.config.DocumentPool, l.config.SchemaParseOptions...)
	if err != nil {
		return nil, err
	}
	key := l.loadKey(systemID, result.Schema.TargetNamespace)
	return l.loadParsedWithJournal(result, systemID, key, nil)
}
