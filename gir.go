package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/bjwbell/gir/codegen"
	"github.com/bjwbell/gir/config"
	"github.com/bjwbell/gir/exec"
	"github.com/bjwbell/gir/parse"
	"github.com/bjwbell/gir/scan"
	"github.com/bjwbell/gir/token"
	"github.com/bjwbell/gir/value"
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
	var o = flag.String("o", "", "output *.s file ")
	flag.Parse()

	file := ""
	outfile := ""
	log.SetFlags(log.Lshortfile)
	if *f != "" {
		file = *f
	} else {
		log.Fatalf("Error no file provided")
	}
	if *o != "" {
		outfile = *o
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
	asm := ""
	for _, fnDecl := range fileDecl.Decls {
		ssafn, ok := codegen.BuildSSA(&fnDecl, fileDecl.Name, false)
		if ssafn == nil || !ok {
			fmt.Println("Error building SSA form")
			return
		} else {
			fmt.Println("ssa:\n", ssafn)
			fnProgs, success := codegen.GenProg(ssafn)
			if !success {
				fmt.Println("Error generating assembly")
			} else {
				for _, p := range fnProgs {
					asm += p.Sprint(false) + "\n"
				}
			}
		}
	}

	if outfile != "" {
		err = ioutil.WriteFile(outfile, []byte(asm), 0644)
		if err != nil {
			panic(err)
		}
	}
}
