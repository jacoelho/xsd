package validator

import (
	"encoding/base64"
	"encoding/hex"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
	xsdxml "github.com/jacoelho/xsd/internal/xml"
	"github.com/jacoelho/xsd/internal/xpath"
)

type fieldNodeKind int

const (
	fieldNodeElement fieldNodeKind = iota
	fieldNodeAttribute
)

type fieldNodeKey struct {
	attrNamespace types.NamespaceURI
	attrLocal     string
	kind          fieldNodeKind
	elemID        uint64
}

type fieldCapture struct {
	match      *selectorMatch
	fieldIndex int
}

type fieldState struct {
	nodes        map[fieldNodeKey]struct{}
	value        string
	display      string
	count        int
	multiple     bool
	missing      bool
	invalid      bool
	valueInvalid bool
	hasValue     bool
}

func (s *fieldState) addNode(key fieldNodeKey) bool {
	if s.nodes == nil {
		s.nodes = make(map[fieldNodeKey]struct{})
	}
	if _, ok := s.nodes[key]; ok {
		return false
	}
	s.nodes[key] = struct{}{}
	s.count++
	if s.count > 1 {
		s.multiple = true
	}
	return true
}

type selectorMatch struct {
	constraint *constraintState
	fields     []fieldState
	id         uint64
	depth      int
	invalid    bool
}

type constraintState struct {
	constraint      *grammar.CompiledConstraint
	selectorMatches map[uint64]*selectorMatch
}

type keyRefEntry struct {
	constraint *grammar.CompiledConstraint
	value      string
	display    string
	path       string
	line       int
	column     int
}

type identityScope struct {
	decl        *grammar.CompiledElement
	keyTables   map[types.QName]map[string]string
	constraints []*constraintState
	keyRefs     []keyRefEntry
	rootID      uint64
	rootDepth   int
	invalid     bool
}

func (r *streamRun) handleIdentityStart(frame *streamFrame, attrs []streamAttr) {
	if frame == nil {
		return
	}
	r.openIdentityScopes(frame)
	currentDepth := len(r.frames) - 1

	for _, scope := range r.identityScopes {
		if scope.invalid {
			continue
		}
		for _, state := range scope.constraints {
			if _, ok := state.selectorMatches[frame.id]; !ok {
				for _, path := range state.constraint.SelectorPaths {
					if matchPath(path, r.frames, scope.rootDepth, currentDepth) {
						state.selectorMatches[frame.id] = &selectorMatch{
							id:         frame.id,
							depth:      currentDepth,
							constraint: state,
							fields:     make([]fieldState, len(state.constraint.FieldPaths)),
						}
						break
					}
				}
			}
		}
	}

	for _, scope := range r.identityScopes {
		if scope.invalid {
			continue
		}
		for _, state := range scope.constraints {
			for _, match := range state.selectorMatches {
				if match.invalid {
					continue
				}
				for fieldIndex := range match.fields {
					fieldState := &match.fields[fieldIndex]
					if fieldState.multiple {
						continue
					}
					for _, path := range state.constraint.FieldPaths[fieldIndex] {
						if path.Attribute != nil {
							if matchPath(path, r.frames, match.depth, currentDepth) {
								r.applyAttributeSelection(fieldState, *path.Attribute, frame, attrs, match, fieldIndex)
							}
						} else if matchPath(path, r.frames, match.depth, currentDepth) {
							r.applyElementSelection(fieldState, frame, match, fieldIndex)
						}
						if fieldState.multiple {
							break
						}
					}
				}
			}
		}
	}
}

func (r *streamRun) handleIdentityEnd(frame *streamFrame) {
	if frame == nil || len(r.identityScopes) == 0 {
		return
	}
	if frame.invalid {
		r.abortIdentityFrame(frame)
		return
	}

	r.applyFieldCaptures(frame)
	r.finalizeSelectorMatches(frame)
	r.closeIdentityScopes(frame)
}

func (r *streamRun) openIdentityScopes(frame *streamFrame) {
	decls := r.constraintDeclsForQName(frame.qname)
	if len(decls) == 0 {
		return
	}
	currentDepth := len(r.frames) - 1
	for _, decl := range decls {
		scope := &identityScope{
			rootID:      frame.id,
			rootDepth:   currentDepth,
			decl:        decl,
			constraints: make([]*constraintState, len(decl.Constraints)),
			keyTables:   make(map[types.QName]map[string]string),
		}
		for i, constraint := range decl.Constraints {
			scope.constraints[i] = &constraintState{
				constraint:      constraint,
				selectorMatches: make(map[uint64]*selectorMatch),
			}
			if constraint.Original.Type == types.KeyConstraint || constraint.Original.Type == types.UniqueConstraint {
				keyQName := constraint.QName(decl)
				if _, ok := scope.keyTables[keyQName]; !ok {
					scope.keyTables[keyQName] = make(map[string]string)
				}
			}
		}
		r.identityScopes = append(r.identityScopes, scope)
	}
}

func (r *streamRun) constraintDeclsForQName(qname types.QName) []*grammar.CompiledElement {
	if r.constraintDecls != nil {
		if cached, ok := r.constraintDecls[qname]; ok {
			return cached
		}
	}
	if r.schema == nil || len(r.schema.ElementsWithConstraints()) == 0 {
		return nil
	}
	if r.validator != nil && r.validator.grammar != nil {
		if lookup := r.validator.grammar.ConstraintDeclsByQName; lookup != nil {
			return lookup[qname]
		}
	}
	var matches []*grammar.CompiledElement
	for _, decl := range r.schema.ElementsWithConstraints() {
		if decl.QName == qname {
			matches = append(matches, decl)
			continue
		}
		for _, sub := range r.schema.SubstitutionGroup(decl.QName) {
			if sub.QName == qname {
				matches = append(matches, decl)
				break
			}
		}
	}
	if matches == nil {
		return nil
	}
	if r.constraintDecls == nil {
		r.constraintDecls = make(map[types.QName][]*grammar.CompiledElement)
	}
	r.constraintDecls[qname] = matches
	return matches
}

func (r *streamRun) applyElementSelection(state *fieldState, frame *streamFrame, match *selectorMatch, fieldIndex int) {
	if state.multiple {
		return
	}
	key := fieldNodeKey{kind: fieldNodeElement, elemID: frame.id}
	if !state.addNode(key) {
		return
	}
	if state.count > 1 {
		state.multiple = true
		return
	}
	if frame.textType == nil {
		state.invalid = true
		return
	}
	frame.collectStringValue = true
	frame.fieldCaptures = append(frame.fieldCaptures, fieldCapture{match: match, fieldIndex: fieldIndex})
}

func (r *streamRun) applyAttributeSelection(state *fieldState, test xpath.NodeTest, frame *streamFrame, attrs []streamAttr, match *selectorMatch, fieldIndex int) {
	if state.multiple {
		return
	}
	field := match.constraint.constraint.Original.Fields[fieldIndex]

	if test.Any {
		for _, attr := range attrs {
			if isXMLNSAttribute(attr) {
				continue
			}
			attrQName := types.QName{
				Namespace: types.NamespaceURI(attr.NamespaceURI()),
				Local:     attr.LocalName(),
			}
			r.addAttributeValue(state, field, frame, attrQName, attr.Value(), nil)
			if state.multiple {
				return
			}
		}
		r.addWildcardAttributeDefaults(state, field, frame, test)
		return
	}

	if test.Local == "*" && test.NamespaceSpecified {
		for _, attr := range attrs {
			if isXMLNSAttribute(attr) {
				continue
			}
			attrNamespace := types.NamespaceURI(attr.NamespaceURI())
			attrQName := types.QName{
				Namespace: attrNamespace,
				Local:     attr.LocalName(),
			}
			r.addAttributeValue(state, field, frame, attrQName, attr.Value(), nil)
			if state.multiple {
				return
			}
		}
		r.addWildcardAttributeDefaults(state, field, frame, test)
		return
	}

	if test.NamespaceSpecified {
		if attr, ok := findAttrByNamespace(attrs, test.Namespace, test.Local); ok {
			attrQName := types.QName{
				Namespace: test.Namespace,
				Local:     test.Local,
			}
			r.addAttributeValue(state, field, frame, attrQName, attr.Value(), nil)
			return
		}
		if value, valueContext, ok := r.lookupAttributeDefault(frame, types.QName{Namespace: test.Namespace, Local: test.Local}); ok {
			attrQName := types.QName{Namespace: test.Namespace, Local: test.Local}
			r.addAttributeValue(state, field, frame, attrQName, value, valueContext)
		}
		return
	}

	if attr, ok := findAttrByNamespace(attrs, types.NamespaceEmpty, test.Local); ok {
		attrQName := types.QName{Namespace: types.NamespaceEmpty, Local: test.Local}
		r.addAttributeValue(state, field, frame, attrQName, attr.Value(), nil)
		return
	}
	if value, valueContext, ok := r.lookupAttributeDefault(frame, types.QName{Namespace: types.NamespaceEmpty, Local: test.Local}); ok {
		attrQName := types.QName{Namespace: types.NamespaceEmpty, Local: test.Local}
		r.addAttributeValue(state, field, frame, attrQName, value, valueContext)
	}
}

func (r *streamRun) addWildcardAttributeDefaults(state *fieldState, field types.Field, frame *streamFrame, test xpath.NodeTest) {
	if state.multiple || frame == nil {
		return
	}
	typ := frame.typ
	if typ == nil && frame.decl != nil {
		typ = frame.decl.Type
	}
	if typ == nil {
		return
	}
	for _, attr := range typ.AllAttributes {
		if !attr.HasDefault && !attr.HasFixed {
			continue
		}
		if !nodeTestMatches(test, attr.QName) {
			continue
		}
		value := attr.Default
		var valueContext map[string]string
		if attr.HasFixed {
			value = attr.Fixed
			if attr.Original != nil {
				valueContext = attr.Original.FixedContext
			}
		} else if attr.Original != nil {
			valueContext = attr.Original.DefaultContext
		}
		r.addAttributeValue(state, field, frame, attr.QName, value, valueContext)
		if state.multiple {
			return
		}
	}
}

func (r *streamRun) addAttributeValue(state *fieldState, field types.Field, frame *streamFrame, attrQName types.QName, value string, context map[string]string) {
	if state.multiple {
		return
	}
	key := fieldNodeKey{
		kind:          fieldNodeAttribute,
		elemID:        frame.id,
		attrNamespace: attrQName.Namespace,
		attrLocal:     attrQName.Local,
	}
	if !state.addNode(key) {
		return
	}
	if state.count > 1 {
		state.multiple = true
		return
	}
	normalized, keyState := r.normalizeAttributeValue(value, field, frame, attrQName, context)
	switch keyState {
	case KeyInvalidValue:
		state.valueInvalid = true
		return
	case KeyInvalidSelection:
		state.invalid = true
		return
	}
	state.value = normalized
	state.display = strings.Clone(types.TrimXMLWhitespace(value))
	state.hasValue = true
}

func (r *streamRun) applyFieldCaptures(frame *streamFrame) {
	for _, capture := range frame.fieldCaptures {
		match := capture.match
		if match.invalid {
			continue
		}
		fieldState := &match.fields[capture.fieldIndex]
		if fieldState.multiple || fieldState.invalid {
			continue
		}
		field := match.constraint.constraint.Original.Fields[capture.fieldIndex]
		raw, valueContext, hasValue := effectiveElementValue(frame)
		if !hasValue {
			fieldState.missing = true
			continue
		}
		normalized, keyState := r.normalizeElementValue(raw, field, frame, valueContext)
		switch keyState {
		case KeyInvalidValue:
			fieldState.valueInvalid = true
			continue
		case KeyInvalidSelection:
			fieldState.invalid = true
			continue
		}
		fieldState.value = normalized
		fieldState.display = raw
		fieldState.hasValue = true
	}
}

func effectiveElementValue(frame *streamFrame) (string, map[string]string, bool) {
	if frame == nil {
		return "", nil, false
	}
	if frame.nilled {
		return "", nil, false
	}
	if len(frame.textBuf) > 0 || frame.hasChildElements || frame.textType == nil {
		return string(frame.textBuf), nil, true
	}
	if frame.decl == nil {
		return string(frame.textBuf), nil, true
	}
	if frame.decl.HasDefault {
		var ctx map[string]string
		if frame.decl.Original != nil {
			ctx = frame.decl.Original.DefaultContext
		}
		return frame.decl.Default, ctx, true
	}
	if frame.decl.HasFixed {
		var ctx map[string]string
		if frame.decl.Original != nil {
			ctx = frame.decl.Original.FixedContext
		}
		return frame.decl.Fixed, ctx, true
	}
	return "", nil, true
}

func (r *streamRun) finalizeSelectorMatches(frame *streamFrame) {
	for _, scope := range r.identityScopes {
		if scope.invalid {
			continue
		}
		for _, state := range scope.constraints {
			match, ok := state.selectorMatches[frame.id]
			if !ok {
				continue
			}
			delete(state.selectorMatches, frame.id)
			if match.invalid {
				continue
			}
			r.finalizeSelectorMatch(scope, state, match)
		}
	}
}

func (r *streamRun) finalizeSelectorMatch(scope *identityScope, state *constraintState, match *selectorMatch) {
	constraint := state.constraint
	fields := constraint.Original.Fields
	normalizedValues := make([]string, 0, len(fields))
	displayValues := make([]string, 0, len(fields))
	elemPath := r.path.String()

	for i := range fields {
		fieldState := match.fields[i]
		switch {
		case fieldState.multiple:
			r.addIdentityFieldError(constraint, elemPath,
				"field selects multiple nodes for element at %s", constraint.Original.Name)
			return
		case fieldState.count == 0 || fieldState.missing:
			if constraint.Original.Type == types.UniqueConstraint || constraint.Original.Type == types.KeyRefConstraint {
				return
			}
			r.addIdentityFieldError(constraint, elemPath,
				"field value is absent for element at %s", constraint.Original.Name)
			return
		case fieldState.valueInvalid:
			if constraint.Original.Type == types.UniqueConstraint {
				return
			}
			r.addIdentityFieldError(constraint, elemPath,
				"field value is invalid for element at %s", constraint.Original.Name)
			return
		case fieldState.invalid || !fieldState.hasValue:
			r.addIdentityFieldError(constraint, elemPath,
				"field selects non-simple content for element at %s", constraint.Original.Name)
			return
		default:
			normalizedValues = append(normalizedValues, fieldState.value)
			displayValues = append(displayValues, fieldState.display)
		}
	}

	tuple := strings.Join(normalizedValues, "\x00")
	display := strings.Join(displayValues, ", ")

	switch constraint.Original.Type {
	case types.KeyConstraint, types.UniqueConstraint:
		keyQName := constraint.QName(scope.decl)
		table := scope.keyTables[keyQName]
		if table == nil {
			table = make(map[string]string)
			scope.keyTables[keyQName] = table
		}
		if firstPath, exists := table[tuple]; exists {
			code := errors.ErrIdentityDuplicate
			label := "unique"
			if constraint.Original.Type == types.KeyConstraint {
				label = "key"
			}
			violation := errors.NewValidationf(code, elemPath,
				"%s '%s': duplicate value '%s' at %s (first occurrence at %s)",
				label, constraint.Original.Name, display, elemPath, firstPath)
			r.addViolation(&violation)
			return
		}
		table[tuple] = elemPath
	case types.KeyRefConstraint:
		scope.keyRefs = append(scope.keyRefs, keyRefEntry{
			constraint: constraint,
			value:      tuple,
			display:    display,
			path:       elemPath,
			line:       r.currentLine,
			column:     r.currentColumn,
		})
	}
}

func (r *streamRun) closeIdentityScopes(frame *streamFrame) {
	for i := 0; i < len(r.identityScopes); {
		scope := r.identityScopes[i]
		if scope.rootID != frame.id {
			i++
			continue
		}
		if !scope.invalid {
			r.finalizeKeyRefs(scope)
		}
		r.identityScopes = append(r.identityScopes[:i], r.identityScopes[i+1:]...)
	}
}

func (r *streamRun) finalizeKeyRefs(scope *identityScope) {
	if scope == nil {
		return
	}
	entriesByConstraint := make(map[*grammar.CompiledConstraint][]keyRefEntry)
	for _, entry := range scope.keyRefs {
		entriesByConstraint[entry.constraint] = append(entriesByConstraint[entry.constraint], entry)
	}

	for _, state := range scope.constraints {
		constraint := state.constraint
		if constraint.Original.Type != types.KeyRefConstraint {
			continue
		}
		referQName := constraint.Original.ReferQName
		referName := referQName.String()
		keyTable := scope.keyTables[referQName]
		if keyTable == nil {
			elemPath := r.path.String()
			violation := errors.NewValidationf(errors.ErrIdentityKeyRefFailed, elemPath,
				"keyref '%s' refers to undefined key '%s'", constraint.Original.Name, referName)
			r.addViolation(&violation)
			continue
		}

		for _, entry := range entriesByConstraint[constraint] {
			if _, ok := keyTable[entry.value]; !ok {
				violation := errors.NewValidationf(errors.ErrIdentityKeyRefFailed, entry.path,
					"keyref '%s': value '%s' not found in referenced key '%s' at %s",
					constraint.Original.Name, entry.display, referName, entry.path)
				r.addViolationAt(&violation, entry.line, entry.column)
			}
		}
	}
}

func (r *streamRun) abortIdentityFrame(frame *streamFrame) {
	for _, capture := range frame.fieldCaptures {
		capture.match.invalid = true
	}
	for _, scope := range r.identityScopes {
		for _, state := range scope.constraints {
			delete(state.selectorMatches, frame.id)
		}
		if scope.rootID == frame.id {
			scope.invalid = true
		}
	}
	r.closeIdentityScopes(frame)
}

func (r *streamRun) addIdentityFieldError(constraint *grammar.CompiledConstraint, path, message, name string) {
	switch constraint.Original.Type {
	case types.KeyConstraint:
		violation := errors.NewValidationf(errors.ErrIdentityAbsent, path, "key '%s': "+message, name, path)
		r.addViolation(&violation)
	case types.UniqueConstraint:
		violation := errors.NewValidationf(errors.ErrIdentityDuplicate, path, "unique '%s': "+message, name, path)
		r.addViolation(&violation)
	case types.KeyRefConstraint:
		violation := errors.NewValidationf(errors.ErrIdentityKeyRefFailed, path, "keyref '%s': "+message, name, path)
		r.addViolation(&violation)
	}
}

func (r *streamRun) normalizeElementValue(value string, field types.Field, frame *streamFrame, context map[string]string) (string, KeyState) {
	fieldType := field.ResolvedType
	if fieldType == nil {
		fieldType = field.Type
	}
	if (fieldType == nil || isAnySimpleOrAnyType(fieldType)) && frame != nil {
		switch {
		case frame.textType != nil && frame.textType.Original != nil:
			fieldType = frame.textType.Original
		case frame.typ != nil && frame.typ.Original != nil:
			fieldType = frame.typ.Original
		case frame.decl != nil && frame.decl.Type != nil:
			fieldType = frame.decl.Type.Original
		}
	}
	if fieldType == nil {
		fieldType = types.GetBuiltin(types.TypeName("string"))
	}
	if _, ok := fieldType.(*types.ComplexType); ok {
		return "", KeyInvalidSelection
	}
	return r.normalizeValueByTypeStream(value, fieldType, frame.scopeDepth, context)
}

func isAnySimpleOrAnyType(fieldType types.Type) bool {
	if fieldType == nil {
		return false
	}
	name := fieldType.Name()
	if name.Namespace != xsdxml.XSDNamespace {
		return false
	}
	return name.Local == "anySimpleType" || name.Local == "anyType"
}

func (r *streamRun) normalizeAttributeValue(value string, field types.Field, frame *streamFrame, attrQName types.QName, context map[string]string) (string, KeyState) {
	fieldType := field.ResolvedType
	if fieldType == nil {
		fieldType = field.Type
	}

	attrDeclared := false
	if frame != nil {
		typ := frame.typ
		if typ == nil && frame.decl != nil {
			typ = frame.decl.Type
		}
		if typ != nil {
			for _, attr := range typ.AllAttributes {
				if attr.QName.Local != attrQName.Local {
					continue
				}
				if attr.QName.Namespace != attrQName.Namespace {
					continue
				}
				attrDeclared = true
				if attr.Type != nil && attr.Type.Original != nil {
					return r.normalizeValueByTypeStream(value, attr.Type.Original, frame.scopeDepth, context)
				}
			}
			if fieldType == nil && !attrDeclared && typ.AnyAttribute != nil && typ.AnyAttribute.AllowsQName(attrQName) {
				return "", KeyInvalidSelection
			}
		}
	}

	if fieldType == nil {
		fieldType = types.GetBuiltin(types.TypeName("string"))
	}
	return r.normalizeValueByTypeStream(value, fieldType, frame.scopeDepth, context)
}

const (
	keyTypeSeparator = "\x01"
	keyListSeparator = "\x02"
)

func (r *streamRun) normalizeValueByTypeStream(value string, fieldType types.Type, scopeDepth int, context map[string]string) (string, KeyState) {
	if fieldType == nil {
		fieldType = types.GetBuiltin(types.TypeNameString)
	}
	return r.normalizeValueByType(value, fieldType, scopeDepth, context)
}

func (r *streamRun) normalizeValueByType(value string, fieldType types.Type, scopeDepth int, context map[string]string) (string, KeyState) {
	if fieldType == nil {
		fieldType = types.GetBuiltin(types.TypeNameString)
	}
	if !types.IdentityNormalizable(fieldType) {
		return "", KeyInvalidValue
	}

	if itemType, ok := types.IdentityListItemType(fieldType); ok {
		normalized, err := types.NormalizeValue(value, fieldType)
		if err != nil {
			return "", KeyInvalidValue
		}
		return r.normalizeListValue(normalized, itemType, fieldType, scopeDepth, context)
	}

	if st, ok := types.AsSimpleType(fieldType); ok && st.Variety() == types.UnionVariety {
		members := types.IdentityMemberTypes(fieldType)
		if len(members) == 0 {
			return "", KeyInvalidValue
		}
		for _, member := range members {
			if member == nil {
				continue
			}
			normalized, state := r.normalizeValueByType(value, member, scopeDepth, context)
			if state == KeyValid {
				return normalized, KeyValid
			}
		}
		return "", KeyInvalidValue
	}

	normalized, err := types.NormalizeValue(value, fieldType)
	if err != nil {
		return "", KeyInvalidValue
	}
	return r.normalizeAtomicValue(normalized, fieldType, scopeDepth, context)
}

func (r *streamRun) normalizeListValue(value string, itemType, listType types.Type, scopeDepth int, context map[string]string) (string, KeyState) {
	var (
		itemState KeyState
		builder   strings.Builder
		first     = true
	)
	itemState = KeyValid
	builder.WriteString(listKeyPrefix(listType))
	builder.WriteString(keyTypeSeparator)
	splitWhitespaceSeq(value, func(item string) bool {
		normalized, state := r.normalizeValueByType(item, itemType, scopeDepth, context)
		if state != KeyValid {
			itemState = state
			return false
		}
		if !first {
			builder.WriteString(keyListSeparator)
		}
		first = false
		builder.WriteString(normalized)
		return true
	})
	if itemState != KeyValid {
		return "", itemState
	}
	return builder.String(), KeyValid
}

func (r *streamRun) normalizeAtomicValue(value string, fieldType types.Type, scopeDepth int, context map[string]string) (string, KeyState) {
	primitiveName := primitiveTypeName(fieldType)
	typePrefix := typeKeyPrefix(fieldType, primitiveName)

	switch primitiveName {
	case "decimal", "integer", "nonPositiveInteger", "negativeInteger",
		"nonNegativeInteger", "positiveInteger", "long", "int", "short", "byte",
		"unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte":
		rat, err := types.ParseDecimal(value)
		if err != nil {
			return "", KeyInvalidValue
		}
		return typePrefix + keyTypeSeparator + rat.String(), KeyValid
	case "float":
		floatValue, err := types.ParseFloat(value)
		if err != nil {
			return "", KeyInvalidValue
		}
		if math.IsNaN(float64(floatValue)) {
			return typePrefix + keyTypeSeparator + "NaN", KeyValid
		}
		if floatValue == 0 && math.Signbit(float64(floatValue)) {
			// normalize -0 to +0 to keep a single zero in the value space.
			floatValue = 0
		}
		return typePrefix + keyTypeSeparator + strconv.FormatFloat(float64(floatValue), 'g', -1, 32), KeyValid
	case "double":
		floatValue, err := types.ParseDouble(value)
		if err != nil {
			return "", KeyInvalidValue
		}
		if math.IsNaN(floatValue) {
			return typePrefix + keyTypeSeparator + "NaN", KeyValid
		}
		if floatValue == 0 && math.Signbit(floatValue) {
			// normalize -0 to +0 to keep a single zero in the value space.
			floatValue = 0
		}
		return typePrefix + keyTypeSeparator + strconv.FormatFloat(floatValue, 'g', -1, 64), KeyValid
	case "boolean":
		boolValue, err := types.ParseBoolean(value)
		if err != nil {
			return "", KeyInvalidValue
		}
		return typePrefix + keyTypeSeparator + strconv.FormatBool(boolValue), KeyValid
	case "dateTime":
		return normalizeTimeValue(typePrefix, value, types.ParseDateTime)
	case "date":
		return normalizeTimeValue(typePrefix, value, types.ParseDate)
	case "time":
		return normalizeTimeValue(typePrefix, value, types.ParseTime)
	case "gYear":
		return normalizeTimeValue(typePrefix, value, types.ParseGYear)
	case "gYearMonth":
		return normalizeTimeValue(typePrefix, value, types.ParseGYearMonth)
	case "gMonth":
		return normalizeTimeValue(typePrefix, value, types.ParseGMonth)
	case "gMonthDay":
		return normalizeTimeValue(typePrefix, value, types.ParseGMonthDay)
	case "gDay":
		return normalizeTimeValue(typePrefix, value, types.ParseGDay)
	case "duration":
		xsdDur, err := types.ParseXSDDuration(value)
		if err != nil {
			return "", KeyInvalidValue
		}
		normalized := types.ComparableXSDDuration{Value: xsdDur, Typ: fieldType}.String()
		return typePrefix + keyTypeSeparator + normalized, KeyValid
	case "hexBinary":
		decoded, err := types.ParseHexBinary(value)
		if err != nil {
			return "", KeyInvalidValue
		}
		return typePrefix + keyTypeSeparator + hex.EncodeToString(decoded), KeyValid
	case "base64Binary":
		decoded, err := types.ParseBase64Binary(value)
		if err != nil {
			return "", KeyInvalidValue
		}
		return typePrefix + keyTypeSeparator + base64.StdEncoding.EncodeToString(decoded), KeyValid
	case "QName", "NOTATION":
		normalized, err := r.normalizeQNameValue(value, scopeDepth, context)
		if err != nil {
			return "", KeyInvalidValue
		}
		return typePrefix + keyTypeSeparator + normalized, KeyValid
	}

	return typePrefix + keyTypeSeparator + value, KeyValid
}

func primitiveTypeName(fieldType types.Type) string {
	if fieldType == nil {
		return ""
	}
	if bt, ok := fieldType.(*types.BuiltinType); ok {
		if pt := bt.PrimitiveType(); pt != nil {
			switch prim := pt.(type) {
			case *types.BuiltinType:
				return prim.Name().Local
			case *types.SimpleType:
				return prim.QName.Local
			}
		}
		return bt.Name().Local
	}
	if pt := fieldType.PrimitiveType(); pt != nil {
		switch prim := pt.(type) {
		case *types.SimpleType:
			return prim.QName.Local
		case *types.BuiltinType:
			return prim.Name().Local
		}
	}
	return ""
}

func typeKeyPrefix(fieldType types.Type, primitiveName string) string {
	if primitiveName != "" {
		return primitiveName
	}
	if fieldType != nil {
		return fieldType.Name().String()
	}
	return ""
}

func listKeyPrefix(fieldType types.Type) string {
	return "list:" + typeKeyPrefix(fieldType, primitiveTypeName(fieldType))
}

func normalizeTimeValue(prefix, value string, parse func(string) (time.Time, error)) (string, KeyState) {
	parsed, err := parse(value)
	if err != nil {
		return "", KeyInvalidValue
	}
	tzMarker := "local"
	if types.HasTimezone(value) {
		tzMarker = "tz"
	}
	normalized := parsed.UTC().Format(time.RFC3339Nano)
	return prefix + keyTypeSeparator + tzMarker + keyTypeSeparator + normalized, KeyValid
}

func (r *streamRun) normalizeQNameValue(value string, scopeDepth int, context map[string]string) (string, error) {
	qname, err := r.parseQNameValueWithContext(value, scopeDepth, context)
	if err != nil {
		return "", err
	}
	return "{" + qname.Namespace.String() + "}" + qname.Local, nil
}

func (r *streamRun) lookupAttributeDefault(frame *streamFrame, attrQName types.QName) (string, map[string]string, bool) {
	if frame == nil {
		return "", nil, false
	}
	typ := frame.typ
	if typ == nil && frame.decl != nil {
		typ = frame.decl.Type
	}
	if typ == nil {
		return "", nil, false
	}
	for _, attr := range typ.AllAttributes {
		if attr.QName.Local != attrQName.Local {
			continue
		}
		if attr.QName.Namespace != attrQName.Namespace {
			continue
		}
		if attr.HasFixed {
			var ctx map[string]string
			if attr.Original != nil {
				ctx = attr.Original.FixedContext
			}
			return attr.Fixed, ctx, true
		}
		if attr.HasDefault {
			var ctx map[string]string
			if attr.Original != nil {
				ctx = attr.Original.DefaultContext
			}
			return attr.Default, ctx, true
		}
		return "", nil, false
	}
	return "", nil, false
}

func findAttrByNamespace(attrs []streamAttr, namespace types.NamespaceURI, local string) (streamAttr, bool) {
	for _, attr := range attrs {
		if isXMLNSAttribute(attr) {
			continue
		}
		if types.NamespaceURI(attr.NamespaceURI()) != namespace {
			continue
		}
		if attr.LocalName() == local {
			return attr, true
		}
	}
	return streamAttr{}, false
}

func matchPath(path xpath.Path, frames []streamFrame, startDepth, currentDepth int) bool {
	if startDepth < 0 || currentDepth < 0 || currentDepth >= len(frames) {
		return false
	}
	if len(path.Steps) == 0 {
		return currentDepth == startDepth
	}
	return matchSteps(path.Steps, frames, startDepth, currentDepth)
}

func matchSteps(steps []xpath.Step, frames []streamFrame, startDepth, currentDepth int) bool {
	var match func(stepIndex, nodeDepth int) bool
	match = func(stepIndex, nodeDepth int) bool {
		if nodeDepth < startDepth || nodeDepth >= len(frames) || stepIndex < 0 {
			return false
		}
		step := steps[stepIndex]
		if !nodeTestMatches(step.Test, frames[nodeDepth].qname) {
			return false
		}
		if stepIndex == 0 {
			return axisMatchesStart(step.Axis, startDepth, nodeDepth)
		}
		switch step.Axis {
		case xpath.AxisChild:
			return match(stepIndex-1, nodeDepth-1)
		case xpath.AxisSelf:
			return match(stepIndex-1, nodeDepth)
		case xpath.AxisDescendant:
			for prev := nodeDepth - 1; prev >= startDepth; prev-- {
				if match(stepIndex-1, prev) {
					return true
				}
			}
			return false
		case xpath.AxisDescendantOrSelf:
			for prev := nodeDepth; prev >= startDepth; prev-- {
				if match(stepIndex-1, prev) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}

	return match(len(steps)-1, currentDepth)
}

func axisMatchesStart(axis xpath.Axis, startDepth, nodeDepth int) bool {
	switch axis {
	case xpath.AxisChild:
		return nodeDepth == startDepth+1
	case xpath.AxisSelf:
		return nodeDepth == startDepth
	case xpath.AxisDescendant:
		return nodeDepth > startDepth
	case xpath.AxisDescendantOrSelf:
		return nodeDepth >= startDepth
	default:
		return false
	}
}

func nodeTestMatches(test xpath.NodeTest, qname types.QName) bool {
	if test.Any {
		return true
	}
	if test.Local != "*" && qname.Local != test.Local {
		return false
	}
	if test.NamespaceSpecified && qname.Namespace != test.Namespace {
		return false
	}
	return true
}
