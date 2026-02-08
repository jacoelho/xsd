package runtime

import (
	"fmt"

	"github.com/jacoelho/xsd/pkg/xmlstream"
)

type Builder struct {
	namespaces namespaceBuilder
	symbols    symbolBuilder
	sealed     bool

	emptyNS NamespaceID
	xmlNS   NamespaceID
	xsiNS   NamespaceID

	predef PredefinedSymbols
}

func NewBuilder() *Builder {
	b := &Builder{
		namespaces: newNamespaceBuilder(),
		symbols:    newSymbolBuilder(),
	}
	b.emptyNS = b.namespaces.intern(nil)
	b.xmlNS = b.namespaces.intern([]byte(xmlstream.XMLNamespace))
	b.xsiNS = b.namespaces.intern([]byte(xmlstream.XSINamespace))

	b.predef = PredefinedSymbols{
		XsiType:                      b.symbols.intern(b.xsiNS, []byte("type")),
		XsiNil:                       b.symbols.intern(b.xsiNS, []byte("nil")),
		XsiSchemaLocation:            b.symbols.intern(b.xsiNS, []byte("schemaLocation")),
		XsiNoNamespaceSchemaLocation: b.symbols.intern(b.xsiNS, []byte("noNamespaceSchemaLocation")),
		XmlLang:                      b.symbols.intern(b.xmlNS, []byte("lang")),
		XmlSpace:                     b.symbols.intern(b.xmlNS, []byte("space")),
	}

	return b
}

func (b *Builder) InternNamespace(uri []byte) (NamespaceID, error) {
	if err := b.ensureMutable(); err != nil {
		return 0, err
	}
	return b.namespaces.intern(uri), nil
}

func (b *Builder) InternSymbol(nsID NamespaceID, local []byte) (SymbolID, error) {
	if err := b.ensureMutable(); err != nil {
		return 0, err
	}
	return b.symbols.intern(nsID, local), nil
}

func (b *Builder) Build() (*Schema, error) {
	if err := b.ensureMutable(); err != nil {
		return nil, err
	}
	b.sealed = true
	namespaces, err := b.namespaces.build()
	if err != nil {
		return nil, fmt.Errorf("runtime build: namespaces: %w", err)
	}
	symbols, err := b.symbols.build()
	if err != nil {
		return nil, fmt.Errorf("runtime build: symbols: %w", err)
	}
	return &Schema{
		Namespaces: namespaces,
		Symbols:    symbols,
		Predef:     b.predef,
		PredefNS: PredefinedNamespaces{
			Empty: b.emptyNS,
			Xml:   b.xmlNS,
			Xsi:   b.xsiNS,
		},
	}, nil
}

func (b *Builder) ensureMutable() error {
	if b.sealed {
		return fmt.Errorf("runtime.Builder used after Build")
	}
	return nil
}
