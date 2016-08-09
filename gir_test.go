package main

import (
	"bufio"
	"io"
	"os"
	"testing" 

	"github.com/bjwbell/gir/config"
	"github.com/bjwbell/gir/exec"
	"github.com/bjwbell/gir/parse"
	"github.com/bjwbell/gir/scan"
	"github.com/bjwbell/gir/value"
)

func TestGir(t *testing.T) {
	var (
		conf    config.Config
		context value.Context
		fd      io.Reader
		err     error
	)
	context = exec.NewContext(&conf)
	file := "test.gir"
	fd, err = os.Open(file)
	if err != nil {
		t.Fatalf("gir: %s\n", err)
	}
	scanner := scan.New(context, file, bufio.NewReader(fd))
	parser := parse.NewParser(file, scanner, context)
	t.Logf("parser: %#v\n", parser)
	return
}
