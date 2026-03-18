/**
 * Harbor Memory Plugin for OpenClaw
 *
 * Adds curated, cross-device, topic-first memory to OpenClaw agents.
 * Works alongside memory-core (does NOT replace it).
 *
 * Features:
 * - harbor_remember tool: save insights by topic with optional refs
 * - harbor_recall tool: structured recall by topic/connector/keyword
 * - Session start hook: inject Harbor context into workspace
 * - Pre-compaction hook: auto-capture context before it's lost
 */

import { execSync } from "node:child_process";
import { writeFileSync, mkdirSync, existsSync } from "node:fs";
import { dirname, join } from "node:path";

// ── harborFetch — exported for community tools ──────────────────────

export interface HarborFetchOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  auth: string;
  authHeader?: string;
  headers?: Record<string, string>;
}

/**
 * Auth-proxied HTTP via Harbor. Use this in your OpenClaw tools —
 * Harbor injects the credential from its keychain, the tool code
 * never sees the raw API key.
 *
 * @example
 * ```ts
 * import { harborFetch } from "@harbor/openclaw-plugin";
 *
 * const result = await harborFetch("https://api.tavily.com/search", {
 *   auth: "tavily",
 *   body: { query: "AI agent memory", max_results: 5 },
 * });
 * ```
 */
export function harborFetch(url: string, options: HarborFetchOptions): Record<string, unknown> | null {
  const args: string[] = ["fetch", url, "--auth", options.auth];

  if (options.method) args.push("-X", options.method);
  if (options.authHeader) args.push("--auth-header", options.authHeader);
  if (options.body) {
    const bodyStr = typeof options.body === "string" ? options.body : JSON.stringify(options.body);
    args.push("-d", bodyStr);
  }
  if (options.headers) {
    for (const [k, v] of Object.entries(options.headers)) {
      args.push("-H", `${k}: ${v}`);
    }
  }

  const raw = harborExec(args.join(" "));
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return { data: raw };
  }
}

// ── Helpers ─────────────────────────────────────────────────────────

function harborExec(args: string): string | null {
  try {
    return execSync(`harbor ${args}`, {
      encoding: "utf-8",
      timeout: 10_000,
      env: { ...process.env, NO_COLOR: "1" },
    }).trim();
  } catch {
    return null;
  }
}

function harborAvailable(): boolean {
  return harborExec("version") !== null;
}

// ── Tools ───────────────────────────────────────────────────────────

function createHarborRememberTool() {
  return {
    name: "harbor_remember",
    description: `Save an analysis conclusion to Harbor's cross-session memory, organized by topic.

Use this when you've reached a conclusion worth preserving for future sessions:
- Root causes of bugs or issues
- Architecture decisions and rationale
- API behavior patterns or quirks
- Configuration insights

Notes persist across sessions, devices, and agents via Harbor Cloud.`,
    parameters: {
      type: "object" as const,
      required: ["topic", "note"],
      properties: {
        topic: {
          type: "string",
          description: 'Topic key for this insight (e.g. "ws-reconnect", "billing-logic", "auth-flow")',
        },
        note: {
          type: "string",
          description: "The insight or conclusion to remember (be specific and actionable)",
        },
        connector: {
          type: "string",
          description: "Optional: scope to a specific data connector (e.g. \"kuse-hive\", \"coingecko\")",
        },
        refs: {
          type: "array",
          items: { type: "string" },
          description: "Optional: memory IDs this note references (e.g. [\"mem_abc123\"])",
        },
      },
    },
    async execute(params: {
      topic: string;
      note: string;
      connector?: string;
      refs?: string[];
    }) {
      const args: string[] = ["remember"];
      if (params.connector) args.push("--connector", params.connector);
      if (params.refs?.length) args.push("--refs", params.refs.join(","));
      args.push("--author", "OpenClaw Agent");
      args.push(JSON.stringify(params.topic));
      args.push(JSON.stringify(params.note));

      const result = harborExec(args.join(" "));
      if (!result) return { error: "harbor CLI not available or command failed" };
      return { result };
    },
  };
}

function createHarborRecallTool() {
  return {
    name: "harbor_recall",
    description: `Search Harbor's cross-session memory for prior insights and conclusions.

Use this BEFORE starting analysis to check if you've already investigated this topic:
- Search by keyword to find related prior work
- Check specific memory IDs for details
- List recent memories to see what's been captured

Returns curated, structured context from past sessions.`,
    parameters: {
      type: "object" as const,
      required: [],
      properties: {
        query: {
          type: "string",
          description: "Keyword search across all memories (e.g. \"websocket\", \"billing\")",
        },
        id: {
          type: "string",
          description: "Recall a specific memory by ID (e.g. \"mem_abc123\")",
        },
        list: {
          type: "boolean",
          description: "List recent memories (default if no query or id)",
        },
      },
    },
    async execute(params: { query?: string; id?: string; list?: boolean }) {
      let args = "recall";
      if (params.id) {
        args += ` ${params.id}`;
      } else if (params.query) {
        args += ` --search ${JSON.stringify(params.query)}`;
      } else {
        args += " --list";
      }

      const result = harborExec(args);
      if (!result) return { error: "harbor CLI not available or no memories found" };
      return { result };
    },
  };
}

// ── Context sync ────────────────────────────────────────────────────

function syncHarborContextToWorkspace(workspaceDir: string, contextFile: string) {
  const recalls = harborExec("recall --list");
  if (!recalls || recalls.includes("No memories found")) return;

  const targetPath = join(workspaceDir, contextFile);
  mkdirSync(dirname(targetPath), { recursive: true });

  const content = [
    "# Harbor Context (auto-synced)",
    "",
    "> Cross-session insights from prior investigations.",
    "> Updated on each session start. Do not edit manually.",
    "",
    recalls,
    "",
    `_Last synced: ${new Date().toISOString()}_`,
  ].join("\n");

  writeFileSync(targetPath, content, "utf-8");
}

// ── Plugin ──────────────────────────────────────────────────────────

type PluginConfig = {
  autoSync?: boolean;
  autoCapture?: boolean;
  contextFile?: string;
};

const harborPlugin = {
  id: "harbor",
  name: "Harbor Memory",
  description: "Cross-session curated memory, cross-device sync, and knowledge graph",

  configSchema: {
    type: "object",
    additionalProperties: false,
    properties: {
      autoSync: { type: "boolean", default: true },
      autoCapture: { type: "boolean", default: true },
      contextFile: { type: "string", default: "memory/harbor-context.md" },
    },
  },

  register(api: any) {
    const config: PluginConfig = (api.pluginConfig ?? {}) as PluginConfig;
    const autoSync = config.autoSync !== false;
    const autoCapture = config.autoCapture !== false;
    const contextFile = config.contextFile || "memory/harbor-context.md";

    // Check harbor CLI availability once at registration.
    if (!harborAvailable()) {
      api.logger.warn("harbor CLI not found in PATH — tools will be registered but may fail");
    }

    // Auto-provision cloud account if not configured (best-effort, non-blocking).
    try {
      const status = harborExec("cloud status");
      if (status?.includes("not configured")) {
        api.logger.info("Harbor Cloud not configured — auto-provisioning free account...");
        const result = harborExec("cloud enable");
        if (result) api.logger.info(result);
      }
    } catch {
      // Never block plugin registration.
    }

    // ── Register tools ──────────────────────────────────────────

    api.registerTool(
      () => [createHarborRememberTool(), createHarborRecallTool()],
      { names: ["harbor_remember", "harbor_recall"] },
    );

    // ── Hook: session_start → sync Harbor context to workspace ──

    if (autoSync) {
      api.on("session_start", (event: any, ctx: any) => {
        try {
          const workspaceDir = ctx?.workspaceDir || ctx?.agentDir;
          if (workspaceDir) {
            syncHarborContextToWorkspace(workspaceDir, contextFile);
            api.logger.info(`Harbor context synced to ${contextFile}`);
          }
        } catch (err) {
          api.logger.warn(`Harbor context sync failed: ${err}`);
        }
      });
    }

    // ── Hook: before_compaction → capture context before it's lost ──

    if (autoCapture) {
      api.on("before_compaction", (event: any) => {
        try {
          // Extract recent assistant messages that might contain insights.
          const messages = event?.messages ?? [];
          const assistantMsgs = messages
            .filter((m: any) => m.role === "assistant" && typeof m.content === "string")
            .slice(-3); // Last 3 assistant messages

          if (assistantMsgs.length === 0) return;

          // Concatenate and save as a session summary note.
          const summary = assistantMsgs
            .map((m: any) => m.content)
            .join("\n---\n")
            .slice(0, 2000);

          harborExec(
            `remember --author "OpenClaw Agent" session-summary ${JSON.stringify(summary)}`,
          );
        } catch {
          // Best-effort — never block compaction.
        }
      });
    }

    // ── Register CLI ────────────────────────────────────────────

    api.registerCli(
      ({ program }: any) => {
        const cmd = program.command("harbor").description("Harbor memory commands");

        cmd
          .command("status")
          .description("Check Harbor CLI and memory status")
          .action(() => {
            const version = harborExec("version");
            if (version) {
              console.log(`Harbor CLI: ${version}`);
              const list = harborExec("recall --list");
              console.log(list || "No memories.");
            } else {
              console.log("Harbor CLI not found. Install: go install github.com/oseaitic/harbor/cmd/harbor@latest");
            }
          });

        cmd
          .command("sync")
          .description("Sync Harbor context to workspace now")
          .action(() => {
            const workspaceDir = process.cwd();
            syncHarborContextToWorkspace(workspaceDir, contextFile);
            console.log(`Synced to ${contextFile}`);
          });
      },
      { commands: ["harbor"] },
    );
  },
};

export default harborPlugin;
