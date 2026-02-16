# Harbor Connector Specification v1

This document defines the contract for Harbor connectors. It is designed to be read by both humans and AI agents building connectors.

## Overview

A Harbor connector is a standalone Node.js script (bundled as a single `.js` file) that:

1. Accepts `--describe` → outputs tool schemas as JSON array to stdout
2. Accepts `--resource <name> --params <json>` → outputs a Response to stdout
3. Reads auth from `HARBOR_AUTH` environment variable (never CLI args)

## SDK

All connectors use the `harbor-sdk` TypeScript package, which provides:

```typescript
import {
  parseArgs,       // Parse --resource, --params, --raw from argv
  output,          // Write JSON response to stdout
  buildMeta,       // Build metadata with timestamps + request ID
  errorResponse,   // Create structured error response
  handleDescribe,  // Handle --describe flag (emit schemas + exit)
  type HarborToolSchema,
  type HarborResponse,
  type HarborMeta,
  type HarborError,
  type ConnectorArgs,
} from "harbor-sdk";
```

## Response Format

Every connector resource handler must return this envelope:

```typescript
interface HarborResponse<T = unknown> {
  data: T[];              // Normalized data as array of objects
  meta: HarborMeta;       // Provenance metadata
  raw: unknown | null;    // Original upstream API response (optional)
  errors: HarborError[];  // Empty array if no errors
}

interface HarborMeta {
  source: string;            // Connector name (e.g. "coingecko")
  connector_version: string; // Semantic version (e.g. "0.1.0")
  schema: string;            // Schema identifier (e.g. "crypto.prices.v1")
  tool_schema_version?: string;
  fetched_at: string;        // ISO 8601 timestamp
  request_id: string;        // UUID for tracing
}

interface HarborError {
  code: string;     // e.g. "rate_limit_exceeded", "auth_required"
  message: string;
  detail?: string;
}
```

### Key Rules

- `data` MUST be an array, even for single-item responses: `[{...}]`
- `meta.schema` follows the pattern: `<domain>.<resource>.v<N>` (e.g. `crypto.prices.v1`, `finance.quote.v1`)
- `raw` should contain the unmodified upstream JSON for debugging
- `errors` is an empty array `[]` when there are no errors

## Tool Schema Format

The `--describe` flag must output a JSON array of tool schemas:

```typescript
interface HarborToolSchema {
  type: "function";
  function: {
    name: string;           // "connector.resource" format
    description: string;    // What this resource does
    parameters: {           // JSON Schema for input parameters
      type: "object";
      properties: Record<string, {
        type: string;
        description: string;
      }>;
      required?: string[];
    };
    summary_fields?: string[];    // Fields to keep in compact view
    summary_template?: string;    // Template for natural language summary
  };
}
```

### summary_fields

An array of field names from the `data` objects that are most useful for LLM reasoning. These fields are kept in the compact memory layer while other fields are stripped. Choose 3-6 fields that capture the essence of each data item.

### summary_template

A template string using `{field_name}` placeholders for generating one-line summaries per data item. Example: `"{name} ({symbol}): ${price} ({change}%)"`.

## Authentication

- Auth tokens are passed via the `HARBOR_AUTH` environment variable
- Never accept auth via CLI arguments (security risk: visible in process lists)
- Use `parseArgs()` which reads `process.env.HARBOR_AUTH` into `args.auth`
- If your API doesn't require auth, ignore the auth field

## Connector Structure

```
connectors/<name>/
├── src/
│   └── index.ts       ← Main connector source
├── package.json       ← Dependencies (harbor-sdk)
├── tsconfig.json      ← TypeScript configuration
└── dist/
    └── <name>.js      ← Bundled output (esbuild)
```

## Implementation Pattern

```typescript
import {
  parseArgs, output, buildMeta, errorResponse, handleDescribe,
  type HarborToolSchema,
} from "harbor-sdk";

const CONNECTOR_VERSION = "0.1.0";
const SOURCE = "<connector-name>";
const BASE_URL = "<api-base-url>";

// 1. Define tool schemas
const toolSchemas: HarborToolSchema[] = [
  {
    type: "function",
    function: {
      name: "<connector>.<resource>",
      description: "What this resource does",
      parameters: {
        type: "object",
        properties: {
          param1: { type: "string", description: "..." },
        },
        required: ["param1"],
      },
      summary_fields: ["field1", "field2", "field3"],
      summary_template: "{field1}: {field2} ({field3})",
    },
  },
];

// 2. Implement resource handlers
async function fetchResource(
  params: Record<string, string>
): Promise<{ data: unknown[]; raw: unknown }> {
  const url = `${BASE_URL}/endpoint?param=${params.param1}`;
  const resp = await fetch(url);
  if (!resp.ok) throw new Error(`API error: ${resp.status}`);
  const raw = await resp.json();

  // Normalize: transform upstream response into array of objects
  const data = normalizeResponse(raw);
  return { data, raw };
}

// 3. Main entry point
async function main() {
  // Handle --describe flag
  if (handleDescribe(toolSchemas)) return;

  const { resource, params } = parseArgs();

  // Route to handler
  const handlers: Record<string, (p: Record<string, string>) => Promise<{ data: unknown[]; raw: unknown }>> = {
    "resource": fetchResource,
  };

  const handler = handlers[resource];
  if (!handler) {
    output(errorResponse(SOURCE, "resource_not_found",
      `Unknown resource: ${resource}. Available: ${Object.keys(handlers).join(", ")}`));
    process.exit(1);
  }

  try {
    const { data, raw } = await handler(params);
    output({
      data,
      meta: buildMeta({
        source: SOURCE,
        connector_version: CONNECTOR_VERSION,
        schema: `<domain>.${resource}.v1`,
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
```

## Data Normalization Guidelines

1. **Always return arrays**: Even single items should be `[item]`
2. **Flatten nested structures**: Prefer `{ symbol: "AAPL", price: 150 }` over `{ AAPL: { price: 150 } }`
3. **Add identifying fields**: If the upstream uses keys-as-IDs (e.g. `{ "bitcoin": {...} }`), add an `id` field
4. **Consistent naming**: Use snake_case for field names
5. **Trim verbose fields**: For large text fields (descriptions, etc.), truncate to ~500 chars

## Building & Installing

```bash
# Scaffold a new connector
harbor scaffold <name>

# Build (npm install + esbuild bundle)
harbor build <name>

# Build + install
harbor build <name> --install

# Manual install from local file
harbor install <name> --from connectors/<name>/dist/<name>.js
```

## Validation

After building, Harbor automatically validates the connector by:
1. Running `<name> --describe`
2. Checking the output is valid JSON
3. Verifying each schema has required fields (name, description, parameters)

You can also validate manually:
```bash
node connectors/<name>/dist/<name>.js --describe | python3 -m json.tool
```

## Complete Example

See `connectors/coingecko/src/index.ts` for a fully annotated reference implementation with three resources (prices, coin, trending).
