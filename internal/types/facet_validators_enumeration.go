package types

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
)

type Enumeration struct {
	aux    atomic.Pointer[enumAux]
	values []string
	sealed atomic.Bool
}

// NewEnumeration creates an enumeration facet with immutable values.
func NewEnumeration(values []string) *Enumeration {
	if len(values) == 0 {
		return &Enumeration{}
	}
	return &Enumeration{values: slices.Clone(values)}
}

// Values returns a copy of the enumeration values.
func (e *Enumeration) Values() []string {
	if e == nil || len(e.values) == 0 {
		return nil
	}
	return slices.Clone(e.values)
}

// AppendValue adds a value and optional namespace context during schema parsing.
func (e *Enumeration) AppendValue(value string, context map[string]string) {
	if e == nil {
		return
	}
	e.ensureMutable()
	e.values = append(e.values, value)

	aux := e.aux.Load()
	if (aux == nil || len(aux.valueContexts) == 0) && context == nil {
		e.setAux(nil, nil)
		return
	}

	var contexts []map[string]string
	if aux != nil && len(aux.valueContexts) > 0 {
		contexts = cloneValueContexts(aux.valueContexts)
	}
	if len(contexts) < len(e.values) {
		next := make([]map[string]string, len(e.values))
		copy(next, contexts)
		contexts = next
	}
	contexts[len(contexts)-1] = copyNamespaceContext(context)
	e.setAux(contexts, nil)
}

// Seal marks the enumeration as immutable.
func (e *Enumeration) Seal() {
	if e == nil {
		return
	}
	e.sealed.Store(true)
}

func (e *Enumeration) ensureMutable() {
	if e == nil {
		return
	}
	if e.sealed.Load() {
		panic("enumeration is sealed")
	}
}

type enumAux struct {
	caches        enumCaches
	valueContexts []map[string]string
	qnameValues   []QName
}

type enumCaches struct {
	atomicCache atomic.Pointer[enumCacheAtomic]
	unionCache  atomic.Pointer[enumCacheUnion]
	listCache   atomic.Pointer[enumCacheList]
}

type enumCacheAtomic struct {
	base   Type
	values []TypedValue
}

type enumCacheUnion struct {
	base   Type
	values []TypedValue
}

type enumCacheList struct {
	base   Type
	values [][][]TypedValue
}

// Name returns the facet name
func (e *Enumeration) Name() string {
	return "enumeration"
}

// Validate checks if the value is in the enumeration (unified Facet interface)
func (e *Enumeration) Validate(value TypedValue, baseType Type) error {
	return e.ValidateLexical(value.Lexical(), baseType)
}

// ValidateLexical checks if the lexical value is in the enumeration.
func (e *Enumeration) ValidateLexical(lexical string, baseType Type) error {
	if e == nil {
		return nil
	}
	if baseType == nil {
		return fmt.Errorf("enumeration: missing base type")
	}

	normalized := NormalizeWhiteSpace(lexical, baseType)

	if isQNameOrNotationType(baseType) {
		if slices.Contains(e.values, normalized) {
			return nil
		}
		return fmt.Errorf("value %s not in enumeration: %s", lexical, FormatEnumerationValues(e.values))
	}

	if itemType, ok := ListItemType(baseType); ok {
		match, err := e.matchesListEnumeration(normalized, baseType, itemType)
		if err != nil {
			return err
		}
		if match {
			return nil
		}
		return fmt.Errorf("value %s not in enumeration: %s", lexical, FormatEnumerationValues(e.values))
	}

	if memberTypes := unionMemberTypes(baseType); len(memberTypes) > 0 {
		match, err := e.matchesUnionEnumeration(normalized, baseType, memberTypes)
		if err != nil {
			return err
		}
		if match {
			return nil
		}
		return fmt.Errorf("value %s not in enumeration: %s", lexical, FormatEnumerationValues(e.values))
	}

	match, err := e.matchesAtomicEnumeration(normalized, baseType)
	if err != nil {
		return err
	}
	if match {
		return nil
	}
	return fmt.Errorf("value %s not in enumeration: %s", lexical, FormatEnumerationValues(e.values))
}

// ValidateLexicalQName validates QName/NOTATION enumerations using namespace context.
func (e *Enumeration) ValidateLexicalQName(lexical string, baseType Type, context map[string]string) error {
	if e == nil {
		return nil
	}
	if baseType == nil {
		return fmt.Errorf("enumeration: missing base type")
	}
	if !isQNameOrNotationType(baseType) {
		return e.ValidateLexical(lexical, baseType)
	}
	if context == nil {
		return fmt.Errorf("namespace context unavailable for QName/NOTATION enumeration")
	}
	normalized := NormalizeWhiteSpace(lexical, baseType)
	qname, err := ParseQNameValue(normalized, context)
	if err != nil {
		return err
	}
	allowed, err := e.ResolveQNameValues()
	if err != nil {
		return err
	}
	if slices.Contains(allowed, qname) {
		return nil
	}
	return fmt.Errorf("value %s not in enumeration: %s", lexical, FormatEnumerationValues(e.values))
}

// ValueContexts returns namespace contexts aligned with Values.
func (e *Enumeration) ValueContexts() []map[string]string {
	if e == nil {
		return nil
	}
	aux := e.aux.Load()
	if aux == nil || len(aux.valueContexts) == 0 {
		return nil
	}
	return cloneValueContexts(aux.valueContexts)
}

// SetValueContexts stores namespace contexts aligned with Values.
func (e *Enumeration) SetValueContexts(values []map[string]string) {
	if e == nil {
		return
	}
	e.ensureMutable()
	contexts := cloneValueContexts(values)
	var qnames []QName
	if aux := e.aux.Load(); aux != nil && len(aux.qnameValues) > 0 {
		qnames = slices.Clone(aux.qnameValues)
	}
	e.setAux(contexts, qnames)
}

// QNameValues returns resolved QName values for QName/NOTATION enumerations.
func (e *Enumeration) QNameValues() []QName {
	if e == nil {
		return nil
	}
	aux := e.aux.Load()
	if aux == nil || len(aux.qnameValues) == 0 {
		return nil
	}
	return slices.Clone(aux.qnameValues)
}

// SetQNameValues stores resolved QName values for QName/NOTATION enumerations.
func (e *Enumeration) SetQNameValues(values []QName) {
	if e == nil {
		return
	}
	e.ensureMutable()
	qnames := slices.Clone(values)
	var contexts []map[string]string
	if aux := e.aux.Load(); aux != nil && len(aux.valueContexts) > 0 {
		contexts = cloneValueContexts(aux.valueContexts)
	}
	e.setAux(contexts, qnames)
}

func (e *Enumeration) ensureAux() *enumAux {
	if e == nil {
		return nil
	}
	if aux := e.aux.Load(); aux != nil {
		return aux
	}
	created := &enumAux{}
	if e.aux.CompareAndSwap(nil, created) {
		return created
	}
	return e.aux.Load()
}

func (e *Enumeration) cacheSet() *enumCaches {
	aux := e.ensureAux()
	if aux == nil {
		return nil
	}
	return &aux.caches
}

func (e *Enumeration) setAux(valueContexts []map[string]string, qnameValues []QName) {
	e.aux.Store(&enumAux{
		valueContexts: valueContexts,
		qnameValues:   qnameValues,
	})
}

func cloneValueContexts(values []map[string]string) []map[string]string {
	if len(values) == 0 {
		return nil
	}
	copied := make([]map[string]string, len(values))
	for i, ctx := range values {
		copied[i] = copyNamespaceContext(ctx)
	}
	return copied
}

func (e *Enumeration) matchesAtomicEnumeration(lexical string, baseType Type) (bool, error) {
	actual, err := parseTypedValue(lexical, baseType)
	if err != nil {
		return false, err
	}
	allowed, err := e.atomicEnumerationValues(baseType)
	if err != nil {
		return false, err
	}
	for _, candidate := range allowed {
		if ValuesEqual(actual, candidate) {
			return true, nil
		}
	}
	return false, nil
}

func (e *Enumeration) matchesUnionEnumeration(lexical string, baseType Type, memberTypes []Type) (bool, error) {
	actualValues, err := parseUnionValueVariants(lexical, memberTypes)
	if err != nil {
		return false, err
	}
	allowed, err := e.unionEnumerationValues(baseType, memberTypes)
	if err != nil {
		return false, err
	}
	for _, actual := range actualValues {
		for _, candidate := range allowed {
			if ValuesEqual(actual, candidate) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (e *Enumeration) matchesListEnumeration(lexical string, baseType, itemType Type) (bool, error) {
	actualItems, err := parseListValueVariants(lexical, itemType)
	if err != nil {
		return false, err
	}
	allowed, err := e.listEnumerationValues(baseType, itemType)
	if err != nil {
		return false, err
	}
	for _, candidate := range allowed {
		if listValuesEqual(actualItems, candidate) {
			return true, nil
		}
	}
	return false, nil
}

func (e *Enumeration) atomicEnumerationValues(baseType Type) ([]TypedValue, error) {
	cacheSet := e.cacheSet()
	if cacheSet != nil {
		if cache := cacheSet.atomicCache.Load(); cache != nil && cache.base == baseType {
			return cache.values, nil
		}
	}
	values := make([]TypedValue, 0, len(e.values))
	for _, val := range e.values {
		normalized := NormalizeWhiteSpace(val, baseType)
		typed, err := parseTypedValue(normalized, baseType)
		if err != nil {
			return nil, fmt.Errorf("enumeration value %q: %w", val, err)
		}
		values = append(values, typed)
	}
	if cacheSet != nil {
		cacheSet.atomicCache.Store(&enumCacheAtomic{base: baseType, values: values})
	}
	return values, nil
}

func (e *Enumeration) unionEnumerationValues(baseType Type, memberTypes []Type) ([]TypedValue, error) {
	cacheSet := e.cacheSet()
	if cacheSet != nil {
		if cache := cacheSet.unionCache.Load(); cache != nil && cache.base == baseType {
			return cache.values, nil
		}
	}
	values := make([]TypedValue, 0, len(e.values))
	for _, val := range e.values {
		normalized := NormalizeWhiteSpace(val, baseType)
		typed, err := parseUnionValueVariants(normalized, memberTypes)
		if err != nil {
			return nil, fmt.Errorf("enumeration value %q: %w", val, err)
		}
		values = append(values, typed...)
	}
	if cacheSet != nil {
		cacheSet.unionCache.Store(&enumCacheUnion{base: baseType, values: values})
	}
	return values, nil
}

func (e *Enumeration) listEnumerationValues(baseType, itemType Type) ([][][]TypedValue, error) {
	cacheSet := e.cacheSet()
	if cacheSet != nil {
		if cache := cacheSet.listCache.Load(); cache != nil && cache.base == baseType {
			return cache.values, nil
		}
	}
	values := make([][][]TypedValue, len(e.values))
	for i, val := range e.values {
		normalized := NormalizeWhiteSpace(val, baseType)
		parsed, err := parseListValueVariants(normalized, itemType)
		if err != nil {
			return nil, fmt.Errorf("enumeration value %q: %w", val, err)
		}
		values[i] = parsed
	}
	if cacheSet != nil {
		cacheSet.listCache.Store(&enumCacheList{base: baseType, values: values})
	}
	return values, nil
}

func parseTypedValue(lexical string, typ Type) (TypedValue, error) {
	switch t := typ.(type) {
	case *SimpleType:
		return t.parseValueInternal(lexical, false)
	case *BuiltinType:
		return t.ParseValue(lexical)
	default:
		return nil, fmt.Errorf("unsupported type %T", typ)
	}
}

func parseUnionValueVariants(lexical string, memberTypes []Type) ([]TypedValue, error) {
	if len(memberTypes) == 0 {
		return nil, fmt.Errorf("union has no member types")
	}
	values := make([]TypedValue, 0, len(memberTypes))
	var firstErr error
	for _, memberType := range memberTypes {
		typed, err := parseTypedValue(lexical, memberType)
		if err == nil {
			values = append(values, typed)
			continue
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if len(values) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("value %q does not match any union member type", lexical)
	}
	return values, nil
}

func parseListValueVariants(lexical string, itemType Type) ([][]TypedValue, error) {
	if itemType == nil {
		return nil, fmt.Errorf("list item type is nil")
	}
	parsed := make([][]TypedValue, 0, 4)
	for item := range FieldsXMLWhitespaceSeq(lexical) {
		values, err := parseValueVariants(item, itemType)
		if err != nil {
			return nil, fmt.Errorf("invalid list item %q: %w", item, err)
		}
		parsed = append(parsed, values)
	}
	if len(parsed) == 0 {
		return [][]TypedValue{}, nil
	}
	return parsed, nil
}

func parseValueVariants(lexical string, typ Type) ([]TypedValue, error) {
	if members := unionMemberTypes(typ); len(members) > 0 {
		return parseUnionValueVariants(lexical, members)
	}
	typed, err := parseTypedValue(lexical, typ)
	if err != nil {
		return nil, err
	}
	return []TypedValue{typed}, nil
}

func listValuesEqual(left, right [][]TypedValue) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !anyValueEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}

func anyValueEqual(left, right []TypedValue) bool {
	for _, l := range left {
		for _, r := range right {
			if ValuesEqual(l, r) {
				return true
			}
		}
	}
	return false
}

// ResolveQNameValues parses enumeration values as QNames using ValueContexts.
// It returns a QName for each entry in Values or an error if any value cannot be resolved.
func (e *Enumeration) ResolveQNameValues() ([]QName, error) {
	if e == nil || len(e.values) == 0 {
		return nil, nil
	}
	contexts := e.ValueContexts()
	if len(contexts) != len(e.values) {
		return nil, fmt.Errorf("enumeration contexts %d do not match values %d", len(contexts), len(e.values))
	}
	qnames := make([]QName, len(e.values))
	for i, value := range e.values {
		context := contexts[i]
		if context == nil {
			return nil, fmt.Errorf("missing namespace context for enumeration value %q", value)
		}
		qname, err := ParseQNameValue(value, context)
		if err != nil {
			return nil, fmt.Errorf("invalid QName enumeration value %q: %w", value, err)
		}
		qnames[i] = qname
	}
	return qnames, nil
}

// FormatEnumerationValues returns a quoted list for enumeration errors.
func FormatEnumerationValues(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = strconv.Quote(value)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
