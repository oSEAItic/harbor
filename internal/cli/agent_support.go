package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oseaitic/harbor/internal/auth"
	"github.com/oseaitic/harbor/internal/connector"
	"github.com/oseaitic/harbor/internal/harborhome"
	"github.com/oseaitic/harbor/internal/protocol"
)

type commandTemplate struct {
	Name        string `json:"name"`
	Template    string `json:"template"`
	Description string `json:"description"`
}

type connectorResourceCapability struct {
	Name                  string          `json:"name"`
	Description           string          `json:"description"`
	Parameters            json.RawMessage `json:"parameters"`
	RequiredParams        []string        `json:"required_params,omitempty"`
	SummaryFields         []string        `json:"summary_fields,omitempty"`
	ExampleGetCommand     string          `json:"example_get_command"`
	ExampleRawCommand     string          `json:"example_raw_command"`
	ResponseSchemaExample map[string]any  `json:"response_schema_example"`
}

type connectorCapability struct {
	Name      string                        `json:"name"`
	Resources []connectorResourceCapability `json:"resources"`
	Errors    []string                      `json:"errors,omitempty"`
}

type connectorDevStep struct {
	Step        int    `json:"step"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

type connectorDevGuide struct {
	Workflow           []connectorDevStep `json:"workflow"`
	SDKImports         []string           `json:"sdk_imports"`
	BoilerplateExample string             `json:"boilerplate_example"`
	ResponseFormat     string             `json:"response_format"`
	Tips               []string           `json:"tips"`
}

type errorHint struct {
	Code     protocol.ErrorCode `json:"code"`
	Meaning  string             `json:"meaning"`
	Recovery string             `json:"recovery"`
}

type doctorConnectorStatus struct {
	Name           string   `json:"name"`
	Path           string   `json:"path"`
	Executable     bool     `json:"executable"`
	DescribeOK     bool     `json:"describe_ok"`
	ResourceCount  int      `json:"resource_count"`
	AuthConfigured bool     `json:"auth_configured"`
	Errors         []string `json:"errors,omitempty"`
}

type doctorReport struct {
	GeneratedAt           time.Time               `json:"generated_at"`
	HarborHome            string                  `json:"harbor_home"`
	ConnectorsDir         string                  `json:"connectors_dir"`
	ConnectorsDirWritable bool                    `json:"connectors_dir_writable"`
	NodeFound             bool                    `json:"node_found"`
	NodePath              string                  `json:"node_path,omitempty"`
	InstalledConnectors   []string                `json:"installed_connectors"`
	Connectors            []doctorConnectorStatus `json:"connectors"`
	Issues                []string                `json:"issues,omitempty"`
}

type capabilitiesReport struct {
	GeneratedAt            time.Time             `json:"generated_at"`
	Version                string                `json:"version"`
	RecommendedIntegration string                `json:"recommended_integration"`
	InstallHints           []string              `json:"install_hints"`
	CommandTemplates       []commandTemplate     `json:"command_templates"`
	Connectors             []connectorCapability `json:"connectors"`
	ConnectorDevelopment   connectorDevGuide     `json:"connector_development"`
	CommonErrorRecovery    []errorHint           `json:"common_error_recovery"`
	ResponseEnvelopeSchema map[string]any        `json:"response_envelope_schema"`
}

type agentBootstrapReport struct {
	Capabilities capabilitiesReport `json:"capabilities"`
	Doctor       doctorReport       `json:"doctor"`
}

func buildCapabilitiesReport(version string) capabilitiesReport {
	installed, _ := connector.ListInstalled()
	if installed == nil {
		installed = []string{}
	}
	sort.Strings(installed)

	return capabilitiesReport{
		GeneratedAt:            time.Now().UTC(),
		Version:                version,
		RecommendedIntegration: "mcp",
		InstallHints: []string{
			"Use MCP first: harbor mcp",
			"Fetch data directly: harbor get <connector.resource> --param key=value",
			"Discover tools for function calling: harbor tools export --format openai",
		},
		CommandTemplates:       defaultCommandTemplates(),
		Connectors:             collectConnectorCapabilities(installed),
		ConnectorDevelopment:   defaultConnectorDevGuide(),
		CommonErrorRecovery:    defaultErrorHints(),
		ResponseEnvelopeSchema: responseEnvelopeSchemaExample(),
	}
}

func buildAgentBootstrapReport(version string) agentBootstrapReport {
	return agentBootstrapReport{
		Capabilities: buildCapabilitiesReport(version),
		Doctor:       buildDoctorReport(),
	}
}

func buildDoctorReport() doctorReport {
	installed, _ := connector.ListInstalled()
	if installed == nil {
		installed = []string{}
	}
	sort.Strings(installed)

	connectorsDir := connector.ConnectorsDir()
	report := doctorReport{
		GeneratedAt:         time.Now().UTC(),
		HarborHome:          harborhome.RootDir(),
		ConnectorsDir:       connectorsDir,
		InstalledConnectors: installed,
		Connectors:          []doctorConnectorStatus{},
		Issues:              []string{},
	}

	if nodePath, err := exec.LookPath("node"); err == nil {
		report.NodeFound = true
		report.NodePath = nodePath
	} else {
		report.Issues = append(report.Issues, "node not found in PATH (required by node-based connectors)")
	}

	if writableErr := ensureWritableDir(connectorsDir); writableErr == nil {
		report.ConnectorsDirWritable = true
	} else {
		report.Issues = append(report.Issues, fmt.Sprintf("connectors dir not writable: %v", writableErr))
	}

	if len(installed) == 0 {
		report.Issues = append(report.Issues, "no connectors installed")
	}

	for _, name := range installed {
		status := doctorConnectorStatus{
			Name: name,
			Path: connector.ConnectorPath(name),
		}

		if st, err := os.Stat(status.Path); err != nil {
			status.Errors = append(status.Errors, err.Error())
		} else {
			status.Executable = st.Mode()&0o111 != 0
			if !status.Executable {
				status.Errors = append(status.Errors, "connector file is not executable")
			}
		}

		schemas, err := connector.ExportConnectorToolSchemas(name)
		if err == nil {
			status.DescribeOK = true
			status.ResourceCount = len(schemas)
		} else {
			status.Errors = append(status.Errors, "describe failed: "+err.Error())
			report.Issues = append(report.Issues, fmt.Sprintf("%s describe failed", name))
		}

		token, authErr := auth.Retrieve(name)
		status.AuthConfigured = authErr == nil && strings.TrimSpace(token) != ""

		report.Connectors = append(report.Connectors, status)
	}

	return report
}

func collectConnectorCapabilities(installed []string) []connectorCapability {
	out := []connectorCapability{}

	for _, name := range installed {
		item := connectorCapability{Name: name}
		schemas, err := connector.ExportConnectorToolSchemas(name)
		if err != nil {
			item.Errors = []string{err.Error()}
			out = append(out, item)
			continue
		}

		sort.Slice(schemas, func(i, j int) bool {
			return schemas[i].Function.Name < schemas[j].Function.Name
		})

		for _, schema := range schemas {
			required := parseRequiredParams(schema.Function.Parameters)
			item.Resources = append(item.Resources, connectorResourceCapability{
				Name:                  schema.Function.Name,
				Description:           schema.Function.Description,
				Parameters:            schema.Function.Parameters,
				RequiredParams:        required,
				SummaryFields:         schema.Function.SummaryFields,
				ExampleGetCommand:     buildExampleCommand("get", schema.Function.Name, required),
				ExampleRawCommand:     buildExampleCommand("raw", schema.Function.Name, required),
				ResponseSchemaExample: responseEnvelopeSchemaExample(),
			})
		}

		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	return out
}

func defaultCommandTemplates() []commandTemplate {
	return []commandTemplate{
		{
			Name:        "list_connectors",
			Template:    "harbor list",
			Description: "List installed connectors and catalog entries",
		},
		{
			Name:        "get",
			Template:    "harbor get <connector.resource> --param key=value",
			Description: "Fetch context-compiled output for agents",
		},
		{
			Name:        "raw",
			Template:    "harbor raw <connector.resource> --param key=value",
			Description: "Fetch raw upstream payload",
		},
		{
			Name:        "tool_schemas",
			Template:    "harbor tools export --format openai",
			Description: "Export function schemas for model tool calling",
		},
		{
			Name:        "mcp_server",
			Template:    "harbor mcp",
			Description: "Run Harbor as an MCP server (recommended)",
		},
		{
			Name:        "mcp_proxy",
			Template:    "harbor proxy <upstream-mcp-command>",
			Description: "Wrap an existing MCP server with Harbor compression/memory",
		},
		{
			Name:        "doctor",
			Template:    "harbor doctor --json",
			Description: "Run environment checks and return structured diagnostics",
		},
	}
}

func defaultConnectorDevGuide() connectorDevGuide {
	return connectorDevGuide{
		Workflow: []connectorDevStep{
			{Step: 1, Command: "harbor scaffold <name>", Description: "Creates connectors/<name>/ with TypeScript boilerplate (src/index.ts, package.json, tsconfig.json)"},
			{Step: 2, Command: "edit connectors/<name>/src/index.ts", Description: "Set BASE_URL, define toolSchemas array, implement async fetch handlers returning {data: T[], raw: unknown}"},
			{Step: 3, Command: "harbor build <name>", Description: "Runs npm install + esbuild bundle + validates tool schemas via --describe"},
			{Step: 4, Command: "harbor build <name> --install", Description: "Build and install the bundled connector to ~/.harbor/connectors/"},
			{Step: 5, Command: "harbor get <name>.<resource>", Description: "Test the connector end-to-end"},
		},
		SDKImports: []string{
			"parseArgs — parses CLI args into {resource, params, auth, raw}",
			"output — writes JSON response envelope to stdout",
			"buildMeta — creates metadata {source, connector_version, schema, fetched_at, request_id}",
			"errorResponse — creates error envelope with code + message",
			"handleDescribe — returns true and prints toolSchemas if --describe flag is set",
			"execCLI — shell out to external CLIs (e.g. psql, curl) with timeout and JSON parsing",
		},
		BoilerplateExample: `import { parseArgs, output, buildMeta, errorResponse, handleDescribe, type HarborToolSchema } from "@oseaitic/harbor-sdk";

const toolSchemas: HarborToolSchema[] = [{
  type: "function",
  function: {
    name: "myapi.items",
    description: "Fetch items from MyAPI",
    parameters: { type: "object", properties: { query: { type: "string", description: "Search query" } }, required: ["query"] },
    summary_fields: ["id", "name", "status"],
    summary_template: "{id}: {name} ({status})",
  },
}];

async function fetchItems(params: Record<string, string>): Promise<{ data: unknown[]; raw: unknown }> {
  const resp = await fetch("https://api.example.com/items?q=" + encodeURIComponent(params.query || ""));
  if (!resp.ok) throw new Error("API error: " + resp.status);
  const raw = await resp.json();
  const data = (raw.items || []).map((item: any) => ({ id: item.id, name: item.name, status: item.status }));
  return { data, raw };
}

async function main() {
  if (handleDescribe(toolSchemas)) return;
  const { resource, params } = parseArgs();
  const handlers: Record<string, (p: Record<string, string>) => Promise<{ data: unknown[]; raw: unknown }>> = { items: fetchItems };
  const handler = handlers[resource];
  if (!handler) { output(errorResponse("myapi", "resource_not_found", "Unknown: " + resource)); process.exit(1); }
  const { data, raw } = await handler(params);
  output({ data, meta: buildMeta({ source: "myapi", connector_version: "0.1.0", schema: "myapi." + resource + ".v1" }), raw, errors: [] });
}
main();`,
		ResponseFormat: `Connector stdout must be a JSON envelope: { "data": [...], "meta": { "source": "string", "connector_version": "string", "schema": "string", "fetched_at": "RFC3339", "request_id": "UUID" }, "errors": [], "raw": { ...original API response... } }`,
		Tips: []string{
			"Each resource handler must return {data: array, raw: original_response}",
			"summary_fields defines which fields appear in compressed output — pick 3-6 most important",
			"Use HARBOR_AUTH env var for API keys, set via: harbor auth <name>",
			"harbor build validates schema format automatically — fix errors before installing",
			"Connector is a single bundled .js file executed as: node <connector> --resource <name> --params <json>",
			"Add contract.tests.json with test cases: {\"cases\": [{\"name\": \"...\", \"resource\": \"...\", \"params\": {...}}]}",
			"Always set 'required' array and 'description' on each parameter — this enables CLI features like positional args (harbor get x.y <value>) and missing-param error hints",
		},
	}
}

func defaultErrorHints() []errorHint {
	return []errorHint{
		{
			Code:     protocol.ErrConnectorNotFound,
			Meaning:  "Connector is not installed",
			Recovery: "Run `harbor list` then `harbor install <connector>`",
		},
		{
			Code:     protocol.ErrResourceNotFound,
			Meaning:  "Connector does not expose requested resource",
			Recovery: "Run `harbor tools export` and choose a valid connector.resource",
		},
		{
			Code:     protocol.ErrInvalidParams,
			Meaning:  "Required parameters are missing or malformed",
			Recovery: "Use `harbor tools export` and pass each required `--param key=value`",
		},
		{
			Code:     protocol.ErrAuthRequired,
			Meaning:  "Connector requires credentials",
			Recovery: "Run `harbor auth <connector>` to store credentials",
		},
		{
			Code:     protocol.ErrExecution,
			Meaning:  "Connector runtime request failed",
			Recovery: "Check network/runtime dependencies, then retry with `harbor raw` for diagnostics",
		},
		{
			Code:     protocol.ErrSchemaValidation,
			Meaning:  "Connector returned an invalid response envelope",
			Recovery: "Update/reinstall connector and verify `--describe` output",
		},
	}
}

func responseEnvelopeSchemaExample() map[string]any {
	return map[string]any{
		"data": "array",
		"meta": map[string]any{
			"source":              "string",
			"connector_version":   "string",
			"schema":              "string",
			"tool_schema_version": "string",
			"fetched_at":          "RFC3339 timestamp",
			"request_id":          "string",
		},
		"errors": []map[string]any{
			{
				"code":    "string enum",
				"message": "string",
				"detail":  "string (optional)",
			},
		},
		"overview": map[string]any{
			"total_items": "number",
			"fields":      []string{"field_name"},
		},
		"summary": "string",
	}
}

func parseRequiredParams(raw json.RawMessage) []string {
	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		return []string{}
	}

	rawRequired, ok := params["required"].([]any)
	if !ok {
		return []string{}
	}

	required := make([]string, 0, len(rawRequired))
	for _, v := range rawRequired {
		s, ok := v.(string)
		if ok && strings.TrimSpace(s) != "" {
			required = append(required, s)
		}
	}
	sort.Strings(required)
	return required
}

func buildExampleCommand(mode, resource string, required []string) string {
	if mode != "raw" {
		mode = "get"
	}

	cmd := []string{"harbor", mode, resource}
	if len(required) == 0 {
		return strings.Join(cmd, " ")
	}

	for _, key := range required {
		cmd = append(cmd, "--param", key+"="+defaultParamValue(key))
	}

	return strings.Join(cmd, " ")
}

func defaultParamValue(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "symbol":
		return "AAPL"
	case "symbols":
		return "AAPL,MSFT"
	case "ids":
		return "bitcoin"
	case "vs_currencies":
		return "usd"
	case "region":
		return "US"
	case "query":
		return "tesla"
	case "owner":
		return "openai"
	case "repo":
		return "harbor"
	case "per_page", "limit":
		return "5"
	default:
		return "<value>"
	}
}

func ensureWritableDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	testFile := filepath.Join(dir, ".harbor-write-test")
	if err := os.WriteFile(testFile, []byte("ok"), 0o644); err != nil {
		return err
	}
	return os.Remove(testFile)
}
