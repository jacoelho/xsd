package validator

import "github.com/jacoelho/xsd/internal/runtime"

// NameID identifies a name entry within a single document.
type NameID uint32

type nameEntry struct {
	Sym      runtime.SymbolID
	NS       runtime.NamespaceID
	LocalOff uint32
	LocalLen uint32
	NSOff    uint32
	NSLen    uint32
}
