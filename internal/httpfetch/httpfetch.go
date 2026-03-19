// Package httpfetch implements Harbor's auth-proxy HTTP fetcher.
// It makes HTTP requests with credentials injected from the Harbor keychain,
// so the caller (agent or user) never sees raw API keys.
package httpfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oseaitic/harbor/internal/auth"
	"github.com/oseaitic/harbor/internal/protocol"
)

const defaultTimeout = 30 * time.Second

// MissingCredentialError is returned when a credential is not found in the keychain.
// The CLI layer uses this to trigger the interactive setup flow (browser-based or prompt).
type MissingCredentialError struct {
	Name  string // credential name (e.g. "tavily")
	Cause error
}

func (e *MissingCredentialError) Error() string {
	return fmt.Sprintf("credential %q not found: %v", e.Name, e.Cause)
}

func (e *MissingCredentialError) Unwrap() error { return e.Cause }

// Options configures an HTTP fetch through Harbor's auth proxy.
type Options struct {
	URL        string            // Full URL to fetch
	Method     string            // HTTP method (default: GET)
	Headers    map[string]string // Additional request headers
	Body       string            // Request body for POST/PUT/PATCH
	AuthName   string            // Credential name in Harbor keychain
	AuthHeader string            // How to inject credential (e.g. "Authorization: Bearer", "x-cg-pro-api-key")
}

// Result holds the fetch output plus derived connector/resource names for pipeline integration.
type Result struct {
	Response  *protocol.Response
	Connector string // derived from URL domain (e.g. "coingecko")
	Resource  string // derived from URL path (e.g. "search-trending")
}

// Fetch executes an authenticated HTTP request and returns a Harbor protocol response.
func Fetch(ctx context.Context, opts Options) (*Result, error) {
	if opts.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	if opts.AuthName == "" {
		return nil, fmt.Errorf("auth credential name is required")
	}
	if opts.Method == "" {
		opts.Method = "GET"
	}
	if opts.AuthHeader == "" {
		opts.AuthHeader = "Authorization: Bearer"
	}

	// Retrieve credential from keychain
	secret, err := auth.Retrieve(opts.AuthName)
	if err != nil {
		// Credential not found — return error with setup hint.
		// The CLI layer (fetch.go) handles the interactive setup flow.
		return nil, &MissingCredentialError{Name: opts.AuthName, Cause: err}
	}

	// Build HTTP request
	var body io.Reader
	if opts.Body != "" {
		body = strings.NewReader(opts.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, strings.ToUpper(opts.Method), opts.URL, body)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	// Inject auth header
	injectAuth(httpReq, opts.AuthHeader, secret)

	// Add user-specified headers
	for k, v := range opts.Headers {
		httpReq.Header.Set(k, v)
	}

	// Execute
	client := &http.Client{Timeout: defaultTimeout}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Ensure response body is valid JSON; wrap as JSON string if not.
	var rawData json.RawMessage
	if json.Valid(respBody) {
		rawData = respBody
	} else {
		rawData, _ = json.Marshal(string(respBody))
	}

	connectorName := DomainToConnector(opts.URL)
	resource := URLToResource(opts.URL)
	now := time.Now().UTC()

	resp := &protocol.Response{
		Data: rawData,
		Raw:  rawData,
		Meta: protocol.Meta{
			Source:           connectorName,
			Schema:           connectorName + "." + resource + ".v1",
			ConnectorVersion: "http-proxy",
			FetchedAt:        now,
			RequestID:        uuid.New().String(),
		},
		Errors: []protocol.ErrorDetail{},
	}

	// Report HTTP errors
	if httpResp.StatusCode >= 400 {
		resp.Errors = append(resp.Errors, protocol.ErrorDetail{
			Code:    protocol.ErrExecution,
			Message: fmt.Sprintf("HTTP %d %s", httpResp.StatusCode, httpResp.Status),
		})
	}

	return &Result{
		Response:  resp,
		Connector: connectorName,
		Resource:  resource,
	}, nil
}

// injectAuth sets the auth header on the HTTP request.
//
// Formats:
//
//	"Authorization: Bearer" → Authorization: Bearer <secret>
//	"x-cg-pro-api-key"     → x-cg-pro-api-key: <secret>
//	"X-API-Key"             → X-API-Key: <secret>
func injectAuth(req *http.Request, authHeader, secret string) {
	if strings.Contains(authHeader, ":") {
		// "Header-Name: ValuePrefix" format
		parts := strings.SplitN(authHeader, ":", 2)
		name := strings.TrimSpace(parts[0])
		prefix := strings.TrimSpace(parts[1])
		if prefix != "" {
			req.Header.Set(name, prefix+" "+secret)
		} else {
			req.Header.Set(name, secret)
		}
	} else {
		// Plain header name — value is just the secret
		req.Header.Set(authHeader, secret)
	}
}

// DomainToConnector extracts a short connector name from a URL.
//
//	"https://api.coingecko.com/v3/search/trending" → "coingecko"
//	"https://api.github.com/repos/..." → "github"
//	"https://finance.yahoo.com/..." → "yahoo"
func DomainToConnector(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "unknown"
	}
	host := u.Hostname()
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] // second-to-last segment
	}
	return host
}

// URLToResource extracts a resource name from a URL path.
//
//	"/v3/search/trending" → "search-trending"
//	"/repos/owner/repo/issues" → "repos-owner-repo-issues"
//	"/" → "root"
func URLToResource(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "unknown"
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return "root"
	}
	segments := strings.Split(path, "/")
	var clean []string
	for _, s := range segments {
		if s == "" {
			continue
		}
		// Strip version prefixes (v1, v2, v3, etc.)
		if len(s) <= 3 && len(s) >= 2 && s[0] == 'v' && s[1] >= '0' && s[1] <= '9' {
			continue
		}
		clean = append(clean, s)
	}
	if len(clean) == 0 {
		return "root"
	}
	return strings.Join(clean, "-")
}

// PassthroughExecutor wraps a pre-built response so it can be fed to pipeline.Execute.
type PassthroughExecutor struct {
	Response *protocol.Response
}

// Execute returns the pre-built response.
func (e *PassthroughExecutor) Execute(_ context.Context, _ protocol.Request) (*protocol.Response, error) {
	return e.Response, nil
}
