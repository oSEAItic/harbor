import {
  parseArgs,
  output,
  buildMeta,
  errorResponse,
  handleDescribe,
  execCLI,
  type HarborToolSchema,
} from "harbor-sdk";

const CONNECTOR_VERSION = "0.1.0";
const SOURCE = "postgresql";

const toolSchemas: HarborToolSchema[] = [
  {
    type: "function",
    function: {
      name: "postgresql.query",
      description: "Run a read-only SQL query against PostgreSQL and return rows as JSON",
      parameters: {
        type: "object",
        properties: {
          sql: {
            type: "string",
            description: "SQL SELECT query to run",
          },
          limit: {
            type: "string",
            description: "Optional row limit applied on top of query result (default 100)",
          },
          connection_url: {
            type: "string",
            description: "Optional PostgreSQL connection URL. If omitted, Harbor uses HARBOR_AUTH",
          },
        },
        required: ["sql"],
      },
      summary_fields: ["id"],
      summary_template: "row id={id}",
    },
  },
  {
    type: "function",
    function: {
      name: "postgresql.tables",
      description: "List tables from information_schema.tables",
      parameters: {
        type: "object",
        properties: {
          schema: {
            type: "string",
            description: "Schema name (default: public)",
          },
          limit: {
            type: "string",
            description: "Optional row limit (default 200)",
          },
          connection_url: {
            type: "string",
            description: "Optional PostgreSQL connection URL. If omitted, Harbor uses HARBOR_AUTH",
          },
        },
      },
      summary_fields: ["table_schema", "table_name", "table_type"],
      summary_template: "{table_schema}.{table_name} ({table_type})",
    },
  },
  {
    type: "function",
    function: {
      name: "postgresql.schemas",
      description: "List available schemas in the database",
      parameters: {
        type: "object",
        properties: {
          connection_url: {
            type: "string",
            description: "Optional PostgreSQL connection URL. If omitted, Harbor uses HARBOR_AUTH",
          },
        },
      },
      summary_fields: ["schema_name"],
      summary_template: "{schema_name}",
    },
  },
];

function parseLimit(input: string | undefined, fallback: number, max: number): number {
  const n = Number(input ?? "");
  if (!Number.isFinite(n) || n <= 0) return fallback;
  return Math.min(Math.floor(n), max);
}

function escapeLiteral(value: string): string {
  return value.replace(/'/g, "''");
}

function cleanSQL(sql: string): string {
  return sql.replace(/;\s*$/g, "").trim();
}

function ensureConnectionURL(params: Record<string, string>, auth: string): string {
  const conn = (params.connection_url || auth || "").trim();
  if (!conn) {
    throw new Error("missing PostgreSQL connection URL. Set --param connection_url=... or run `harbor auth postgresql`");
  }
  return conn;
}

async function runQuery(connectionURL: string, sql: string): Promise<{ data: unknown[]; raw: unknown }> {
  const wrapped = `SELECT COALESCE(json_agg(t), '[]'::json)::text FROM (${cleanSQL(sql)}) AS t`;

  const rawText = await execCLI("psql", [
    connectionURL,
    "-X",
    "-v",
    "ON_ERROR_STOP=1",
    "-At",
    "-c",
    wrapped,
  ], {
    parseJSON: false,
    timeout: 45000,
  });

  const text = String(rawText).trim();
  if (!text) {
    return { data: [], raw: [] };
  }

  const data = JSON.parse(text);
  if (!Array.isArray(data)) {
    throw new Error("query did not return a JSON array");
  }
  return { data, raw: data };
}

async function fetchQuery(params: Record<string, string>, auth: string): Promise<{ data: unknown[]; raw: unknown }> {
  const connectionURL = ensureConnectionURL(params, auth);
  const sql = (params.sql || "").trim();
  if (!sql) throw new Error("sql parameter is required");
  const limit = parseLimit(params.limit, 100, 1000);

  const limited = `SELECT * FROM (${cleanSQL(sql)}) AS q LIMIT ${limit}`;
  return runQuery(connectionURL, limited);
}

async function fetchTables(params: Record<string, string>, auth: string): Promise<{ data: unknown[]; raw: unknown }> {
  const connectionURL = ensureConnectionURL(params, auth);
  const schema = (params.schema || "public").trim();
  const limit = parseLimit(params.limit, 200, 1000);

  const sql = `
    SELECT
      table_schema,
      table_name,
      table_type
    FROM information_schema.tables
    WHERE table_schema = '${escapeLiteral(schema)}'
    ORDER BY table_name
    LIMIT ${limit}
  `;

  return runQuery(connectionURL, sql);
}

async function fetchSchemas(params: Record<string, string>, auth: string): Promise<{ data: unknown[]; raw: unknown }> {
  const connectionURL = ensureConnectionURL(params, auth);
  const sql = `
    SELECT schema_name
    FROM information_schema.schemata
    ORDER BY schema_name
  `;
  return runQuery(connectionURL, sql);
}

async function main() {
  if (handleDescribe(toolSchemas)) return;

  const { resource, params, auth } = parseArgs();

  const handlers: Record<
    string,
    (p: Record<string, string>, a: string) => Promise<{ data: unknown[]; raw: unknown }>
  > = {
    query: fetchQuery,
    tables: fetchTables,
    schemas: fetchSchemas,
  };

  const handler = handlers[resource];
  if (!handler) {
    output(
      errorResponse(
        SOURCE,
        "resource_not_found",
        `Unknown resource: ${resource}. Available: ${Object.keys(handlers).join(", ")}`
      )
    );
    process.exit(1);
  }

  try {
    const { data, raw } = await handler(params, auth);
    output({
      data,
      meta: buildMeta({
        source: SOURCE,
        connector_version: CONNECTOR_VERSION,
        schema: `database.${resource}.v1`,
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

