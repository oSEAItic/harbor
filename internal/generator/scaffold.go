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

// ── Tool schemas ─────────────────────────────────────────────────
//
// Harbor design principles for schema fields:
//
//   summary_fields  — IDENTITY only, not importance.
//     List 2-4 fields that uniquely identify and describe the item
//     (e.g. name, status, id). Do NOT pick fields based on what seems
//     "important" — importance is task-dependent and handled by the
//     agent + Harbor memory layer at runtime.
//
//   field_visibility — PRIVACY and SIZE, not importance.
//     "internal"     : large arrays or verbose text (shown with --full)
//     "confidential" : sensitive data (PII, secrets — requires explicit access)
//     Do NOT use this to hide "unimportant" fields. All public fields
//     are returned; the agent decides what's relevant for its task.
//
//   Your job as connector author: Normalize (clean, structure, rename
//   API-specific fields to meaningful names). Harbor handles Compress
//   and Govern at the pipeline level.

const toolSchemas: HarborToolSchema[] = [
  {
    type: "function",
    function: {
      name: "%s.resource", // TODO: Rename to your resource (e.g. "%s.items")
      description: "TODO: Describe what data this fetches and when to use it",
      parameters: {
        type: "object",
        properties: {
          // TODO: Define your parameters
          // id: { type: "string", description: "The unique item identifier" },
        },
        // IMPORTANT: List required params here — enables CLI positional shorthand
        // e.g. required: ["id"] lets users run: harbor get %s.resource <id>
        required: [],
      },
      // Identity fields only — 2-4 fields that name/describe this item
      summary_fields: [],
      // One-line summary shown in compact output and memory index
      summary_template: "",
      // Privacy/size annotations — omit fields that are always fine to show
      field_visibility: {
        // "items": "internal",        // large array — hidden unless --full
        // "raw_content": "internal",  // verbose text — hidden unless --full
        // "api_secret": "confidential", // sensitive — restricted access
      },
    },
  },
];

// ── Auth helper ──────────────────────────────────────────────────
//
// auth is injected by Harbor from the keychain (set via: harbor auth %s)
// It arrives as a plain string — use it however your API expects:
//   Bearer token:  headers["Authorization"] = "Bearer " + auth
//   API key:       headers["x-api-key"] = auth
//   Cookie:        headers["cookie"] = "session=" + auth

function buildHeaders(auth: string): Record<string, string> {
  return {
    "Content-Type": "application/json",
    // TODO: Set auth header appropriate for your API
    "Authorization": `+"`Bearer ${auth}`"+`,
  };
}

// ── Resource handlers ─────────────────────────────────────────────

async function fetchResource(
  params: Record<string, string>,
  auth: string
): Promise<{ data: unknown[]; raw: unknown }> {
  // TODO: Build URL from params
  // const url = `+"`${BASE_URL}/endpoint/${params.id}`"+`;

  // TODO: Fetch from API
  // const resp = await fetch(url, { headers: buildHeaders(auth) });
  // if (!resp.ok) throw new Error(`+"`API error: ${resp.status}`"+`);
  // const raw = await resp.json() as Record<string, unknown>;

  // TODO: Normalize — rename API fields to clean, meaningful names.
  // Strip internal/implementation-specific fields the agent doesn't need.
  // const data = [{
  //   id:         raw.internal_id,
  //   name:       raw.display_name,
  //   status:     raw.state,
  //   created_at: raw.creation_timestamp,
  //   // ...
  // }];

  // return { data, raw };
  throw new Error("Not implemented — fill in your API logic");
}

// ── Main ──────────────────────────────────────────────────────────

async function main() {
  if (handleDescribe(toolSchemas)) return;

  const { resource, params, auth } = parseArgs();

  if (!auth) {
    output(errorResponse(SOURCE, "auth_required",
      "Authentication required. Run: harbor auth %s"));
    process.exit(1);
  }

  const handlers: Record<
    string,
    (p: Record<string, string>, a: string) => Promise<{ data: unknown[]; raw: unknown }>
  > = {
    resource: fetchResource, // TODO: Map resource names to handlers
  };

  const handler = handlers[resource];
  if (!handler) {
    output(errorResponse(SOURCE, "resource_not_found",
      `+"`Unknown resource: ${resource}. Available: ${Object.keys(handlers).join(\", \")}`"+`));
    process.exit(1);
  }

  try {
    const { data, raw } = await handler(params, auth);
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
    output(errorResponse(SOURCE, "execution_error",
      err instanceof Error ? err.message : String(err)));
    process.exit(1);
  }
}

main();
`, name, name, name, name, name, name)

	if err := os.WriteFile(filepath.Join(srcDir, "index.ts"), []byte(indexTS), 0o644); err != nil {
		return fmt.Errorf("writing src/index.ts: %w", err)
	}

	return nil
}
