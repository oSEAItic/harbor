package main

import (
	"fmt"
	"os"

	"github.com/oseaitic/harbor/internal/mcpserver"
)

var version = "dev"

func main() {
	if err := mcpserver.Serve(version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
