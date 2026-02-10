package xpath

import "github.com/jacoelho/xsd/internal/runtime"

// CompilePrograms compiles a restricted XPath expression into runtime path programs.
func CompilePrograms(expr string, nsContext map[string]string, policy AttributePolicy, schema *runtime.Schema) ([]runtime.PathProgram, error) {
	if schema == nil {
		return nil, xpathErrorf("schema is nil")
	}
	parsed, err := Parse(expr, nsContext, policy)
	if err != nil {
		return nil, err
	}
	if len(parsed.Paths) == 0 {
		return nil, xpathErrorf("xpath contains no paths: %s", expr)
	}
	programs := make([]runtime.PathProgram, 0, len(parsed.Paths))
	for _, path := range parsed.Paths {
		program, err := compilePathProgram(path, schema)
		if err != nil {
			return nil, err
		}
		programs = append(programs, program)
	}
	return programs, nil
}

func compilePathProgram(path Path, schema *runtime.Schema) (runtime.PathProgram, error) {
	if len(path.Steps) == 0 && path.Attribute == nil {
		return runtime.PathProgram{}, xpathErrorf("xpath must contain at least one step")
	}
	ops := make([]runtime.PathOp, 0, len(path.Steps)+1)

	for _, step := range path.Steps {
		if isDescendStep(step) {
			ops = append(ops, runtime.PathOp{Op: runtime.OpDescend})
			continue
		}
		if step.Axis == AxisSelf && step.Test.Any {
			if len(ops) == 0 {
				ops = append(ops, runtime.PathOp{Op: runtime.OpRootSelf})
			} else {
				ops = append(ops, runtime.PathOp{Op: runtime.OpSelf})
			}
			continue
		}
		if step.Axis == AxisDescendant || step.Axis == AxisDescendantOrSelf {
			return runtime.PathProgram{}, xpathErrorf("xpath uses unsupported descendant axis")
		}
		if step.Axis != AxisChild {
			return runtime.PathProgram{}, xpathErrorf("xpath uses unsupported axis")
		}
		op, err := compileNodeTest(step.Test, schema, false)
		if err != nil {
			return runtime.PathProgram{}, err
		}
		ops = append(ops, op)
	}

	if path.Attribute != nil {
		op, err := compileNodeTest(*path.Attribute, schema, true)
		if err != nil {
			return runtime.PathProgram{}, err
		}
		ops = append(ops, op)
	}

	return runtime.PathProgram{Ops: ops}, nil
}

func isDescendStep(step Step) bool {
	return step.Axis == AxisDescendantOrSelf && step.Test.Any
}

func compileNodeTest(test NodeTest, schema *runtime.Schema, attribute bool) (runtime.PathOp, error) {
	test = CanonicalizeNodeTest(test)
	if test.Any {
		if attribute {
			return runtime.PathOp{Op: runtime.OpAttrAny}, nil
		}
		return runtime.PathOp{Op: runtime.OpChildAny}, nil
	}

	if test.Local == "*" {
		if !test.NamespaceSpecified {
			return runtime.PathOp{}, xpathErrorf("xpath wildcard namespace missing prefix")
		}
		nsID, err := resolveNamespace(schema, test.Namespace)
		if err != nil {
			return runtime.PathOp{}, err
		}
		if attribute {
			return runtime.PathOp{Op: runtime.OpAttrNSAny, NS: nsID}, nil
		}
		return runtime.PathOp{Op: runtime.OpChildNSAny, NS: nsID}, nil
	}

	nsURI := ""
	if test.NamespaceSpecified {
		nsURI = test.Namespace
	}
	nsID, err := resolveNamespace(schema, nsURI)
	if err != nil {
		return runtime.PathOp{}, err
	}

	sym := schema.Symbols.Lookup(nsID, []byte(test.Local))
	if sym == 0 {
		return runtime.PathOp{}, xpathErrorf("xpath name %q not found in schema", test.Local)
	}
	if attribute {
		return runtime.PathOp{Op: runtime.OpAttrName, Sym: sym, NS: nsID}, nil
	}
	return runtime.PathOp{Op: runtime.OpChildName, Sym: sym, NS: nsID}, nil
}

func resolveNamespace(schema *runtime.Schema, uri string) (runtime.NamespaceID, error) {
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
