package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oseaitic/harbor/internal/connector"
	"github.com/oseaitic/harbor/internal/protocol"
	"github.com/spf13/cobra"
)

func newToolsCmd() *cobra.Command {
	var (
		format       string
		withExamples bool
	)

	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Tool schema utilities for LLM/agent integration",
	}

	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export tool schemas for all installed connectors",
		RunE: func(cmd *cobra.Command, args []string) error {
			format = strings.ToLower(strings.TrimSpace(format))

			var (
				out []byte
				err error
			)

			switch format {
			case "", "openai":
				schemas, exportErr := connector.ExportToolSchemas()
				if exportErr != nil {
					return fmt.Errorf("exporting openai tool schemas: %w", exportErr)
				}
				if withExamples {
					payload := map[string]any{
						"format":                   "openai",
						"tools":                    schemas,
						"response_envelope_schema": responseEnvelopeSchemaExample(),
						"examples":                 toolExamples(schemas),
					}
					out, err = json.MarshalIndent(payload, "", "  ")
				} else {
					out, err = json.MarshalIndent(schemas, "", "  ")
				}
			case "mcp":
				schemas, exportErr := connector.ExportMCPToolSchemas()
				if exportErr != nil {
					return fmt.Errorf("exporting mcp tool schemas: %w", exportErr)
				}
				out, err = json.MarshalIndent(schemas, "", "  ")
			default:
				return fmt.Errorf("unsupported format %q (supported: openai, mcp)", format)
			}
			if err != nil {
				return fmt.Errorf("marshaling schemas: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	exportCmd.Flags().StringVar(&format, "format", "openai", "Export format: openai | mcp")
	exportCmd.Flags().BoolVar(&withExamples, "with-examples", false, "Include CLI examples and response schema samples (openai format only)")
	cmd.AddCommand(exportCmd)

	return cmd
}

func toolExamples(schemas []protocol.ToolSchema) []map[string]any {
	out := make([]map[string]any, 0, len(schemas))
	for _, schema := range schemas {
		required := parseRequiredParams(schema.Function.Parameters)
		out = append(out, map[string]any{
			"tool":                schema.Function.Name,
			"get_command":         buildExampleCommand("get", schema.Function.Name, required),
			"raw_command":         buildExampleCommand("raw", schema.Function.Name, required),
			"required_parameters": required,
		})
	}
	return out
}
