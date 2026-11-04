package tests_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestXMLStreamBorrowedProvenance(t *testing.T) {
	const source = `package fixture
import (
 "bytes"
 "encoding/xml"
 "slices"
 "github.com/jacoelho/xsd/internal/xmlstream"
)
var state struct { data []byte; values []any; value any; fn func(); flag bool; name xml.Name; count int }
var global any
func consume(any) {}
func dataField(tok *xmlstream.Token) {
 state.data = tok.Data // want field store
}
func directiveField(tok *xmlstream.Token) {
 state.data = tok.Directive // want field store
}
func erasedAlias(tok *xmlstream.Token) {
 value := any(tok)
 state.value = value // want field store
}
func implicitInterface(tok *xmlstream.Token) {
 var value any = tok
 global = value // want global store
}
func rawValue(attr *xmlstream.Attr) {
 raw, ok := attr.RawValue()
 state.flag = ok
 state.data = raw // want field store
}
func rawMethodExpression(attr *xmlstream.Attr) {
 raw, _ := (*xmlstream.Attr).RawValue(attr)
 state.data = raw // want field store
}
func rawMethodAlias(attr *xmlstream.Attr) {
 method := attr.RawValue
 raw, _ := method()
 state.data = raw // want field store
}
func tupleAssignment(tok *xmlstream.Token) {
 var raw []byte
 var scalar int
 scalar, raw = tok.Line, tok.Data
 state.count = scalar
 state.data = raw // want field store
}
func tupleDeclaration(attr *xmlstream.Attr) {
 var raw, ok = attr.RawValue()
 state.flag = ok
 state.data = raw // want field store
}
func erasedAssertion(tok *xmlstream.Token) {
 value := any(tok.Data)
 raw, ok := value.([]byte)
 state.flag = ok
 state.data = raw[1:] // want field store
}
func erasedTypeSwitch(tok *xmlstream.Token) {
 var value any = tok.Data
 switch raw := value.(type) {
 case []byte:
  state.data = raw // want field store
 }
}
func returned(tok *xmlstream.Token) []byte {
 return tok.Data // want return
}
func namedReturn(tok *xmlstream.Token) (raw []byte) { // want return
 raw = tok.Data
 return
}
func globalDeclaration(tok *xmlstream.Token) {
 global = tok // want global store
}
func localShadow(tok *xmlstream.Token) {
 global := any(tok)
 consume(global)
}
func indexed(tok *xmlstream.Token, destination []any) {
 destination[0] = tok.Data // want indexed store
}
func pointerStore(tok *xmlstream.Token, destination *any) {
 *destination = tok.Data // want indirect store
}
func send(tok *xmlstream.Token, destination chan any) {
 destination <- tok.Data // want channel send
}
func delayedArgument(tok *xmlstream.Token) {
 go consume(tok.Data) // want delayed call
 defer consume(any(tok)) // want delayed call
}
func delayedReceiver(attr *xmlstream.Attr) {
 defer attr.RawValue() // want delayed call
}
func delayedMethod(attr *xmlstream.Attr) {
 method := attr.RawValue
 go method() // want delayed call
}
func delayedClosure(tok *xmlstream.Token) {
 raw := tok.Data
 go func() { consume(raw) }() // want delayed call
}
func capturedBeforeAssignment(tok *xmlstream.Token) {
 var raw []byte
 callback := func() { consume(raw) }
 raw = tok.Data
 state.fn = callback // want field store
}
func returnedClosure(tok *xmlstream.Token) func() {
 raw := tok.Data
 return func() { consume(raw) } // want return
}
func storedClosure(tok *xmlstream.Token) {
 state.fn = func() { consume(tok.Data) } // want field store
}
func synchronousClosure(tok *xmlstream.Token) {
 raw := tok.Data
 func() { consume(raw) }()
}
func closureResult(tok *xmlstream.Token) {
 raw := tok.Data
 get := func() []byte { return raw } // want return
 state.data = get() // want field store
}
func branch(tok *xmlstream.Token, condition bool) {
 var raw []byte
 if condition { raw = tok.Data }
 state.data = raw // want field store
}
func loop(tok *xmlstream.Token) {
 var first, second []byte
 for i := 0; i < 2; i++ { first = second; second = tok.Data }
 state.data = first // want field store
}
func sticky(tok *xmlstream.Token) {
 raw := tok.Data
 raw = bytes.Clone(raw)
 state.data = raw // want field store
}
func indirectAlias(tok *xmlstream.Token) {
 raw := tok.Data
 pointer := &raw
 state.data = *pointer // want field store
}
func composite(tok *xmlstream.Token) {
 values := []any{tok.Data}
 state.value = values // want field store
 state.value = values[0] // want field store
}
func mapComposite(tok *xmlstream.Token) {
 values := map[string]any{"raw":tok.Data}
 state.value = values // want field store
}
func rangeAlias(tok *xmlstream.Token) {
 values := []any{tok.Data}
 for _, value := range values { state.value = value } // want field store
}
func referenceConversion(tok *xmlstream.Token) {
 type rawBytes []byte
 raw := rawBytes(tok.Data)
 state.data = []byte(raw) // want field store
}
func addressIntoToken(tok *xmlstream.Token) {
 state.value = &tok.Line // want field store
 state.value = &tok.End.Name.Space // want field store
 state.value = &tok.Data[0] // want field store
}
func shallowCopy(tok *xmlstream.Token, destination []any) {
 values := []any{tok.Data}
 copy(destination, values) // want indexed store
 copy(destination, []any{tok}) // want indexed store
}
func rangeMapKey(tok *xmlstream.Token) {
 values := map[any]bool{tok:true}
 for key := range values { state.value = key } // want field store
}
type mapKey struct{ v any }
func mapKeyStores(tok *xmlstream.Token, m map[any]bool, p map[*xmlstream.Token]bool, c map[any]int, outer map[string]map[any]bool, n int, ptr *map[any]bool, a map[[1]any]bool) {
 m[any(tok)] = true // want indexed store
 p[tok] = true // want indexed store
 m[mapKey{v: tok}] = true // want indexed store
 c[any(tok)] += 1 // want indexed store
 c[any(tok)]++ // want indexed store
 outer["a"][tok] = true // want indexed store
 (*ptr)[tok] = true // want indexed store
 a[[1]any{tok}] = true // want indexed store
 m[any(tok)], n = true, 1 // want indexed store
 consume(n)
}
// A local map is sunk at the store like an indexed slice store; proving it
// never escapes would need alias tracking.
func localMapKey(tok *xmlstream.Token) {
 m := map[any]bool{}
 m[tok] = true // want indexed store
 consume(m)
}
func typeParameterMapKey[M ~map[any]bool](m M, tok *xmlstream.Token) {
 m[tok] = true // want indexed store
}
func mapKeyReads(tok *xmlstream.Token, m map[any]bool, outer map[any]map[string]bool, s map[string]bool, l map[int]bool, reader *xmlstream.Reader) {
 outer[tok]["a"] = true
 _ = m[tok]
 delete(m, tok)
 v, ok := m[tok]
 consume(v)
 consume(ok)
 s[string(tok.Data)] = true
 l[tok.Line] = true
 m[reader.InternBytes(tok.Data)] = true
}
func shallowClone(tok *xmlstream.Token) {
 values := []any{tok.Data}
 state.value = slices.Clone(values) // want field store
}
func borrowedAppend(tok *xmlstream.Token) {
 state.data = append(tok.Data, 'x') // want field store
}
func shallowAppend(tok *xmlstream.Token) {
 state.value = append([]any(nil), tok) // want field store
}
func appendIntoRetainedBacking(tok *xmlstream.Token, destination []any) {
 local := append(destination[:0], tok.Data) // want indexed store
 consume(local)
 _ = append(destination[:0], tok.Data) // want indexed store
 values := []any{tok.Data}
 _ = append(destination[:0], values...) // want indexed store
}
func appendIntoStateBacking(tok *xmlstream.Token) {
 local := state.values[:0]
 _ = append(local, tok.Data) // want indexed store
 _ = append(state.values[:0], any(tok)) // want indexed store
 _ = append(state.values[:0], string(tok.Data))
 _ = append(state.values[:0], bytes.Clone(tok.Data))
}
func returnFreshAppend(tok *xmlstream.Token) any {
 local := append([]any(nil), tok.Data)
 return local // want return
}
func delayFreshAppend(tok *xmlstream.Token) {
 go consume(append([]any(nil), tok.Data)) // want delayed call
}
func appendIntoFreshBacking(tok *xmlstream.Token) {
 consume(append([]any(nil), tok.Data))
 consume(append([]any{}, tok.Data))
 consume(append(make([]any, 0), tok.Data))
 consume(append(make([]any, 0, 8), tok.Data))
}
func ownedCopies(tok *xmlstream.Token, attr *xmlstream.Attr, reader *xmlstream.Reader) {
 state.value = string(tok.Data)
 state.data = []byte(string(tok.Data))
 state.data = bytes.Clone(tok.Data)
 state.data = slices.Clone(tok.Data)
 state.data = slices.Clone[[]byte](tok.Data)
 state.data = append(state.data, tok.Data...)
 copy(state.data, tok.Data)
 state.value = reader.InternBytes(tok.Data)
 value, ok := reader.MaterializeValue(attr)
 state.value, state.flag = value, ok
 state.name = tok.Start.Name
 state.count = tok.Line
 state.count = int(tok.Data[0])
}
func unrelated(data []byte, value any) {
 state.data = data
 state.value = value
}
func structuralField() {
 var local struct { token *xmlstream.Token } // want field type
 consume(local)
}
func structuralAlias() {
 type Alias = xmlstream.Attr // want type alias
 var attr Alias
 consume(attr)
}
var forbidden *xmlstream.Token // want global type
func structuralResult(tok *xmlstream.Token) *xmlstream.Token { // want result type
 return tok // want return
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	violations, err := analyzeXMLStreamPackage(fset, "github.com/jacoelho/xsd/tests/fixture", []*ast.File{file})
	if err != nil {
		t.Fatal(err)
	}
	expected := make(map[int]string)
	for i, line := range strings.Split(source, "\n") {
		if _, kind, found := strings.Cut(line, "// want "); found {
			expected[i+1] = kind
		}
	}
	for _, violation := range violations {
		line := fset.Position(violation.pos).Line
		kind, ok := expected[line]
		if !ok || kind != violation.kind {
			t.Errorf("unexpected %s at line %d; want %q", violation.kind, line, kind)
		}
		delete(expected, line)
	}
	for line, kind := range expected {
		t.Errorf("missing %s at line %d", kind, line)
	}
}
