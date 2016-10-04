package main

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bjwbell/gir/codegen"
	"github.com/bjwbell/gir/config"
	"github.com/bjwbell/gir/ctx"
	"github.com/bjwbell/gir/parse"
	"github.com/bjwbell/gir/scan"
	"github.com/bjwbell/gir/value"
)

func buildTest(t *testing.T, filename string) {
	doTest(t, filename, "build")
}

func runTest(t *testing.T, filename string) {
	doTest(t, filename, "run")
}

func doTest(t *testing.T, filename string, kind string) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", kind, filepath.Join("testdata", filename))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed: %v:\nOut: %s\nStderr: %s\n", err, &stdout, &stderr)
	}
}

func TestGir(t *testing.T) {
	var (
		conf    config.Config
		context value.Context
		fd      *os.File
		err     error
	)
	context = ctx.NewContext(&conf)
	for _, file := range []string{"test.gir", "test1.gir", "test2.gir", "test3.gir", "test4.gir"} {
		fd, err = os.Open(file)
		defer fd.Close()
		if err != nil {
			t.Fatalf("gir: %s\n", err)
		}
		scanner := scan.New(context, file, bufio.NewReader(fd))
		parser := parse.NewParser(file, scanner, context)
		fileDecl := parser.ParseFile()
		t.Log("tree(exprs): ", parse.Tree(fileDecl))

		for _, fnDecl := range fileDecl.Decls {
			ssafn, ok := codegen.BuildSSA(&fnDecl, fileDecl.Name, false)
			if ssafn == nil || !ok {
				t.Fatalf("gir: Error building SSA form")
				return
			} else {
				t.Log("ssa:\n", ssafn)
			}
		}
	}

	return
}
