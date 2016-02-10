package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/bjwbell/gir/parse"
	"github.com/bjwbell/gir/scan"
	"github.com/bjwbell/gir/value"
)

var (
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

	scanner := scan.New(context, "<args>", strings.NewReader(strings.Join(flag.Args(), " ")))
	parser := parse.NewParser("<args>", scanner, context)

	fmt.Printf("parser: %#v\n", parser)
}
