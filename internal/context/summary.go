package context

import (
	"fmt"
	"strings"
)

// summaryTemplates maps schema strings to template functions.
var summaryTemplates = map[string]func(items []map[string]interface{}) string{
	"crypto.trending.v1":  summarizeTrending,
	"crypto.prices.v1":    summarizePrices,
	"crypto.coin.v1":      summarizeCoin,
	"finance.quote.v1":    summarizeStockQuotes,
	"finance.summary.v1":  summarizeStockSummary,
	"finance.search.v1":   summarizeSearch,
	"finance.trending.v1": summarizeStockTrending,
}

// GenerateSummary produces a natural language summary from data items.
// Uses schema-specific templates or falls back to a generic summary.
func GenerateSummary(items []map[string]interface{}, summaryFields []string, schema string) string {
	if fn, ok := summaryTemplates[schema]; ok {
		return fn(items)
	}
	return genericSummary(items, summaryFields, schema)
}

func summarizeTrending(items []map[string]interface{}) string {
	if len(items) == 0 {
		return "No trending coins."
	}

	var top []string
	limit := 5
	if len(items) < limit {
		limit = len(items)
	}

	for i := 0; i < limit; i++ {
		item := items[i]
		name := strVal(item, "name")
		symbol := strVal(item, "symbol")
		rank := strVal(item, "market_cap_rank")

		entry := fmt.Sprintf("%s (%s", name, symbol)
		if rank != "" {
			entry += fmt.Sprintf(", #%s", rank)
		}
		entry += ")"
		top = append(top, entry)
	}

	return fmt.Sprintf("%d trending coins. Top: %s.", len(items), strings.Join(top, ", "))
}

func summarizePrices(items []map[string]interface{}) string {
	if len(items) == 0 {
		return "No price data."
	}

	var parts []string
	for _, item := range items {
		id := strVal(item, "id")
		usd := numVal(item, "usd")
		change := numVal(item, "usd_24h_change")

		if usd != "" {
			entry := fmt.Sprintf("%s: $%s", id, usd)
			if change != "" {
				entry += fmt.Sprintf(" (%s%%)", change)
			}
			parts = append(parts, entry)
		}
	}

	return strings.Join(parts, ", ")
}

func summarizeCoin(items []map[string]interface{}) string {
	if len(items) == 0 {
		return "No coin data."
	}

	item := items[0]
	name := strVal(item, "name")
	symbol := strVal(item, "symbol")
	rank := strVal(item, "market_cap_rank")

	result := fmt.Sprintf("%s (%s)", name, symbol)
	if rank != "" {
		result += fmt.Sprintf(" — rank #%s", rank)
	}
	return result
}

func summarizeStockQuotes(items []map[string]interface{}) string {
	if len(items) == 0 {
		return "No quote data."
	}

	var parts []string
	for _, item := range items {
		symbol := strVal(item, "symbol")
		name := strVal(item, "shortName")
		price := numVal(item, "regularMarketPrice")
		change := numVal(item, "regularMarketChangePercent")

		entry := symbol
		if name != "" {
			entry = fmt.Sprintf("%s (%s)", symbol, name)
		}
		if price != "" {
			entry += fmt.Sprintf(": $%s", price)
		}
		if change != "" {
			entry += fmt.Sprintf(" (%s%%)", change)
		}
		parts = append(parts, entry)
	}

	return fmt.Sprintf("%d quotes. %s.", len(items), strings.Join(parts, "; "))
}

func summarizeStockSummary(items []map[string]interface{}) string {
	if len(items) == 0 {
		return "No summary data."
	}

	item := items[0]
	symbol := strVal(item, "symbol")
	sector := strVal(item, "sector")
	industry := strVal(item, "industry")
	pe := numVal(item, "trailingPE")
	rec := strVal(item, "recommendationKey")

	result := symbol
	if sector != "" {
		result += fmt.Sprintf(" — %s", sector)
		if industry != "" {
			result += fmt.Sprintf("/%s", industry)
		}
	}
	if pe != "" {
		result += fmt.Sprintf(", P/E: %s", pe)
	}
	if rec != "" {
		result += fmt.Sprintf(", recommendation: %s", rec)
	}
	return result
}

func summarizeSearch(items []map[string]interface{}) string {
	if len(items) == 0 {
		return "No search results."
	}

	limit := 5
	if len(items) < limit {
		limit = len(items)
	}

	var parts []string
	for i := 0; i < limit; i++ {
		item := items[i]
		symbol := strVal(item, "symbol")
		name := strVal(item, "shortname")
		qtype := strVal(item, "quoteType")

		entry := symbol
		if name != "" {
			entry += fmt.Sprintf(" (%s)", name)
		}
		if qtype != "" {
			entry += fmt.Sprintf(" [%s]", qtype)
		}
		parts = append(parts, entry)
	}

	return fmt.Sprintf("%d results. Top: %s.", len(items), strings.Join(parts, ", "))
}

func summarizeStockTrending(items []map[string]interface{}) string {
	if len(items) == 0 {
		return "No trending symbols."
	}

	limit := 10
	if len(items) < limit {
		limit = len(items)
	}

	var parts []string
	for i := 0; i < limit; i++ {
		item := items[i]
		symbol := strVal(item, "symbol")
		name := strVal(item, "shortName")
		change := numVal(item, "regularMarketChangePercent")

		entry := symbol
		if name != "" {
			entry = fmt.Sprintf("%s (%s)", symbol, name)
		}
		if change != "" {
			entry += fmt.Sprintf(" %s%%", change)
		}
		parts = append(parts, entry)
	}

	return fmt.Sprintf("%d trending symbols. %s.", len(items), strings.Join(parts, ", "))
}

func genericSummary(items []map[string]interface{}, summaryFields []string, schema string) string {
	source := schema
	if source == "" {
		source = "unknown"
	}

	fieldList := ""
	if len(summaryFields) > 0 {
		fieldList = fmt.Sprintf(", fields: %s", strings.Join(summaryFields, ", "))
	} else if len(items) > 0 {
		fields := extractFields(items[0])
		if len(fields) > 8 {
			fields = fields[:8]
			fields = append(fields, "...")
		}
		fieldList = fmt.Sprintf(", fields: %s", strings.Join(fields, ", "))
	}

	return fmt.Sprintf("%d items from %s%s.", len(items), source, fieldList)
}

// strVal safely extracts a string value from a map.
func strVal(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%.2f", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// numVal safely extracts a numeric value as a string from a map.
func numVal(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%.2f", val)
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}
