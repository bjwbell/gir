package main

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"

	"github.com/bjwbell/gir/codegen"
	"github.com/bjwbell/gir/config"
	"github.com/bjwbell/gir/ctx"
	"github.com/bjwbell/gir/parse"
	"github.com/bjwbell/gir/scan"
	"github.com/bjwbell/gir/value"
)

func runTest(t *testing.T, filename string) {
	context := ctx.NewContext(&conf)
	fd, err := os.Open(filepath.Join("testdata", filename))
	defer fd.Close()
	if err != nil {
		t.Fatalf("gir: %s\n", err)
	}
	scanner := scan.New(context, filename, bufio.NewReader(fd))
	parser := parse.NewParser(filename, scanner, context)
	fileDecl := parser.ParseFile()
	for _, fnDecl := range fileDecl.Decls {
		ssafn, ok := codegen.BuildSSA(&fnDecl, fileDecl.PkgName, false)
		if ssafn == nil || !ok {
			t.Fatalf("gir: Error building SSA form")
			return
		} else {
			t.Log("ssa:\n", ssafn)
		}
	}
}

// TestEmptyFile tests lexing and parsing of an empty gir file"
func TestEmptyFile(t *testing.T) { runTest(t, "empty.gir") }

func TestGir(t *testing.T) {
	var (
		conf    config.Config
		context value.Context
		fd      *os.File
		err     error
	)
	context = ctx.NewContext(&conf)
	for _, file := range []string{filepath.Join("testdata", "test.gir"), filepath.Join("testdata", "test1.gir"), filepath.Join("testdata", "test2.gir"), filepath.Join("testdata", "test3.gir"), filepath.Join("testdata", "test4.gir")} {
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
			ssafn, ok := codegen.BuildSSA(&fnDecl, fileDecl.PkgName, false)
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
