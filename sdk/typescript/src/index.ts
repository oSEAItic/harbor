import { randomUUID } from "crypto";
import { execFile } from "child_process";

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

/** Visibility levels for field-level access control (Govern). */
export type HarborVisibility =
  | "public"
  | "internal"
  | "confidential"
  | "restricted";

export interface HarborToolSchema {
  type: "function";
  function: {
    name: string;
    description: string;
    parameters: Record<string, unknown>;
    summary_fields?: string[];
    summary_template?: string;
    /** Maps field names to their visibility level. Fields not listed default to "public". */
    field_visibility?: Record<string, HarborVisibility>;
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

// ── CLI adapter ─────────────────────────────────────────────────

export interface ExecCLIOptions {
  cwd?: string;
  env?: Record<string, string>;
  timeout?: number;
  parseJSON?: boolean;
}

/**
 * Execute an external CLI tool and capture its output.
 * Uses execFile (no shell) to avoid command injection.
 *
 * Example — wrapping Notion CLI:
 * ```ts
 * const raw = await execCLI("notion", ["search", "--query", params.query, "--format", "json"]);
 * ```
 */
export function execCLI(
  command: string,
  args: string[],
  options?: ExecCLIOptions
): Promise<unknown> {
  const timeout = options?.timeout ?? 30_000;
  const parseJSON = options?.parseJSON ?? true;

  return new Promise((resolve, reject) => {
    const proc = execFile(
      command,
      args,
      {
        cwd: options?.cwd,
        env: options?.env ? { ...process.env, ...options.env } : undefined,
        timeout,
        maxBuffer: 10 * 1024 * 1024, // 10 MB
      },
      (error, stdout, stderr) => {
        if (error) {
          const msg = stderr?.trim() || error.message;
          reject(new Error(`${command} failed: ${msg}`));
          return;
        }

        const output = stdout.trim();

        if (parseJSON) {
          try {
            resolve(JSON.parse(output));
          } catch {
            resolve(output);
          }
        } else {
          resolve(output);
        }
      }
    );
  });
}

// ── Describe helper ─────────────────────────────────────────────

export function handleDescribe(schemas: HarborToolSchema[]): boolean {
  if (process.argv.includes("--describe")) {
    process.stdout.write(JSON.stringify(schemas) + "\n");
    return true;
  }
  return false;
}
