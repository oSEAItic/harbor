"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.parseArgs = parseArgs;
exports.output = output;
exports.buildMeta = buildMeta;
exports.errorResponse = errorResponse;
exports.execCLI = execCLI;
exports.harborFetch = harborFetch;
exports.handleDescribe = handleDescribe;
const crypto_1 = require("crypto");
const child_process_1 = require("child_process");
// ── Argument parsing ────────────────────────────────────────────
function parseArgs() {
    const args = process.argv.slice(2);
    let resource = "";
    let params = {};
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
                }
                catch {
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
function output(response) {
    process.stdout.write(JSON.stringify(response) + "\n");
}
// ── Convenience builders ────────────────────────────────────────
function buildMeta(opts) {
    return {
        source: opts.source,
        connector_version: opts.connector_version,
        schema: opts.schema,
        tool_schema_version: "harbor.tools.v1",
        fetched_at: new Date().toISOString(),
        request_id: (0, crypto_1.randomUUID)(),
    };
}
function errorResponse(source, code, message) {
    return {
        data: [],
        meta: {
            source,
            connector_version: "0.0.0",
            schema: "",
            fetched_at: new Date().toISOString(),
            request_id: (0, crypto_1.randomUUID)(),
        },
        raw: null,
        errors: [{ code, message }],
    };
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
function execCLI(command, args, options) {
    const timeout = options?.timeout ?? 30_000;
    const parseJSON = options?.parseJSON ?? true;
    return new Promise((resolve, reject) => {
        const proc = (0, child_process_1.execFile)(command, args, {
            cwd: options?.cwd,
            env: options?.env ? { ...process.env, ...options.env } : undefined,
            timeout,
            maxBuffer: 10 * 1024 * 1024, // 10 MB
        }, (error, stdout, stderr) => {
            if (error) {
                const msg = stderr?.trim() || error.message;
                reject(new Error(`${command} failed: ${msg}`));
                return;
            }
            const output = stdout.trim();
            if (parseJSON) {
                try {
                    resolve(JSON.parse(output));
                }
                catch {
                    resolve(output);
                }
            }
            else {
                resolve(output);
            }
        });
    });
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
async function harborFetch(url, options) {
    const args = ["fetch", url, "--auth", options.auth];
    if (options.method) {
        args.push("-X", options.method);
    }
    if (options.authHeader) {
        args.push("--auth-header", options.authHeader);
    }
    if (options.body) {
        const bodyStr = typeof options.body === "string"
            ? options.body
            : JSON.stringify(options.body);
        args.push("-d", bodyStr);
    }
    if (options.headers) {
        for (const [k, v] of Object.entries(options.headers)) {
            args.push("-H", `${k}: ${v}`);
        }
    }
    const result = await execCLI("harbor", args, {
        timeout: options.timeout ?? 30_000,
        parseJSON: true,
    });
    // Harbor fetch returns a standard HarborResponse envelope.
    const resp = result;
    return {
        status: resp.meta?.from_memory ? 200 : 200,
        data: resp.data ?? resp,
        fromMemory: !!resp.meta?.from_memory,
    };
}
// ── Describe helper ─────────────────────────────────────────────
function handleDescribe(schemas) {
    if (process.argv.includes("--describe")) {
        process.stdout.write(JSON.stringify(schemas) + "\n");
        return true;
    }
    return false;
}
