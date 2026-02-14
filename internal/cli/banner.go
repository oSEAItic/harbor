package cli

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/oseaitic/harbor/internal/connector"
	"github.com/oseaitic/harbor/internal/registry"
)

// ANSI color codes
const (
	cCyan  = "\033[36m"
	cBlue  = "\033[34m"
	cBold  = "\033[1m"
	cDim   = "\033[2m"
	cReset = "\033[0m"
	cWhite = "\033[97m"
	cGreen = "\033[32m"
)

func printBanner(version string) {
	installed, _ := connector.ListInstalled()
	catalog := registry.ListCatalog()

	// Connector status
	status := fmt.Sprintf("%d installed", len(installed))
	if avail := len(catalog) - len(installed); avail > 0 {
		status += fmt.Sprintf(", %d available", avail)
	}

	// Working directory (shortened)
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(cwd, home) {
		cwd = "~" + cwd[len(home):]
	}

	// Installed connector names
	names := "(none)"
	if len(installed) > 0 {
		names = strings.Join(installed, ", ")
	}

	platform := runtime.GOOS + "/" + runtime.GOARCH

	lines := []string{
		"",
		cBlue + "   ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~" + cReset,
		cCyan + "    ⚓  ╦ ╦╔═╗╦═╗╔╗ ╔═╗╦═╗" + cReset,
		cCyan + "       ╠═╣╠═╣╠╦╝╠╩╗║ ║╠╦╝" + cReset + "   " + cBold + cWhite + "Harbor v" + version + cReset,
		cCyan + "       ╩ ╩╩ ╩╩╚═╚═╝╚═╝╩╚═" + cReset + "   " + cDim + platform + cReset + " · " + cGreen + status + cReset,
		"                              " + cDim + cwd + cReset,
		cBlue + "   ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~" + cReset,
		"",
		"   " + cBold + "Connectors:" + cReset + " " + names,
		"   " + cDim + "Run" + cReset + " " + cBold + "harbor get <connector.resource>" + cReset + " " + cDim + "to fetch live data" + cReset,
		"   " + cDim + "Run" + cReset + " " + cBold + "harbor tools export" + cReset + " " + cDim + "for LLM/agent integration" + cReset,
		"",
	}

	fmt.Fprintln(os.Stderr, strings.Join(lines, "\n"))
}
