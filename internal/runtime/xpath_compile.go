package runtime

// CompilePrograms compiles a restricted XPath expression into runtime path programs.
func CompilePrograms(expr string, nsContext map[string]string, policy AttributePolicy, schema *Schema) ([]PathProgram, error) {
	if schema == nil {
		return nil, xpathErrorf("schema is nil")
	}
	parsed, err := Parse(expr, nsContext, policy)
	if err != nil {
		return nil, err
	}
	return CompileExpression(parsed, schema)
}

// CompileExpression compiles a parsed restricted XPath expression into runtime path programs.
func CompileExpression(parsed Expression, schema *Schema) ([]PathProgram, error) {
	if schema == nil {
		return nil, xpathErrorf("schema is nil")
	}
	if len(parsed.Paths) == 0 {
		return nil, xpathErrorf("xpath contains no paths")
	}
	programs := make([]PathProgram, 0, len(parsed.Paths))
	for _, path := range parsed.Paths {
		program, err := compilePathProgram(path, schema)
		if err != nil {
			return nil, err
		}
		programs = append(programs, program)
	}
	return programs, nil
}

func compilePathProgram(path Path, schema *Schema) (PathProgram, error) {
	if len(path.Steps) == 0 && path.Attribute == nil {
		return PathProgram{}, xpathErrorf("xpath must contain at least one step")
	}
	ops := make([]PathOp, 0, len(path.Steps)+1)

	for _, step := range path.Steps {
		if isDescendStep(step) {
			ops = append(ops, PathOp{Op: OpDescend})
			continue
		}
		if step.Axis == AxisSelf && step.Test.Any {
			if len(ops) == 0 {
				ops = append(ops, PathOp{Op: OpRootSelf})
			} else {
				ops = append(ops, PathOp{Op: OpSelf})
			}
			continue
		}
		if step.Axis == AxisDescendant || step.Axis == AxisDescendantOrSelf {
			return PathProgram{}, xpathErrorf("xpath uses unsupported descendant axis")
		}
		if step.Axis != AxisChild {
			return PathProgram{}, xpathErrorf("xpath uses unsupported axis")
		}
		op, err := compileNodeTest(step.Test, schema, false)
		if err != nil {
			return PathProgram{}, err
		}
		ops = append(ops, op)
	}

	if path.Attribute != nil {
		op, err := compileNodeTest(*path.Attribute, schema, true)
		if err != nil {
			return PathProgram{}, err
		}
		ops = append(ops, op)
	}

	return PathProgram{Ops: ops}, nil
}

func isDescendStep(step Step) bool {
	return step.Axis == AxisDescendantOrSelf && step.Test.Any
}

func compileNodeTest(test NodeTest, schema *Schema, attribute bool) (PathOp, error) {
	test = CanonicalizeNodeTest(test)
	if test.Any {
		if attribute {
			return PathOp{Op: OpAttrAny}, nil
		}
		return PathOp{Op: OpChildAny}, nil
	}

	if test.Local == "*" {
		if !test.NamespaceSpecified {
			return PathOp{}, xpathErrorf("xpath wildcard namespace missing prefix")
		}
		nsID, err := resolveNamespace(schema, test.Namespace)
		if err != nil {
			return PathOp{}, err
		}
		if attribute {
			return PathOp{Op: OpAttrNSAny, NS: nsID}, nil
		}
		return PathOp{Op: OpChildNSAny, NS: nsID}, nil
	}

	nsURI := ""
	if test.NamespaceSpecified {
		nsURI = test.Namespace
	}
	nsID, err := resolveNamespace(schema, nsURI)
	if err != nil {
		return PathOp{}, err
	}

	sym := schema.Symbols.Lookup(nsID, []byte(test.Local))
	if sym == 0 {
		return PathOp{}, xpathErrorf("xpath name %q not found in schema", test.Local)
	}
	if attribute {
		return PathOp{Op: OpAttrName, Sym: sym, NS: nsID}, nil
	}
	return PathOp{Op: OpChildName, Sym: sym, NS: nsID}, nil
}

func resolveNamespace(schema *Schema, uri string) (NamespaceID, error) {
	if schema == nil {
		return 0, xpathErrorf("xpath namespace resolve missing schema")
	}
	if uri == "" && schema.PredefNS.Empty != 0 {
		return schema.PredefNS.Empty, nil
	}
	nsID := schema.Namespaces.Lookup([]byte(uri))
	if nsID == 0 {
		return 0, xpathErrorf("xpath namespace %q not found in schema", uri)
	}
	return nsID, nil
}
