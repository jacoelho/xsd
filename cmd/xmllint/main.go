package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/errors"
)

func main() {
	os.Exit(run())
}

func run() int {
	schemaPath := flag.String("schema", "", "path to XSD schema file")
	cpuProfilePath := flag.String("cpuprofile", "", "write CPU profile to file")
	memProfilePath := flag.String("memprofile", "", "write memory profile to file")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s --schema <schema.xsd> <document.xml>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Validates an XML document against an XSD schema.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *schemaPath == "" {
		fmt.Fprintln(os.Stderr, "error: --schema is required")
		flag.Usage()
		return 2
	}

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "error: exactly one XML file argument is required")
		flag.Usage()
		return 2
	}
	xmlPath := args[0]

	if *cpuProfilePath != "" {
		stopCPUProfile, err := startCPUProfile(*cpuProfilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error starting CPU profile: %v\n", err)
			return 1
		}
		defer func() {
			if err := stopCPUProfile(); err != nil {
				fmt.Fprintf(os.Stderr, "error stopping CPU profile: %v\n", err)
			}
		}()
	}

	if *memProfilePath != "" {
		defer func() {
			if err := writeMemProfile(*memProfilePath); err != nil {
				fmt.Fprintf(os.Stderr, "error writing memory profile: %v\n", err)
			}
		}()
	}

	schema, err := xsd.LoadFile(*schemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading schema: %v\n", err)
		return 1
	}

	if err := schema.ValidateFile(xmlPath); err != nil {
		if violations, ok := errors.AsValidations(err); ok {
			for _, v := range violations {
				fmt.Fprintln(os.Stderr, v.Error())
			}
			fmt.Fprintf(os.Stderr, "%s fails to validate\n", xmlPath)
			return 1
		}
		fmt.Fprintf(os.Stderr, "error validating: %v\n", err)
		return 1
	}

	fmt.Printf("%s validates\n", xmlPath)
	return 0
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
