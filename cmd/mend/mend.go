package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"j4k.co/mender"
)

func main() {
	var (
		mendFile        = flag.String("f", "mend.json", "json file containing mend specs")
		mendVersionFile = flag.String("o", "mend-versions.json", "output json file with versioning info for each spec to be used by the web app")
	)
	flag.Usage = func() {
		fmt.Println("Simple tool for processing and versioning js/css.")
		fmt.Println("usage: mend [-f <spec file>] [-o <output version file>] [output dir]")
		flag.PrintDefaults()
		fmt.Println("")
		fmt.Println("TODO: explain spec and version files")
	}
	flag.Parse()
	log.SetFlags(0)

	outputdir := flag.Arg(0)
	if outputdir == "" {
		outputdir = "_build"
	}

	_, err := mender.Build(*mendFile, *mendVersionFile, outputdir, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}
}
