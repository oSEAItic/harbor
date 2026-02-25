import {
  parseArgs,
  output,
  buildMeta,
  errorResponse,
  handleDescribe,
  type HarborToolSchema,
} from "@oseaitic/harbor-sdk";

const CONNECTOR_VERSION = "0.1.0";
const SOURCE = "yahoo";
const BASE_URL = "https://query1.finance.yahoo.com";

// ── Tool schemas for LLM integration ────────────────────────────

const toolSchemas: HarborToolSchema[] = [
  {
    type: "function",
    function: {
      name: "yahoo.quote",
      description:
        "Get current stock quote for one or more symbols (price, change, volume, market cap)",
      parameters: {
        type: "object",
        properties: {
          symbols: {
            type: "string",
            description:
              "Comma-separated stock symbols (e.g. AAPL,MSFT,GOOGL)",
          },
        },
        required: ["symbols"],
      },
      summary_fields: [
        "symbol",
        "shortName",
        "regularMarketPrice",
        "regularMarketChange",
        "regularMarketChangePercent",
        "regularMarketVolume",
        "marketCap",
      ],
      summary_template:
        "{symbol} ({shortName}): ${regularMarketPrice} ({regularMarketChangePercent}%)",
    },
  },
  {
    type: "function",
    function: {
      name: "yahoo.summary",
      description:
        "Get detailed summary for a stock including profile, financials, and key statistics",
      parameters: {
        type: "object",
        properties: {
          symbol: {
            type: "string",
            description: "Stock symbol (e.g. AAPL)",
          },
        },
        required: ["symbol"],
      },
      summary_fields: [
        "symbol",
        "shortName",
        "sector",
        "industry",
        "marketCap",
        "trailingPE",
        "forwardPE",
        "dividendYield",
        "fiftyTwoWeekHigh",
        "fiftyTwoWeekLow",
      ],
      summary_template:
        "{shortName} ({symbol}) — {sector}/{industry}, P/E: {trailingPE}",
    },
  },
  {
    type: "function",
    function: {
      name: "yahoo.search",
      description:
        "Search for stocks, ETFs, and other securities by name or keyword",
      parameters: {
        type: "object",
        properties: {
          query: {
            type: "string",
            description: "Search query (e.g. 'artificial intelligence' or 'Tesla')",
          },
        },
        required: ["query"],
      },
      summary_fields: [
        "symbol",
        "shortname",
        "quoteType",
        "exchange",
        "score",
      ],
      summary_template: "{symbol} — {shortname} ({quoteType}, {exchange})",
    },
  },
  {
    type: "function",
    function: {
      name: "yahoo.trending",
      description: "Get trending stock symbols in a specific region",
      parameters: {
        type: "object",
        properties: {
          region: {
            type: "string",
            description: "Region code (e.g. US, GB, DE). Defaults to US",
          },
        },
      },
      summary_fields: ["symbol"],
      summary_template: "{symbol}",
    },
  },
];

// ── Resource handlers ───────────────────────────────────────────

type JsonObject = Record<string, unknown>;
type JsonArray = JsonObject[];

function normalizeSymbols(input: string): string[] {
  return input
    .split(",")
    .map((s) => s.trim().toUpperCase())
    .filter(Boolean);
}

async function fetchJSON(url: string): Promise<JsonObject> {
  const resp = await fetch(url, {
    headers: {
      "User-Agent": "harbor-yahoo-connector/0.1.0",
      Accept: "application/json",
    },
  });

  if (!resp.ok) {
    throw new Error(`Yahoo API error: ${resp.status}`);
  }

  const data = (await resp.json()) as unknown;
  if (!data || typeof data !== "object") {
    throw new Error("Yahoo API returned invalid JSON");
  }
  return data as JsonObject;
}

async function fetchQuoteRows(symbols: string[]): Promise<JsonArray> {
  if (symbols.length === 0) return [];

  const url = `${BASE_URL}/v7/finance/quote?symbols=${encodeURIComponent(symbols.join(","))}`;
  const payload = await fetchJSON(url);
  const quoteResponse = (payload.quoteResponse || {}) as JsonObject;
  const result = (quoteResponse.result || []) as JsonArray;
  return result;
}

async function fetchQuote(
  params: Record<string, string>
): Promise<{ data: unknown[]; raw: unknown }> {
  const symbols = normalizeSymbols(params.symbols || "AAPL");
  const rows = await fetchQuoteRows(symbols);

  const raw = rows;
  const data = rows.map((q) => ({
    symbol: q.symbol,
    shortName: q.shortName,
    longName: q.longName,
    regularMarketPrice: q.regularMarketPrice,
    regularMarketChange: q.regularMarketChange,
    regularMarketChangePercent: q.regularMarketChangePercent,
    regularMarketVolume: q.regularMarketVolume,
    regularMarketDayHigh: q.regularMarketDayHigh,
    regularMarketDayLow: q.regularMarketDayLow,
    regularMarketOpen: q.regularMarketOpen,
    regularMarketPreviousClose: q.regularMarketPreviousClose,
    marketCap: q.marketCap,
    trailingPE: q.trailingPE,
    forwardPE: q.forwardPE,
    dividendYield: q.dividendYield,
    fiftyTwoWeekHigh: q.fiftyTwoWeekHigh,
    fiftyTwoWeekLow: q.fiftyTwoWeekLow,
    fiftyDayAverage: q.fiftyDayAverage,
    twoHundredDayAverage: q.twoHundredDayAverage,
    averageDailyVolume3Month: q.averageDailyVolume3Month,
    currency: q.currency,
    exchange: q.exchange,
    quoteType: q.quoteType,
  }));

  return { data, raw };
}

async function fetchSummary(
  params: Record<string, string>
): Promise<{ data: unknown[]; raw: unknown }> {
  const symbol = (params.symbol || "AAPL").toUpperCase();
  const modules = [
    "summaryProfile",
    "summaryDetail",
    "defaultKeyStatistics",
    "financialData",
    "earningsTrend",
  ];
  const url =
    `${BASE_URL}/v10/finance/quoteSummary/${encodeURIComponent(symbol)}` +
    `?modules=${encodeURIComponent(modules.join(","))}`;
  const payload = await fetchJSON(url);
  const quoteSummary = (payload.quoteSummary || {}) as JsonObject;
  const results = (quoteSummary.result || []) as JsonArray;
  const raw = results[0] || {};

  const profile = ((raw as JsonObject).summaryProfile || {}) as JsonObject;
  const detail = ((raw as JsonObject).summaryDetail || {}) as JsonObject;
  const stats = ((raw as JsonObject).defaultKeyStatistics || {}) as JsonObject;
  const financials = ((raw as JsonObject).financialData || {}) as JsonObject;

  const data = [
    {
      symbol,
      shortName: symbol,
      sector: (profile as Record<string, unknown>).sector,
      industry: (profile as Record<string, unknown>).industry,
      fullTimeEmployees: (profile as Record<string, unknown>).fullTimeEmployees,
      longBusinessSummary: (profile as Record<string, unknown>).longBusinessSummary,
      marketCap: (detail as Record<string, unknown>).marketCap,
      trailingPE: (detail as Record<string, unknown>).trailingPE,
      forwardPE: (detail as Record<string, unknown>).forwardPE,
      dividendYield: (detail as Record<string, unknown>).dividendYield,
      fiftyTwoWeekHigh: (detail as Record<string, unknown>).fiftyTwoWeekHigh,
      fiftyTwoWeekLow: (detail as Record<string, unknown>).fiftyTwoWeekLow,
      beta: (detail as Record<string, unknown>).beta,
      priceToBook: (stats as Record<string, unknown>).priceToBook,
      enterpriseValue: (stats as Record<string, unknown>).enterpriseValue,
      profitMargins: (stats as Record<string, unknown>).profitMargins,
      revenueGrowth: (financials as Record<string, unknown>).revenueGrowth,
      grossMargins: (financials as Record<string, unknown>).grossMargins,
      operatingMargins: (financials as Record<string, unknown>).operatingMargins,
      returnOnEquity: (financials as Record<string, unknown>).returnOnEquity,
      totalRevenue: (financials as Record<string, unknown>).totalRevenue,
      totalDebt: (financials as Record<string, unknown>).totalDebt,
      totalCash: (financials as Record<string, unknown>).totalCash,
      currentPrice: (financials as Record<string, unknown>).currentPrice,
      targetMeanPrice: (financials as Record<string, unknown>).targetMeanPrice,
      recommendationKey: (financials as Record<string, unknown>).recommendationKey,
    },
  ];

  return { data, raw };
}

async function fetchSearch(
  params: Record<string, string>
): Promise<{ data: unknown[]; raw: unknown }> {
  const query = params.query || "technology";
  const url =
    `${BASE_URL}/v1/finance/search?q=${encodeURIComponent(query)}` +
    "&quotesCount=25&newsCount=0";
  const result = await fetchJSON(url);

  const raw = result;
  const quotes = (result.quotes || []) as JsonArray;
  const data = quotes.map((q) => ({
    symbol: q.symbol,
    shortname: q.shortname,
    longname: q.longname,
    quoteType: q.quoteType,
    exchange: q.exchange,
    sector: q.sector,
    industry: q.industry,
    score: q.score,
  }));

  return { data, raw };
}

async function fetchTrending(
  params: Record<string, string>
): Promise<{ data: unknown[]; raw: unknown }> {
  const region = (params.region || "US").toUpperCase();
  const url = `${BASE_URL}/v1/finance/trending/${encodeURIComponent(region)}`;
  const result = await fetchJSON(url);
  const raw = result;

  const finance = (result.finance || {}) as JsonObject;
  const topGroups = (finance.result || []) as JsonArray;
  const firstGroup = (topGroups[0] || {}) as JsonObject;
  const quotes = (firstGroup.quotes || []) as JsonArray;
  const symbols = quotes
    .map((item) => String(item.symbol || "").toUpperCase())
    .filter(Boolean)
    .slice(0, 20);

  const detailed = await fetchQuoteRows(symbols);
  const data = detailed.map((q) => ({
    symbol: q.symbol,
    shortName: q.shortName,
    regularMarketPrice: q.regularMarketPrice,
    regularMarketChangePercent: q.regularMarketChangePercent,
    marketCap: q.marketCap,
  }));

  return { data, raw };
}

// ── Main ────────────────────────────────────────────────────────

async function main() {
  if (handleDescribe(toolSchemas)) return;

  const { resource, params } = parseArgs();

  const handlers: Record<
    string,
    (p: Record<string, string>) => Promise<{ data: unknown[]; raw: unknown }>
  > = {
    quote: fetchQuote,
    summary: fetchSummary,
    search: fetchSearch,
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
        schema: `finance.${resource}.v1`,
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
