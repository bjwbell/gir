package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/bjwbell/gir/config"
	"github.com/bjwbell/gir/exec"
	"github.com/bjwbell/gir/parse"
	"github.com/bjwbell/gir/scan"
	"github.com/bjwbell/gir/token"
	"github.com/bjwbell/gir/value"
	"github.com/bjwbell/gir/codegen"
)

var (
	conf    config.Config
	context value.Context
)

func filePath(pathName string) string {
	split := strings.Split(pathName, "/")
	dir := ""
	if len(split) == 1 {
		dir = "."
	} else if len(split) == 2 {
		dir = split[0] + "/"
	} else {
		dir = strings.Join(split[0:len(split)-2], "/")
	}
	return dir
}

func main() {

	context = exec.NewContext(&conf)

	var f = flag.String("f", "", "input *.gir file ")
	flag.Parse()

	file := ""
	log.SetFlags(log.Lshortfile)
	if *f != "" {
		file = *f
	} else {
		log.Fatalf("Error no file provided")
	}

	var fd io.Reader
	var err error
	fd, err = os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gir: %s\n", err)
		os.Exit(1)
	}
	var fd2 io.Reader
	var err2 error
	fd2, err2 = os.Open(file)
	if err2 != nil {
		fmt.Fprintf(os.Stderr, "gir: %s\n", err2)
		os.Exit(1)
	}
	scanner1 := scan.New(context, file, bufio.NewReader(fd))
	scanner2 := scan.New(context, file, bufio.NewReader(fd2))

	fmt.Println("Tokens: ")
	for tok := scanner1.Next(); tok.Type != token.EOF; tok = scanner1.Next() {
		fmt.Println(tok, "type ", tok.Type)
	}
	parser := parse.NewParser(file, scanner2, context)
	fileDecl := parser.ParseFile()
	fmt.Println("tree(exprs): ", parse.Tree(fileDecl))

	for _, fnDecl := range fileDecl.Decls {
		ssafn, ok := codegen.BuildSSA(&fnDecl, fileDecl.Name, false)
		if ssafn == nil || !ok {
			fmt.Println("Error building SSA form")
			return
		} else {
			fmt.Println("ssa:\n", ssafn)
		}
	}
}
