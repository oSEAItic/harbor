import {
  parseArgs,
  output,
  buildMeta,
  errorResponse,
  handleDescribe,
  type HarborToolSchema,
} from "@oseaitic/harbor-sdk";

const CONNECTOR_VERSION = "0.1.0";
const SOURCE = "github";
const BASE_URL = "https://api.github.com";

const toolSchemas: HarborToolSchema[] = [
  {
    type: "function",
    function: {
      name: "github.repos",
      description: "List repositories for a GitHub user/org or search repositories",
      parameters: {
        type: "object",
        properties: {
          owner: { type: "string", description: "GitHub user or org login (optional)" },
          query: { type: "string", description: "Search query when owner is not provided" },
          per_page: { type: "string", description: "Results per page (default 20, max 100)" }
        }
      },
      summary_fields: ["id", "full_name", "stargazers_count", "forks_count", "open_issues_count", "language"],
      summary_template: "{full_name} ⭐{stargazers_count} forks:{forks_count} issues:{open_issues_count}"
    }
  },
  {
    type: "function",
    function: {
      name: "github.issues",
      description: "List issues for a repository",
      parameters: {
        type: "object",
        properties: {
          owner: { type: "string", description: "Repository owner login" },
          repo: { type: "string", description: "Repository name" },
          state: { type: "string", description: "Issue state: open, closed, all (default open)" },
          per_page: { type: "string", description: "Results per page (default 20, max 100)" }
        },
        required: ["owner", "repo"]
      },
      summary_fields: ["id", "number", "title", "state", "comments", "updated_at"],
      summary_template: "#{number} {title} ({state}, comments:{comments})"
    }
  },
  {
    type: "function",
    function: {
      name: "github.pull_requests",
      description: "List pull requests for a repository",
      parameters: {
        type: "object",
        properties: {
          owner: { type: "string", description: "Repository owner login" },
          repo: { type: "string", description: "Repository name" },
          state: { type: "string", description: "PR state: open, closed, all (default open)" },
          per_page: { type: "string", description: "Results per page (default 20, max 100)" }
        },
        required: ["owner", "repo"]
      },
      summary_fields: ["id", "number", "title", "state", "created_at", "updated_at"],
      summary_template: "PR #{number} {title} ({state})"
    }
  }
];

function parsePerPage(input?: string): number {
  const n = Number(input || "20");
  if (!Number.isFinite(n) || n <= 0) return 20;
  return Math.min(n, 100);
}

function authHeaders(token: string): HeadersInit {
  const headers: Record<string, string> = {
    Accept: "application/vnd.github+json",
    "User-Agent": "harbor-github-connector/0.1.0",
  };
  if (token) headers.Authorization = `Bearer ${token}`;
  return headers;
}

async function ghFetch(path: string, token: string): Promise<unknown> {
  const resp = await fetch(`${BASE_URL}${path}`, {
    headers: authHeaders(token),
  });
  if (!resp.ok) {
    throw new Error(`GitHub API error: ${resp.status} ${resp.statusText}`);
  }
  return resp.json();
}

async function fetchRepos(
  params: Record<string, string>,
  token: string
): Promise<{ data: unknown[]; raw: unknown }> {
  const perPage = parsePerPage(params.per_page);
  let raw: unknown;

  if (params.owner) {
    raw = await ghFetch(`/users/${encodeURIComponent(params.owner)}/repos?per_page=${perPage}&sort=updated`, token);
  } else {
    const q = params.query || "language:typescript stars:>1000";
    raw = await ghFetch(`/search/repositories?q=${encodeURIComponent(q)}&per_page=${perPage}`, token);
    raw = ((raw as { items?: unknown[] }).items || []) as unknown[];
  }

  const repos = (raw as Array<Record<string, unknown>>).map((r) => ({
    id: r.id,
    full_name: r.full_name,
    html_url: r.html_url,
    description: r.description,
    stargazers_count: r.stargazers_count,
    forks_count: r.forks_count,
    open_issues_count: r.open_issues_count,
    language: r.language,
    updated_at: r.updated_at,
  }));

  return { data: repos, raw };
}

async function fetchIssues(
  params: Record<string, string>,
  token: string
): Promise<{ data: unknown[]; raw: unknown }> {
  const owner = params.owner || "";
  const repo = params.repo || "";
  if (!owner || !repo) throw new Error("owner and repo are required");

  const state = params.state || "open";
  const perPage = parsePerPage(params.per_page);
  const raw = await ghFetch(
    `/repos/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/issues?state=${encodeURIComponent(state)}&per_page=${perPage}`,
    token
  );

  const data = (raw as Array<Record<string, unknown>>)
    .filter((i) => !i.pull_request)
    .map((i) => ({
      id: i.id,
      number: i.number,
      title: i.title,
      state: i.state,
      comments: i.comments,
      user_login: (i.user as Record<string, unknown> | undefined)?.login,
      created_at: i.created_at,
      updated_at: i.updated_at,
      html_url: i.html_url,
    }));

  return { data, raw };
}

async function fetchPullRequests(
  params: Record<string, string>,
  token: string
): Promise<{ data: unknown[]; raw: unknown }> {
  const owner = params.owner || "";
  const repo = params.repo || "";
  if (!owner || !repo) throw new Error("owner and repo are required");

  const state = params.state || "open";
  const perPage = parsePerPage(params.per_page);
  const raw = await ghFetch(
    `/repos/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/pulls?state=${encodeURIComponent(state)}&per_page=${perPage}`,
    token
  );

  const data = (raw as Array<Record<string, unknown>>).map((pr) => ({
    id: pr.id,
    number: pr.number,
    title: pr.title,
    state: pr.state,
    user_login: (pr.user as Record<string, unknown> | undefined)?.login,
    created_at: pr.created_at,
    updated_at: pr.updated_at,
    html_url: pr.html_url,
  }));

  return { data, raw };
}

async function main() {
  if (handleDescribe(toolSchemas)) return;

  const { resource, params, auth } = parseArgs();

  const handlers: Record<
    string,
    (p: Record<string, string>, token: string) => Promise<{ data: unknown[]; raw: unknown }>
  > = {
    repos: fetchRepos,
    issues: fetchIssues,
    pull_requests: fetchPullRequests,
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
        schema: `github.${resource}.v1`,
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

