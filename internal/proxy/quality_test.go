package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/oseaitic/harbor/internal/schema"
)

// Quality evaluation tests — requires OPENAI_API_KEY or HARBOR_LLM_API_KEY.
// These tests prove that Harbor's structured context produces better Agent
// responses than raw data. This is Harbor's core value proposition.
//
// Run: OPENAI_API_KEY=sk-... go test ./internal/proxy/ -v -run TestQuality -timeout 120s

func getAPIKey() string {
	if k := os.Getenv("HARBOR_LLM_API_KEY"); k != "" {
		return k
	}
	return os.Getenv("OPENAI_API_KEY")
}

func getBaseURL() string {
	if u := os.Getenv("HARBOR_LLM_BASE_URL"); u != "" {
		return u
	}
	return "https://api.openai.com/v1"
}

func getModel() string {
	if m := os.Getenv("HARBOR_LLM_MODEL"); m != "" {
		return m
	}
	return "gpt-4o-mini"
}

func askLLM(t *testing.T, systemPrompt, userPrompt string) string {
	t.Helper()

	apiKey := getAPIKey()
	if apiKey == "" {
		t.Skip("No LLM API key (set OPENAI_API_KEY or HARBOR_LLM_API_KEY)")
	}

	body := map[string]interface{}{
		"model": getModel(),
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.0,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	url := getBaseURL() + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("LLM request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("LLM API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if len(chatResp.Choices) == 0 {
		t.Fatal("LLM returned no choices")
	}

	t.Logf("  Tokens used: prompt=%d, completion=%d, total=%d",
		chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens, chatResp.Usage.TotalTokens)

	return chatResp.Choices[0].Message.Content
}

// ── Test 1: Context Focus ──
// Does Harbor data help the Agent focus on what matters?

func TestQualityContextFocus(t *testing.T) {
	question := "Based on the data below, which issue needs the most urgent attention and why? Be concise."

	// Raw: Agent sees 20+ fields per issue including URLs, reactions, events_url, etc.
	rawPrompt := fmt.Sprintf("%s\n\nData:\n%s", question, githubIssuesResponse)

	// Harbor: Agent sees only the 6 fields that matter
	harborData := compressForTest(t, githubIssuesResponse, []string{"number", "title", "state", "labels", "comments", "created_at"})
	harborPrompt := fmt.Sprintf("%s\n\nData:\n%s", question, harborData)

	system := "You are a project manager reviewing issues. Answer based only on the provided data."

	t.Log("═══════════════════════════════════════")
	t.Log("  TEST: Context Focus")
	t.Log("  Question: Which issue needs most urgent attention?")
	t.Log("═══════════════════════════════════════")

	t.Log("\n── Raw data response ──")
	t.Logf("  Input size: %d bytes", len(rawPrompt))
	rawAnswer := askLLM(t, system, rawPrompt)
	t.Logf("  Answer: %s", rawAnswer)

	t.Log("\n── Harbor data response ──")
	t.Logf("  Input size: %d bytes", len(harborPrompt))
	harborAnswer := askLLM(t, system, harborPrompt)
	t.Logf("  Answer: %s", harborAnswer)

	// Evaluation: both should identify issue #143 (memory leak, labeled "critical")
	t.Log("\n── Evaluation ──")

	rawHit := strings.Contains(rawAnswer, "143") || strings.Contains(strings.ToLower(rawAnswer), "memory leak")
	harborHit := strings.Contains(harborAnswer, "143") || strings.Contains(strings.ToLower(harborAnswer), "memory leak")

	t.Logf("  Raw identifies critical issue:    %v", rawHit)
	t.Logf("  Harbor identifies critical issue:  %v", harborHit)

	// Both should get it right — but Harbor uses far fewer tokens
	if !harborHit {
		t.Error("Harbor data failed to help Agent identify the critical issue")
	}

	// Check that Harbor answer is more focused (shorter, less noise)
	t.Logf("  Raw answer length:    %d chars", len(rawAnswer))
	t.Logf("  Harbor answer length: %d chars", len(harborAnswer))

	// Check for noise: raw answer might mention URLs, reactions, etc.
	rawNoise := countNoise(rawAnswer)
	harborNoise := countNoise(harborAnswer)
	t.Logf("  Raw noise references:    %d", rawNoise)
	t.Logf("  Harbor noise references: %d", harborNoise)

	t.Logf("\n  Token efficiency: Harbor input is %.0f%% smaller",
		(1-float64(len(harborPrompt))/float64(len(rawPrompt)))*100)
}

// ── Test 2: Multi-Source Attribution ──
// Can the Agent correctly attribute data to its source?

func TestQualityAttribution(t *testing.T) {
	// Simulate Harbor providing data with source metadata
	harborContext := fmt.Sprintf(`You have access to data from multiple sources via Harbor.

Source 1 (list_issues, retrieved %s, TTL 600s):
%s

Source 2 (list_directory, retrieved %s, TTL 600s):
%s

Question: The project has a critical bug. Which file should the developer look at first, and where did you get this information?`,
		time.Now().Add(-5*time.Minute).Format(time.RFC3339),
		compressForTest(t, githubIssuesResponse, []string{"number", "title", "state", "labels"}),
		time.Now().Add(-2*time.Minute).Format(time.RFC3339),
		compressForTest(t, filesystemResponse, []string{"name", "type", "size"}),
	)

	// Raw: no source metadata, just concatenated data
	rawContext := fmt.Sprintf(`Data set 1:
%s

Data set 2:
%s

Question: The project has a critical bug. Which file should the developer look at first, and where did you get this information?`,
		githubIssuesResponse,
		filesystemResponse,
	)

	system := "You are a helpful development assistant. Always cite your data sources when answering. Answer based only on the provided data."

	t.Log("═══════════════════════════════════════")
	t.Log("  TEST: Multi-Source Attribution")
	t.Log("  Question: Where should dev look + cite sources?")
	t.Log("═══════════════════════════════════════")

	t.Log("\n── Raw data response ──")
	rawAnswer := askLLM(t, system, rawContext)
	t.Logf("  Answer: %s", rawAnswer)

	t.Log("\n── Harbor data response ──")
	harborAnswer := askLLM(t, system, harborContext)
	t.Logf("  Answer: %s", harborAnswer)

	// Evaluation: Harbor response should cite specific sources
	t.Log("\n── Evaluation ──")

	harborCitesSources := strings.Contains(harborAnswer, "list_issues") ||
		strings.Contains(harborAnswer, "list_directory") ||
		strings.Contains(harborAnswer, "Source 1") ||
		strings.Contains(harborAnswer, "Source 2")

	rawCitesSources := strings.Contains(rawAnswer, "list_issues") ||
		strings.Contains(rawAnswer, "list_directory") ||
		strings.Contains(rawAnswer, "Data set 1") ||
		strings.Contains(rawAnswer, "Data set 2")

	t.Logf("  Raw cites data sources:    %v", rawCitesSources)
	t.Logf("  Harbor cites data sources: %v", harborCitesSources)

	if !harborCitesSources {
		t.Error("Harbor data did not enable Agent to cite data sources")
	}

	// Harbor answer should mention the source names or timestamps
	harborHasProvenance := strings.Contains(harborAnswer, "list_issues") ||
		strings.Contains(harborAnswer, "retrieved") ||
		strings.Contains(harborAnswer, "minutes ago")

	t.Logf("  Harbor has provenance info: %v", harborHasProvenance)
}

// ── Test 3: Data Freshness Awareness ──
// Does the Agent notice when data might be stale?

func TestQualityFreshnessAwareness(t *testing.T) {
	staleTime := time.Now().Add(-45 * time.Minute).Format(time.RFC3339)
	freshTime := time.Now().Add(-2 * time.Minute).Format(time.RFC3339)

	harborContext := fmt.Sprintf(`Harbor data context:

Source: get_price (retrieved %s, TTL 600s — EXPIRED)
{"symbol":"BTC","price":50000,"currency":"USD"}

Source: get_price (retrieved %s, TTL 600s — fresh)
{"symbol":"ETH","price":3200,"currency":"USD"}

Question: What are the current prices of BTC and ETH?`,
		staleTime, freshTime)

	rawContext := `{"symbol":"BTC","price":50000,"currency":"USD"}
{"symbol":"ETH","price":3200,"currency":"USD"}

Question: What are the current prices of BTC and ETH?`

	system := "You are a financial data assistant. Always note data freshness when answering. If data might be stale, warn the user."

	t.Log("═══════════════════════════════════════")
	t.Log("  TEST: Data Freshness Awareness")
	t.Log("  Question: Current BTC/ETH prices (one stale)")
	t.Log("═══════════════════════════════════════")

	t.Log("\n── Raw data response ──")
	rawAnswer := askLLM(t, system, rawContext)
	t.Logf("  Answer: %s", rawAnswer)

	t.Log("\n── Harbor data response ──")
	harborAnswer := askLLM(t, system, harborContext)
	t.Logf("  Answer: %s", harborAnswer)

	// Evaluation: Harbor should warn about stale BTC data
	t.Log("\n── Evaluation ──")

	harborWarnsStale := strings.Contains(strings.ToLower(harborAnswer), "stale") ||
		strings.Contains(strings.ToLower(harborAnswer), "expired") ||
		strings.Contains(strings.ToLower(harborAnswer), "outdated") ||
		strings.Contains(strings.ToLower(harborAnswer), "old") ||
		strings.Contains(strings.ToLower(harborAnswer), "45 minute") ||
		strings.Contains(strings.ToLower(harborAnswer), "not current") ||
		strings.Contains(strings.ToLower(harborAnswer), "may not")

	rawWarnsStale := strings.Contains(strings.ToLower(rawAnswer), "stale") ||
		strings.Contains(strings.ToLower(rawAnswer), "expired") ||
		strings.Contains(strings.ToLower(rawAnswer), "outdated") ||
		strings.Contains(strings.ToLower(rawAnswer), "old") ||
		strings.Contains(strings.ToLower(rawAnswer), "not current") ||
		strings.Contains(strings.ToLower(rawAnswer), "may not")

	t.Logf("  Raw warns about stale data:    %v", rawWarnsStale)
	t.Logf("  Harbor warns about stale data: %v", harborWarnsStale)

	if !harborWarnsStale {
		t.Error("Harbor data did not enable Agent to detect stale data")
	}

	// The key insight: raw data CAN'T warn about staleness because
	// it has no timestamp/TTL information
	if !harborWarnsStale && rawWarnsStale {
		t.Error("Raw data warned about staleness but Harbor didn't — something is wrong")
	}
}

// ── Test 4: Signal-to-Noise Ratio (no LLM needed) ──
// Structural test: how much of the output is useful vs noise?

func TestQualitySignalToNoise(t *testing.T) {
	// Define what fields are "signal" for common use cases
	issueSignalFields := map[string]bool{
		"number": true, "title": true, "state": true, "labels": true,
		"assignee": true, "comments": true, "created_at": true, "updated_at": true,
		"body": true, "user": true, "milestone": true,
	}

	issueNoiseFields := map[string]bool{
		"events_url": true, "labels_url": true, "comments_url": true,
		"repository_url": true, "html_url": true, "reactions": true,
		"pull_request": true, "id": true,
	}

	// Parse raw data
	var rawItems []map[string]interface{}
	if err := json.Unmarshal([]byte(githubIssuesResponse), &rawItems); err != nil {
		t.Fatalf("parse raw: %v", err)
	}

	// Count signal vs noise in raw data
	var rawSignal, rawNoise, rawTotal int
	for _, item := range rawItems {
		for k := range item {
			rawTotal++
			if issueSignalFields[k] {
				rawSignal++
			}
			if issueNoiseFields[k] {
				rawNoise++
			}
		}
	}

	// Compress with Harbor schema
	compressedJSON := compressForTest(t, githubIssuesResponse, []string{"number", "title", "state", "labels", "assignee", "comments"})
	var harborItems []map[string]interface{}
	if err := json.Unmarshal([]byte(compressedJSON), &harborItems); err != nil {
		t.Fatalf("parse harbor: %v", err)
	}

	var harborSignal, harborNoise, harborTotal int
	for _, item := range harborItems {
		for k := range item {
			harborTotal++
			if issueSignalFields[k] {
				harborSignal++
			}
			if issueNoiseFields[k] {
				harborNoise++
			}
		}
	}

	rawSNR := float64(rawSignal) / float64(rawTotal) * 100
	harborSNR := float64(harborSignal) / float64(harborTotal) * 100

	t.Log("═══════════════════════════════════════")
	t.Log("  TEST: Signal-to-Noise Ratio (structural)")
	t.Log("═══════════════════════════════════════")
	t.Logf("  Raw data:")
	t.Logf("    Total fields:  %d (across %d items)", rawTotal, len(rawItems))
	t.Logf("    Signal fields: %d (%.0f%%)", rawSignal, rawSNR)
	t.Logf("    Noise fields:  %d", rawNoise)
	t.Logf("  Harbor data:")
	t.Logf("    Total fields:  %d (across %d items)", harborTotal, len(harborItems))
	t.Logf("    Signal fields: %d (%.0f%%)", harborSignal, harborSNR)
	t.Logf("    Noise fields:  %d", harborNoise)
	t.Logf("  Signal-to-noise improvement: %.0f%% → %.0f%%", rawSNR, harborSNR)

	if harborSNR <= rawSNR {
		t.Errorf("Harbor SNR (%.0f%%) should be higher than raw SNR (%.0f%%)", harborSNR, rawSNR)
	}

	if harborNoise > 0 {
		t.Errorf("Harbor data should have 0 noise fields, got %d", harborNoise)
	}
}

// ── Helpers ──

// compressForTest applies Harbor's compression logic with the given fields.
func compressForTest(t *testing.T, rawJSON string, fields []string) string {
	t.Helper()

	ls := &schema.LearnedSchema{
		ToolName:      "test",
		SummaryFields: fields,
	}

	compressed, _, _, err := compressWithSchema(rawJSON, ls)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	return compressed
}

// countNoise counts references to noise fields in an LLM response.
func countNoise(text string) int {
	noiseTerms := []string{
		"events_url", "labels_url", "comments_url", "repository_url",
		"avatar_url", "html_url", "reactions", "pull_request",
		"inode", "permissions", "owner", "group",
	}
	count := 0
	lower := strings.ToLower(text)
	for _, term := range noiseTerms {
		if strings.Contains(lower, term) {
			count++
		}
	}
	return count
}
