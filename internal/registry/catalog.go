package registry

// CatalogEntry describes a connector available in the embedded catalog.
type CatalogEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Runtime     string   `json:"runtime"` // "node", "python", "binary"
	DownloadURL string   `json:"download_url"`
	SHA256      string   `json:"sha256,omitempty"`
	Schemas     []string `json:"schemas"`
}

// catalog is the embedded set of known connectors.
var catalog = []CatalogEntry{
	{
		ID:          "coingecko",
		Name:        "CoinGecko",
		Version:     "0.2.0",
		Description: "Cryptocurrency prices, trending coins, and coin details from CoinGecko",
		Runtime:     "node",
		DownloadURL: "https://github.com/oSEAItic/harbor/releases/download/v0.2.0/coingecko.js",
		Schemas:     []string{"crypto.prices.v1", "crypto.coin.v1", "crypto.trending.v1"},
	},
	{
		ID:          "yahoo",
		Name:        "Yahoo Finance",
		Version:     "0.2.0",
		Description: "Stock quotes, company summaries, search, and trending symbols from Yahoo Finance",
		Runtime:     "node",
		DownloadURL: "https://github.com/oSEAItic/harbor/releases/download/v0.2.0/yahoo.js",
		Schemas:     []string{"finance.quote.v1", "finance.summary.v1", "finance.search.v1", "finance.trending.v1"},
	},
}

// ListCatalog returns all connectors in the embedded catalog.
func ListCatalog() []CatalogEntry {
	return catalog
}

// LookupCatalog finds a connector by ID, or returns nil if not found.
func LookupCatalog(id string) *CatalogEntry {
	for i := range catalog {
		if catalog[i].ID == id {
			return &catalog[i]
		}
	}
	return nil
}
