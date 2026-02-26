package generator

import (
	"fmt"
	"os"
	"path/filepath"
)

// Scaffold creates the connector directory structure with boilerplate.
func Scaffold(name string, connectorDir string) error {
	dir := filepath.Join(connectorDir, name)
	srcDir := filepath.Join(dir, "src")

	// Check if directory already exists
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("directory %s already exists", dir)
	}

	// Create directories
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	// Write package.json
	packageJSON := fmt.Sprintf(`{
  "name": "harbor-connector-%s",
  "version": "0.1.0",
  "description": "Harbor connector for %s",
  "main": "dist/index.js",
  "bin": {
    "%s": "dist/index.js"
  },
  "scripts": {
    "build": "tsc",
    "bundle": "esbuild src/index.ts --bundle --platform=node --target=node18 --format=cjs --outfile=dist/%s.js --banner:js=\"#!/usr/bin/env node\""
  },
  "license": "Apache-2.0",
  "dependencies": {
    "@oseaitic/harbor-sdk": "^0.1.0"
  },
  "devDependencies": {
    "typescript": "^5.4.0",
    "@types/node": "^20.0.0",
    "esbuild": "^0.20.0"
  }
}
`, name, name, name, name)

	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0o644); err != nil {
		return fmt.Errorf("writing package.json: %w", err)
	}

	// Write tsconfig.json
	tsconfig := `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "Node16",
    "moduleResolution": "Node16",
    "outDir": "dist",
    "rootDir": "src",
    "strict": true,
    "declaration": true,
    "esModuleInterop": true,
    "skipLibCheck": true
  },
  "include": ["src"]
}
`

	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfig), 0o644); err != nil {
		return fmt.Errorf("writing tsconfig.json: %w", err)
	}

	// Write src/index.ts boilerplate
	indexTS := fmt.Sprintf(`import {
  parseArgs,
  output,
  buildMeta,
  errorResponse,
  handleDescribe,
  type HarborToolSchema,
} from "@oseaitic/harbor-sdk";

const CONNECTOR_VERSION = "0.1.0";
const SOURCE = "%s";
const BASE_URL = "https://api.example.com"; // TODO: Set your API base URL

// ── Tool schemas for LLM integration ────────────────────────────

const toolSchemas: HarborToolSchema[] = [
  {
    type: "function",
    function: {
      name: "%s.resource", // TODO: Rename to your resource
      description: "TODO: Describe what this resource does",
      parameters: {
        type: "object",
        properties: {
          // TODO: Define your parameters with descriptions
          // id: { type: "string", description: "The unique item identifier" },
        },
        // IMPORTANT: List required params here — enables CLI positional shorthand
        // e.g. required: ["id"] lets users run: harbor get myapi.items <id>
        required: [],
      },
      summary_fields: [],      // TODO: Fields for compact view
      summary_template: "",    // TODO: Template for summary text
    },
  },
];

// ── Resource handlers ───────────────────────────────────────────

async function fetchResource(
  params: Record<string, string>
): Promise<{ data: unknown[]; raw: unknown }> {
  // TODO: Implement your API call
  // const url = `+"`${BASE_URL}/endpoint`"+`;
  // const resp = await fetch(url);
  // if (!resp.ok) throw new Error(`+"`API error: ${resp.status}`"+`);
  // const raw = await resp.json();

  // TODO: Normalize into array of objects
  // const data = [raw];

  throw new Error("Not implemented — fill in your API logic");
}

// ── Main ────────────────────────────────────────────────────────

async function main() {
  // Handle --describe for tool schema export
  if (handleDescribe(toolSchemas)) return;

  const { resource, params } = parseArgs();

  const handlers: Record<
    string,
    (p: Record<string, string>) => Promise<{ data: unknown[]; raw: unknown }>
  > = {
    resource: fetchResource, // TODO: Map your resource names to handlers
  };

  const handler = handlers[resource];
  if (!handler) {
    output(
      errorResponse(
        SOURCE,
        "resource_not_found",
        `+"`Unknown resource: ${resource}. Available: ${Object.keys(handlers).join(\", \")}`"+`
      )
    );
    process.exit(1);
  }

  try {
    const { data, raw } = await handler(params);
    output({
      data,
      meta: buildMeta({
        source: SOURCE,
        connector_version: CONNECTOR_VERSION,
        schema: `+"`${SOURCE}.${resource}.v1`"+`,
      }),
      raw,
      errors: [],
    });
  } catch (err) {
    output(
      errorResponse(
        SOURCE,
        "execution_error",
        err instanceof Error ? err.message : String(err)
      )
    );
    process.exit(1);
  }
}

main();
`, name, name)

	if err := os.WriteFile(filepath.Join(srcDir, "index.ts"), []byte(indexTS), 0o644); err != nil {
		return fmt.Errorf("writing src/index.ts: %w", err)
	}

	return nil
}
