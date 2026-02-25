package main

import (
	"fmt"
	"os"

	"github.com/oseaitic/harbor/internal/cli"
)

var version = "dev"
var sourceDir = "" // set via -ldflags at build time

func main() {
	root := cli.NewRootCmd(version, sourceDir)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
