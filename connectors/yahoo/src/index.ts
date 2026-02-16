import {
  parseArgs,
  output,
  buildMeta,
  errorResponse,
  handleDescribe,
  type HarborToolSchema,
} from "harbor-sdk";
import YahooFinance from "yahoo-finance2";

const CONNECTOR_VERSION = "0.1.0";
const SOURCE = "yahoo";

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

const yf = new YahooFinance({ suppressNotices: ["yahooSurvey"] });

async function fetchQuote(
  params: Record<string, string>
): Promise<{ data: unknown[]; raw: unknown }> {
  const symbols = (params.symbols || "AAPL")
    .split(",")
    .map((s) => s.trim().toUpperCase());

  const results = await Promise.all(
    symbols.map((sym) => yf.quote(sym))
  );

  const raw = results;
  const data = results.map((q: Record<string, unknown>) => ({
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

  const result = await yf.quoteSummary(symbol, {
    modules: [
      "summaryProfile",
      "summaryDetail",
      "defaultKeyStatistics",
      "financialData",
      "earningsTrend",
    ],
  });

  const raw = result;

  const profile = result.summaryProfile || ({} as Record<string, unknown>);
  const detail = result.summaryDetail || ({} as Record<string, unknown>);
  const stats = result.defaultKeyStatistics || ({} as Record<string, unknown>);
  const financials = result.financialData || ({} as Record<string, unknown>);

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
  const result = await yf.search(query);

  const raw = result;
  const data = (result.quotes || []).map((q: Record<string, unknown>) => ({
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
  const region = params.region || "US";
  const result = await yf.trendingSymbols(region);

  const raw = result;
  const symbols = result.quotes || [];

  // Fetch quotes for trending symbols to get meaningful data
  const top = symbols.slice(0, 20);
  const quotes = await Promise.all(
    top.map(async (item: Record<string, unknown>) => {
      try {
        const q = await yf.quote(item.symbol as string);
        return {
          symbol: q.symbol,
          shortName: q.shortName,
          regularMarketPrice: q.regularMarketPrice,
          regularMarketChangePercent: q.regularMarketChangePercent,
          marketCap: q.marketCap,
        };
      } catch {
        return { symbol: item.symbol };
      }
    })
  );

  return { data: quotes, raw };
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
