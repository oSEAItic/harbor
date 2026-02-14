import { randomUUID } from "crypto";

// ── Protocol types ──────────────────────────────────────────────

export interface HarborMeta {
  source: string;
  connector_version: string;
  schema: string;
  tool_schema_version?: string;
  fetched_at: string;
  request_id: string;
  pagination?: {
    cursor?: string;
    has_more: boolean;
    total?: number;
  };
}

export interface HarborError {
  code: string;
  message: string;
  detail?: string;
}

export interface HarborResponse<T = unknown> {
  data: T[];
  meta: HarborMeta;
  raw: unknown | null;
  errors: HarborError[];
}

export interface HarborToolSchema {
  type: "function";
  function: {
    name: string;
    description: string;
    parameters: Record<string, unknown>;
  };
}

export interface ConnectorArgs {
  resource: string;
  params: Record<string, string>;
  auth: string;
  raw: boolean;
}

// ── Argument parsing ────────────────────────────────────────────

export function parseArgs(): ConnectorArgs {
  const args = process.argv.slice(2);
  let resource = "";
  let params: Record<string, string> = {};
  let raw = false;
  const auth = process.env.HARBOR_AUTH || "";

  for (let i = 0; i < args.length; i++) {
    switch (args[i]) {
      case "--resource":
        resource = args[++i] || "";
        break;
      case "--params":
        try {
          params = JSON.parse(args[++i] || "{}");
        } catch {
          params = {};
        }
        break;
      case "--raw":
        raw = true;
        break;
    }
  }

  return { resource, params, auth, raw };
}

// ── Output helper ───────────────────────────────────────────────

export function output<T>(response: HarborResponse<T>): void {
  process.stdout.write(JSON.stringify(response) + "\n");
}

// ── Convenience builders ────────────────────────────────────────

export function buildMeta(opts: {
  source: string;
  connector_version: string;
  schema: string;
}): HarborMeta {
  return {
    source: opts.source,
    connector_version: opts.connector_version,
    schema: opts.schema,
    tool_schema_version: "harbor.tools.v1",
    fetched_at: new Date().toISOString(),
    request_id: randomUUID(),
  };
}

export function errorResponse(
  source: string,
  code: string,
  message: string
): HarborResponse {
  return {
    data: [],
    meta: {
      source,
      connector_version: "0.0.0",
      schema: "",
      fetched_at: new Date().toISOString(),
      request_id: randomUUID(),
    },
    raw: null,
    errors: [{ code, message }],
  };
}

// ── Describe helper ─────────────────────────────────────────────

export function handleDescribe(schemas: HarborToolSchema[]): boolean {
  if (process.argv.includes("--describe")) {
    process.stdout.write(JSON.stringify(schemas) + "\n");
    return true;
  }
  return false;
}
