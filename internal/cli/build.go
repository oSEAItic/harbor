package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/oseaitic/harbor/internal/connector"
	"github.com/oseaitic/harbor/internal/registry"
	"github.com/spf13/cobra"
)

func newBuildCmd() *cobra.Command {
	var (
		install  bool
		external []string
	)

	cmd := &cobra.Command{
		Use:   "build [name-or-path]",
		Short: "Build a connector from source",
		Long: `Build a connector by running npm install and esbuild bundle.

With no arguments, builds the connector in the current directory.
With a name, looks for connectors/<name>/.
With a path (contains / or .), uses it directly.

  --install              Also install after building
  --external <pkg>       Mark npm packages as external in esbuild

Examples:
  harbor build                         Build current directory as connector
  harbor build newsapi                 Build connectors/newsapi/
  harbor build ./my-connector          Build from path
  harbor build newsapi --install       Build and install`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			connectorDir, name, err := resolveConnectorDir(args)
			if err != nil {
				return err
			}

			srcFile := filepath.Join(connectorDir, "src", "index.ts")
			if _, err := os.Stat(srcFile); os.IsNotExist(err) {
				return fmt.Errorf("source file not found: %s\nNot a Harbor connector directory?", srcFile)
			}

			distDir := filepath.Join(connectorDir, "dist")
			if err := os.MkdirAll(distDir, 0o755); err != nil {
				return fmt.Errorf("creating dist dir: %w", err)
			}

			outFile := filepath.Join(distDir, name+".js")

			// Step 1: npm install
			fmt.Fprintf(cmd.OutOrStdout(), "Installing dependencies...\n")
			npmCmd := exec.Command("npm", "install")
			npmCmd.Dir = connectorDir
			npmCmd.Stdout = cmd.OutOrStdout()
			npmCmd.Stderr = cmd.ErrOrStderr()
			if err := npmCmd.Run(); err != nil {
				return fmt.Errorf("npm install failed: %w", err)
			}

			// Step 2: esbuild bundle
			fmt.Fprintf(cmd.OutOrStdout(), "Bundling with esbuild...\n")

			esbuildArgs := []string{
				srcFile,
				"--bundle",
				"--platform=node",
				"--target=node18",
				"--format=cjs",
				"--outfile=" + outFile,
				"--banner:js=#!/usr/bin/env node",
			}

			for _, ext := range external {
				esbuildArgs = append(esbuildArgs, "--external:"+ext)
			}

			esbuildCmd := exec.Command("esbuild", esbuildArgs...)
			esbuildCmd.Dir = connectorDir
			esbuildCmd.Stdout = cmd.OutOrStdout()
			esbuildCmd.Stderr = cmd.ErrOrStderr()
			if err := esbuildCmd.Run(); err != nil {
				return fmt.Errorf("esbuild bundle failed: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Built: %s\n", outFile)

			// Step 3: Validate --describe output
			fmt.Fprintf(cmd.OutOrStdout(), "Validating connector...\n")
			if err := validateConnector(outFile, connectorDir); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: validation failed: %v\n", err)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Validation passed.\n")
			}

			// Step 4: Install if requested
			if install {
				fmt.Fprintf(cmd.OutOrStdout(), "Installing %s...\n", name)
				if err := registry.InstallFromLocal(name, outFile, registry.InstallOptions{}); err != nil {
					return fmt.Errorf("installing %s: %w", name, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Done! Try: harbor get %s.<resource>\n", name)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&install, "install", false, "Also install after building")
	cmd.Flags().StringArrayVar(&external, "external", nil, "Mark npm packages as external in esbuild")

	return cmd
}

// resolveConnectorDir figures out the connector directory from the build args.
// No args: current directory must be a connector (has src/index.ts).
// Path arg (contains / or .): use as-is.
// Name arg: look for ./connectors/<name>/.
func resolveConnectorDir(args []string) (connectorDir, name string, err error) {
	if len(args) == 0 {
		// No args: use current directory
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("getting working directory: %w", err)
		}
		name = filepath.Base(cwd)
		return cwd, name, nil
	}

	arg := args[0]

	// If arg looks like a path, use it directly
	if strings.Contains(arg, "/") || strings.Contains(arg, ".") {
		absPath, err := filepath.Abs(arg)
		if err != nil {
			return "", "", fmt.Errorf("resolving path: %w", err)
		}
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return "", "", fmt.Errorf("directory not found: %s", absPath)
		}
		name = filepath.Base(absPath)
		return absPath, name, nil
	}

	// Name: look for ./connectors/<name>/
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("getting working directory: %w", err)
	}
	connectorDir = filepath.Join(cwd, "connectors", arg)
	if _, err := os.Stat(connectorDir); os.IsNotExist(err) {
		return "", "", fmt.Errorf("connector not found: %s\nRun 'harbor scaffold %s' or use 'harbor build' from the connector directory", connectorDir, arg)
	}
	return connectorDir, arg, nil
}

// validateConnector runs the built connector with --describe and checks the output.
func validateConnector(binPath, connectorDir string) error {
	cmd := exec.Command("node", binPath, "--describe")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running --describe: %w\nstderr: %s", err, stderr.String())
	}

	// Check valid JSON array
	var schemas []json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &schemas); err != nil {
		return fmt.Errorf("--describe output is not a valid JSON array: %w", err)
	}

	if len(schemas) == 0 {
		return fmt.Errorf("--describe returned empty array (no tool schemas)")
	}

	// Check each schema has required fields
	for i, raw := range schemas {
		var schema struct {
			Type     string `json:"type"`
			Function struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Parameters  json.RawMessage `json:"parameters"`
			} `json:"function"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			return fmt.Errorf("schema[%d]: invalid JSON: %w", i, err)
		}
		if schema.Type != "function" {
			return fmt.Errorf("schema[%d]: type must be \"function\", got %q", i, schema.Type)
		}
		if schema.Function.Name == "" {
			return fmt.Errorf("schema[%d]: function.name is required", i)
		}
		if schema.Function.Description == "" {
			return fmt.Errorf("schema[%d]: function.description is required", i)
		}
		if len(schema.Function.Parameters) == 0 {
			return fmt.Errorf("schema[%d]: function.parameters is required", i)
		}
	}

	// Optional contract suite for executable protocol checks.
	contractPath := filepath.Join(connectorDir, "contract.tests.json")
	if _, err := os.Stat(contractPath); err == nil {
		suite, err := connector.LoadContractSuite(contractPath)
		if err != nil {
			return fmt.Errorf("loading contract suite: %w", err)
		}
		if err := connector.RunContractSuite(binPath, suite); err != nil {
			return fmt.Errorf("running contract suite: %w", err)
		}
	}

	return nil
}
