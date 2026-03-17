package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/jacoelho/xsd"
	xsderrors "github.com/jacoelho/xsd/errors"
)

func main() {
	os.Exit(run())
}

func run() int {
	return runWithArgs(os.Args[0], os.Args[1:], os.Stdout, os.Stderr)
}

func runWithArgs(programName string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("xmllint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	schemaPath := fs.String("schema", "", "path to XSD schema file")
	instanceMaxTokenSize := fs.Int("instance-max-token-size", 0, "instance XML max token size in bytes (0 uses default)")
	cpuProfilePath := fs.String("cpuprofile", "", "write CPU profile to file")
	memProfilePath := fs.String("memprofile", "", "write memory profile to file")
	if programName == "" {
		programName = fs.Name()
	}
	var usageErr error
	fs.Usage = func() {
		usageErr = errors.Join(
			usageErr,
			writef(stderr, "Usage: %s --schema <schema.xsd> <document.xml>\n\n", programName),
			writeln(stderr, "Validates an XML document against an XSD schema."),
			writeln(stderr),
			writeln(stderr, "Options:"),
		)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *schemaPath == "" {
		if err := writeln(stderr, "error: --schema is required"); err != nil {
			return 1
		}
		fs.Usage()
		if usageErr != nil {
			return 1
		}
		return 2
	}

	remaining := fs.Args()
	if len(remaining) != 1 {
		if err := writeln(stderr, "error: exactly one XML file argument is required"); err != nil {
			return 1
		}
		fs.Usage()
		if usageErr != nil {
			return 1
		}
		return 2
	}
	xmlPath := remaining[0]
	if *instanceMaxTokenSize < 0 {
		if err := writeln(stderr, "error: --instance-max-token-size must be >= 0"); err != nil {
			return 1
		}
		return 2
	}

	if *cpuProfilePath != "" {
		stopCPUProfile, err := startCPUProfile(*cpuProfilePath)
		if err != nil {
			if writeErr := writef(stderr, "error starting CPU profile: %v\n", err); writeErr != nil {
				return 1
			}
			return 1
		}
		defer func() {
			if err := stopCPUProfile(); err != nil {
				_ = writef(stderr, "error stopping CPU profile: %v\n", err)
			}
		}()
	}

	if *memProfilePath != "" {
		defer func() {
			if err := writeMemProfile(*memProfilePath); err != nil {
				_ = writef(stderr, "error writing memory profile: %v\n", err)
			}
		}()
	}

	loadOpts := xsd.NewLoadOptions()
	if *instanceMaxTokenSize > 0 {
		loadOpts = loadOpts.WithRuntimeOptions(
			xsd.NewRuntimeOptions().WithInstanceMaxTokenSize(*instanceMaxTokenSize),
		)
	}
	schema, err := xsd.LoadFileWithOptions(*schemaPath, loadOpts)
	if err != nil {
		if writeErr := writef(stderr, "error loading schema: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}

	if err := schema.ValidateFile(xmlPath); err != nil {
		if violations, ok := xsderrors.AsValidations(err); ok {
			for _, v := range violations {
				if writeErr := writeln(stderr, v.Error()); writeErr != nil {
					return 1
				}
			}
			if writeErr := writef(stderr, "%s fails to validate\n", xmlPath); writeErr != nil {
				return 1
			}
			return 1
		}
		if writeErr := writef(stderr, "error validating: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}

	if err := writef(stdout, "%s validates\n", xmlPath); err != nil {
		return 1
	}
	return 0
}

func writef(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format, args...)
	return err
}

func writeln(w io.Writer, args ...any) error {
	_, err := fmt.Fprintln(w, args...)
	return err
}

func startCPUProfile(path string) (func() error, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create cpu profile %s: %w", path, err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return nil, fmt.Errorf("start cpu profile %s: %w (close failed: %w)", path, err, closeErr)
		}
		return nil, fmt.Errorf("start cpu profile %s: %w", path, err)
	}
	return func() error {
		pprof.StopCPUProfile()
		if err := f.Close(); err != nil {
			return fmt.Errorf("close cpu profile %s: %w", path, err)
		}
		return nil
	}, nil
}

func writeMemProfile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create mem profile %s: %w", path, err)
	}
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return fmt.Errorf("write mem profile %s: %w (close failed: %w)", path, err, closeErr)
		}
		return fmt.Errorf("write mem profile %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close mem profile %s: %w", path, err)
	}
	return nil
}
