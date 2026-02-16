import {
  parseArgs,
  output,
  buildMeta,
  errorResponse,
  handleDescribe,
  type HarborToolSchema,
} from "harbor-sdk";

const CONNECTOR_VERSION = "0.1.0";
const SOURCE = "coingecko";
const BASE_URL = "https://api.coingecko.com/api/v3";

// ── Tool schemas for LLM integration ────────────────────────────

const toolSchemas: HarborToolSchema[] = [
  {
    type: "function",
    function: {
      name: "coingecko.prices",
      description:
        "Get current prices for one or more cryptocurrencies in a target currency",
      parameters: {
        type: "object",
        properties: {
          ids: {
            type: "string",
            description:
              "Comma-separated CoinGecko coin IDs (e.g. bitcoin,ethereum)",
          },
          vs_currencies: {
            type: "string",
            description: "Target currency (e.g. usd, eur)",
          },
        },
        required: ["ids", "vs_currencies"],
      },
      summary_fields: ["id", "usd", "usd_24h_change", "usd_market_cap"],
      summary_template: "{id}: ${usd} ({usd_24h_change}%)",
    },
  },
  {
    type: "function",
    function: {
      name: "coingecko.coin",
      description: "Get detailed information about a specific cryptocurrency",
      parameters: {
        type: "object",
        properties: {
          id: {
            type: "string",
            description: "CoinGecko coin ID (e.g. bitcoin)",
          },
        },
        required: ["id"],
      },
      summary_fields: ["id", "name", "symbol", "market_cap_rank"],
      summary_template: "{name} ({symbol}) — rank #{market_cap_rank}",
    },
  },
  {
    type: "function",
    function: {
      name: "coingecko.trending",
      description: "Get trending cryptocurrencies on CoinGecko",
      parameters: {
        type: "object",
        properties: {},
      },
      summary_fields: ["id", "name", "symbol", "market_cap_rank", "score"],
      summary_template: "{name} ({symbol}, #{market_cap_rank})",
    },
  },
];

// ── Resource handlers ───────────────────────────────────────────

async function fetchPrices(
  params: Record<string, string>
): Promise<{ data: unknown[]; raw: unknown }> {
  const ids = params.ids || "bitcoin";
  const vs = params.vs_currencies || "usd";
  const url = `${BASE_URL}/simple/price?ids=${ids}&vs_currencies=${vs}&include_24hr_change=true&include_market_cap=true`;

  const resp = await fetch(url);
  if (!resp.ok) throw new Error(`CoinGecko API error: ${resp.status}`);
  const raw = await resp.json();

  // Normalize into array of price objects
  const data = Object.entries(raw as Record<string, Record<string, number>>).map(
    ([coinId, prices]) => ({
      id: coinId,
      ...prices,
    })
  );

  return { data, raw };
}

async function fetchCoin(
  params: Record<string, string>
): Promise<{ data: unknown[]; raw: unknown }> {
  const id = params.id || "bitcoin";
  const url = `${BASE_URL}/coins/${encodeURIComponent(id)}?localization=false&tickers=false&community_data=false&developer_data=false`;

  const resp = await fetch(url);
  if (!resp.ok) throw new Error(`CoinGecko API error: ${resp.status}`);
  const raw = (await resp.json()) as Record<string, unknown>;

  const data = [
    {
      id: raw.id,
      symbol: raw.symbol,
      name: raw.name,
      market_cap_rank: raw.market_cap_rank,
      market_data: raw.market_data,
      description: (raw.description as Record<string, string>)?.en?.slice(0, 500),
    },
  ];

  return { data, raw };
}

async function fetchTrending(): Promise<{ data: unknown[]; raw: unknown }> {
  const url = `${BASE_URL}/search/trending`;
  const resp = await fetch(url);
  if (!resp.ok) throw new Error(`CoinGecko API error: ${resp.status}`);
  const raw = (await resp.json()) as { coins: Array<{ item: unknown }> };

  const data = raw.coins.map((c) => c.item);
  return { data, raw };
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
    prices: fetchPrices,
    coin: fetchCoin,
    trending: fetchTrending,
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
    const { data, raw } = await handler(params);
    output({
      data,
      meta: buildMeta({
        source: SOURCE,
        connector_version: CONNECTOR_VERSION,
        schema: `crypto.${resource}.v1`,
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
