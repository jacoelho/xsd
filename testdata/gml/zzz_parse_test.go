package main

import (
	"flag"
	"testing"
)

func TestBindCommonFlagsParse(t *testing.T) {
	fs := flag.NewFlagSet("prepare", flag.ContinueOnError)
	cfg := bindCommonFlags(fs)
	args := []string{"-skip-download", "-gml-path", "/tmp/x.gml", "-xsd-dir", "/tmp/x"}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse: %v", err)
	}
	t.Logf("args=%v leftover=%v", args, fs.Args())
	t.Logf("nflag=%d skip=%v gml=%q xsd=%q", fs.NFlag(), cfg.skipDownload, cfg.gmlPath, cfg.xsdDir)
	t.Logf("flag skip=%#v", fs.Lookup("skip-download"))
	t.Logf("flag gml=%#v value=%q", fs.Lookup("gml-path"), fs.Lookup("gml-path").Value.String())
	t.Logf("flag xsd=%#v value=%q", fs.Lookup("xsd-dir"), fs.Lookup("xsd-dir").Value.String())
	if !cfg.skipDownload {
		t.Fatalf("skipDownload not set")
	}
}
