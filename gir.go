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
	"github.com/bjwbell/gir/run"
	"github.com/bjwbell/gir/scan"
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

	var pkgName = flag.String("pkg", "", "package name")
	var f = flag.String("f", "", "input *.gir file ")
	var fn = flag.String("fn", "", "function name")
	flag.Parse()

	file := ""
	log.SetFlags(log.Lshortfile)
	if *f != "" {
		file = *f
	} else {
		log.Fatalf("Error no file provided")
	}
	if *fn == "" {
		log.Fatalf("Error no function name(s) provided")
	}
	if *pkgName == "" {
		*pkgName = filePath(file)
	}

	var fd io.Reader
	var err error
	fd, err = os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gir: %s\n", err)
		os.Exit(1)
	}
	scanner := scan.New(context, file, bufio.NewReader(fd))
	fmt.Println("Tokens: ")
	for tok := scanner.Next(); tok.Type != scan.EOF; tok = scanner.Next() {
		fmt.Println(tok, "type ", tok.Type)
	}
	parser := parse.NewParser(file, scanner, context)
	interactive := false
	ok := run.Run(parser, context, interactive)
	if !ok {
		fmt.Println("Error in run.Run")
	} else {
		fmt.Printf("parser: %#v\n", parser)
	}
}
