export interface HarborPageInfo {
    cursor?: string;
    has_more: boolean;
    total?: number;
}
/** Lightweight reference to a related memory entry injected in meta.recalls. */
export interface HarborMemoryRef {
    id: string;
    resource: string;
    /** Human-friendly age string, e.g. "2 days ago", "5 hours ago". */
    age: string;
    summary: string;
    /** True if the memory was created within the freshness window. */
    fresh: boolean;
}
/**
 * Pinned connector-level note written by an agent via harbor_remember.
 * Appears as meta.context on every future access to the connector.
 */
export interface HarborContextRef {
    summary: string;
    age: string;
}
export interface HarborMeta {
    source: string;
    connector_version: string;
    schema: string;
    tool_schema_version?: string;
    fetched_at: string;
    request_id: string;
    pagination?: HarborPageInfo;
    /** ID of the memory entry that backed this response (if any). */
    memory_id?: string;
    /** True when the response was served from the local memory cache. */
    from_memory?: boolean;
    /**
     * Pinned connector-level note from harbor_remember.
     * Present on every access once the agent has saved a conclusion.
     */
    context?: HarborContextRef;
    /**
     * Related memory entries for the same connector, injected automatically
     * by the pipeline to give the agent cross-session context.
     */
    recalls?: HarborMemoryRef[];
    /**
     * Hint shown when no context exists yet, prompting the agent to call
     * harbor_remember after analysis.
     */
    memory_hint?: string;
}
export interface HarborContextOverview {
    total_items: number;
    fields: string[];
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
    overview?: HarborContextOverview;
    summary?: string;
}
/** Visibility levels for field-level access control (Govern). */
export type HarborVisibility = "public" | "internal" | "confidential" | "restricted";
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
export declare function parseArgs(): ConnectorArgs;
export declare function output<T>(response: HarborResponse<T>): void;
export declare function buildMeta(opts: {
    source: string;
    connector_version: string;
    schema: string;
}): HarborMeta;
export declare function errorResponse(source: string, code: string, message: string): HarborResponse;
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
export declare function execCLI(command: string, args: string[], options?: ExecCLIOptions): Promise<unknown>;
export interface HarborFetchOptions {
    /** HTTP method (default: GET, or POST if body is provided). */
    method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
    /** Request body (objects are JSON-serialized automatically). */
    body?: unknown;
    /** Credential name in Harbor keychain (set via `harbor auth <name>`). */
    auth: string;
    /** How to inject the credential (default: "Authorization: Bearer"). */
    authHeader?: string;
    /** Additional headers. */
    headers?: Record<string, string>;
    /** Timeout in ms (default: 30000). */
    timeout?: number;
}
export interface HarborFetchResult {
    /** HTTP status code. */
    status: number;
    /** Parsed response body (JSON if possible, otherwise raw text). */
    data: unknown;
    /** True if the response was served from Harbor memory cache. */
    fromMemory?: boolean;
}
/**
 * Make an auth-proxied HTTP request through Harbor.
 *
 * Harbor injects the credential from its keychain — the calling code
 * never sees the raw API key. Responses go through Harbor's pipeline
 * (memory, schema learning, context injection).
 *
 * @example
 * ```ts
 * // Tavily search — key stored via `harbor auth tavily`
 * const result = await harborFetch("https://api.tavily.com/search", {
 *   auth: "tavily",
 *   body: { query: "AI agent memory", max_results: 5 },
 * });
 *
 * // GitHub API — key stored via `harbor auth github-pat`
 * const repos = await harborFetch("https://api.github.com/user/repos", {
 *   auth: "github-pat",
 * });
 * ```
 */
export declare function harborFetch(url: string, options: HarborFetchOptions): Promise<HarborFetchResult>;
export declare function handleDescribe(schemas: HarborToolSchema[]): boolean;
