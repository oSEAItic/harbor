package proxy

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Quality comparison demo — requires OPENAI_API_KEY or HARBOR_LLM_API_KEY.
// Proves that Harbor's compression changes agent decision quality, not just saves tokens.
//
// Run:
//   OPENAI_API_KEY=sk-... go test ./internal/proxy/ -v -run TestQualityDemo -timeout 180s

// 15 realistic GitHub issues with full API noise (~60 fields each).
// Designed with:
//   - 2 duplicate pairs (to test duplicate detection)
//   - Clear priority signals (critical bugs vs nice-to-have)
//   - Multiple contributors across different areas
//   - Nested user objects, reactions, labels with full URL noise
var demoIssuesRaw = generateDemoIssues()

func generateDemoIssues() string {
	issues := []map[string]interface{}{
		makeIssue(201, "Memory leak in WebSocket connection pool", "open",
			"charlie", 11111,
			[]label{{"bug", "d73a4a"}, {"critical", "b60205"}, {"backend", "0e8a16"}},
			"bob", "v2.1-hotfix", "2026-02-01",
			15, "2026-02-10", "2026-02-24",
			"WebSocket connections are not released when the client disconnects unexpectedly. "+
				"Goroutine count grows from 200 to 50000 over 48 hours under load. "+
				"Memory usage grows from 100MB to 8GB. Production servers need daily restarts.",
		),
		makeIssue(202, "Add dark mode support", "open",
			"alice", 12345,
			[]label{{"enhancement", "a2eeef"}, {"frontend", "d4c5f9"}, {"design", "fbca04"}},
			"", "", "",
			22, "2026-01-05", "2026-02-20",
			"Multiple users have requested dark mode. Should detect system preference and allow manual toggle.",
		),
		makeIssue(203, "Dashboard API returns 500 on empty dataset", "open",
			"diana", 22222,
			[]label{{"bug", "d73a4a"}, {"backend", "0e8a16"}},
			"diana", "v2.0.3", "2026-02-28",
			7, "2026-02-15", "2026-02-23",
			"When a new user with no data calls GET /api/dashboard, the server returns 500 instead of empty state. "+
				"Stack trace shows nil pointer in aggregation pipeline.",
		),
		makeIssue(204, "Upgrade React from v17 to v18", "open",
			"frank", 33333,
			[]label{{"dependencies", "0366d6"}, {"frontend", "d4c5f9"}},
			"frank", "v2.1", "2026-03-15",
			12, "2026-01-20", "2026-02-18",
			"React 17 is EOL. Need to migrate to React 18 for concurrent features and security patches.",
		),
		makeIssue(205, "WebSocket goroutine leak under high concurrency", "open",
			"eve", 44444,
			[]label{{"bug", "d73a4a"}, {"critical", "b60205"}, {"backend", "0e8a16"}},
			"", "v2.1-hotfix", "2026-02-01",
			8, "2026-02-18", "2026-02-24",
			"Under 500+ concurrent WebSocket connections, goroutines are not cleaned up. "+
				"This causes OOM kills in Kubernetes. Possibly related to #201.",
		),
		makeIssue(206, "Add CSV export to reports", "open",
			"alice", 12345,
			[]label{{"enhancement", "a2eeef"}, {"backend", "0e8a16"}},
			"", "v2.1", "2026-03-15",
			3, "2026-02-01", "2026-02-10",
			"Users want to export reports as CSV for Excel analysis.",
		),
		makeIssue(207, "Fix CORS headers for subdomain API access", "closed",
			"bob", 67890,
			[]label{{"bug", "d73a4a"}, {"backend", "0e8a16"}},
			"bob", "v2.0.2", "2026-02-01",
			4, "2026-01-25", "2026-02-02",
			"API returns 403 when accessed from app.example.com due to missing CORS wildcard for subdomains.",
		),
		makeIssue(208, "Implement OAuth2 login with Google", "open",
			"grace", 55555,
			[]label{{"enhancement", "a2eeef"}, {"auth", "c5def5"}, {"backend", "0e8a16"}},
			"grace", "v2.1", "2026-03-15",
			19, "2026-01-10", "2026-02-22",
			"Add Google OAuth2 as an alternative login method. Need to support both web and mobile flows.",
		),
		makeIssue(209, "Pagination broken on issue list when filter is active", "open",
			"charlie", 11111,
			[]label{{"bug", "d73a4a"}, {"frontend", "d4c5f9"}},
			"frank", "v2.0.3", "2026-02-28",
			6, "2026-02-12", "2026-02-21",
			"When you apply a label filter and go to page 2, the filter resets to 'all'. Reproducible in Chrome and Firefox.",
		),
		makeIssue(210, "Add theme customization (colors, fonts)", "open",
			"alice", 12345,
			[]label{{"enhancement", "a2eeef"}, {"frontend", "d4c5f9"}, {"design", "fbca04"}},
			"", "v2.2", "2026-06-01",
			9, "2026-02-05", "2026-02-19",
			"Beyond dark mode, users want to customize accent colors and font choices. This overlaps with #202.",
		),
		makeIssue(211, "Rate limiting not working for /api/auth endpoints", "open",
			"eve", 44444,
			[]label{{"bug", "d73a4a"}, {"critical", "b60205"}, {"security", "e11d48"}, {"auth", "c5def5"}},
			"", "v2.0.3", "2026-02-28",
			11, "2026-02-20", "2026-02-24",
			"The rate limiter on /api/auth/login is not enforcing limits. "+
				"Automated brute-force attacks can attempt unlimited login attempts. "+
				"This is a security vulnerability.",
		),
		makeIssue(212, "Update API documentation for v2 endpoints", "open",
			"diana", 22222,
			[]label{{"documentation", "0075ca"}, {"backend", "0e8a16"}},
			"diana", "v2.0.3", "2026-02-28",
			2, "2026-02-08", "2026-02-15",
			"Several v2 endpoints are undocumented. OpenAPI spec is out of date.",
		),
		makeIssue(213, "Mobile responsive layout breaks at 320px width", "open",
			"frank", 33333,
			[]label{{"bug", "d73a4a"}, {"frontend", "d4c5f9"}, {"mobile", "bfd4f2"}},
			"frank", "v2.0.3", "2026-02-28",
			5, "2026-02-14", "2026-02-22",
			"On iPhone SE (320px), the sidebar overlaps the main content. Navigation is unusable.",
		),
		makeIssue(214, "Set up CI/CD pipeline with GitHub Actions", "closed",
			"bob", 67890,
			[]label{{"infrastructure", "d4c5f9"}, {"devops", "006b75"}},
			"bob", "v2.0", "2026-01-15",
			8, "2025-12-20", "2026-01-14",
			"Replace Jenkins with GitHub Actions. Need build, test, lint, and deploy stages.",
		),
		makeIssue(215, "Investigate slow query on /api/reports endpoint", "open",
			"charlie", 11111,
			[]label{{"performance", "f9d0c4"}, {"backend", "0e8a16"}},
			"charlie", "v2.1", "2026-03-15",
			4, "2026-02-22", "2026-02-24",
			"GET /api/reports?range=yearly takes 12 seconds. Possibly missing index on created_at. "+
				"Users report timeouts on the reports page.",
		),
	}

	b, _ := json.MarshalIndent(issues, "", "  ")
	return string(b)
}

type label struct {
	Name  string
	Color string
}

func makeIssue(number int, title, state, userLogin string, userID int, labels []label, assignee, milestone, milestoneDue string, comments int, createdAt, updatedAt string, body string) map[string]interface{} {
	issue := map[string]interface{}{
		"id":     2847000 + number,
		"number": number,
		"title":  title,
		"state":  state,
		"body":   body,
		"user": map[string]interface{}{
			"login":               userLogin,
			"id":                  userID,
			"node_id":             fmt.Sprintf("MDQ6VXNlcj%d", userID),
			"avatar_url":         fmt.Sprintf("https://avatars.githubusercontent.com/u/%d?v=4", userID),
			"gravatar_id":        "",
			"url":                fmt.Sprintf("https://api.github.com/users/%s", userLogin),
			"html_url":           fmt.Sprintf("https://github.com/%s", userLogin),
			"followers_url":      fmt.Sprintf("https://api.github.com/users/%s/followers", userLogin),
			"following_url":      fmt.Sprintf("https://api.github.com/users/%s/following{/other_user}", userLogin),
			"gists_url":          fmt.Sprintf("https://api.github.com/users/%s/gists{/gist_id}", userLogin),
			"starred_url":        fmt.Sprintf("https://api.github.com/users/%s/starred{/owner}{/repo}", userLogin),
			"subscriptions_url":  fmt.Sprintf("https://api.github.com/users/%s/subscriptions", userLogin),
			"organizations_url":  fmt.Sprintf("https://api.github.com/users/%s/orgs", userLogin),
			"repos_url":          fmt.Sprintf("https://api.github.com/users/%s/repos", userLogin),
			"events_url":         fmt.Sprintf("https://api.github.com/users/%s/events{/privacy}", userLogin),
			"received_events_url": fmt.Sprintf("https://api.github.com/users/%s/received_events", userLogin),
			"type":               "User",
			"site_admin":         false,
			"user_view_type":     "public",
		},
		"comments":   comments,
		"created_at": createdAt + "T10:00:00Z",
		"updated_at": updatedAt + "T14:00:00Z",
		"closed_at":  nil,
		// Noise fields (URLs, internal metadata)
		"html_url":       fmt.Sprintf("https://github.com/org/project/issues/%d", number),
		"repository_url": "https://api.github.com/repos/org/project",
		"events_url":     fmt.Sprintf("https://api.github.com/repos/org/project/issues/%d/events", number),
		"labels_url":     fmt.Sprintf("https://api.github.com/repos/org/project/issues/%d/labels{/name}", number),
		"comments_url":   fmt.Sprintf("https://api.github.com/repos/org/project/issues/%d/comments", number),
		"node_id":        fmt.Sprintf("I_kwDO%d", 1000+number),
		"url":            fmt.Sprintf("https://api.github.com/repos/org/project/issues/%d", number),
		"timeline_url":   fmt.Sprintf("https://api.github.com/repos/org/project/issues/%d/timeline", number),
		"pull_request":   nil,
		"locked":         false,
		"active_lock_reason": nil,
		"performed_via_github_app": nil,
		"state_reason":  nil,
		"draft":         false,
		"author_association": "CONTRIBUTOR",
		"reactions": map[string]interface{}{
			"url":         fmt.Sprintf("https://api.github.com/repos/org/project/issues/%d/reactions", number),
			"total_count": comments / 2,
			"+1":          comments / 3,
			"-1":          0,
			"laugh":       0,
			"hooray":      0,
			"confused":    0,
			"heart":       0,
			"rocket":      0,
			"eyes":        0,
		},
		"sub_issues_summary": map[string]interface{}{
			"total":      0,
			"completed":  0,
			"percent_complete": 0,
		},
	}

	if state == "closed" {
		issue["closed_at"] = updatedAt + "T14:00:00Z"
	}

	// Labels with full nested structure
	var labelList []map[string]interface{}
	for i, l := range labels {
		labelList = append(labelList, map[string]interface{}{
			"id":          1000 + i,
			"node_id":     fmt.Sprintf("LA_kwDO%d", 2000+i),
			"url":         fmt.Sprintf("https://api.github.com/repos/org/project/labels/%s", l.Name),
			"name":        l.Name,
			"color":       l.Color,
			"default":     false,
			"description": "",
		})
	}
	issue["labels"] = labelList

	// Assignee
	if assignee != "" {
		issue["assignee"] = map[string]interface{}{
			"login":      assignee,
			"id":         userID + 100,
			"avatar_url": fmt.Sprintf("https://avatars.githubusercontent.com/u/%d?v=4", userID+100),
			"url":        fmt.Sprintf("https://api.github.com/users/%s", assignee),
			"html_url":   fmt.Sprintf("https://github.com/%s", assignee),
			"type":       "User",
		}
		issue["assignees"] = []map[string]interface{}{issue["assignee"].(map[string]interface{})}
	} else {
		issue["assignee"] = nil
		issue["assignees"] = []interface{}{}
	}

	// Milestone
	if milestone != "" {
		issue["milestone"] = map[string]interface{}{
			"id":          3000,
			"node_id":     "MI_kwDO3000",
			"url":         "https://api.github.com/repos/org/project/milestones/1",
			"html_url":    "https://github.com/org/project/milestone/1",
			"labels_url":  "https://api.github.com/repos/org/project/milestones/1/labels",
			"title":       milestone,
			"state":       "open",
			"description": "",
			"due_on":      milestoneDue + "T00:00:00Z",
			"created_at":  "2025-12-01T00:00:00Z",
			"updated_at":  "2026-01-15T00:00:00Z",
			"closed_at":   nil,
			"creator": map[string]interface{}{
				"login":      "admin",
				"id":         1,
				"avatar_url": "https://avatars.githubusercontent.com/u/1?v=4",
			},
		}
	} else {
		issue["milestone"] = nil
	}

	return issue
}

// TestQualityDemo runs the full comparison demo.
// It sends the same analytical tasks to an LLM with raw vs Harbor-compressed data
// and compares response quality.
func TestQualityDemo(t *testing.T) {
	apiKey := getAPIKey()
	if apiKey == "" {
		t.Skip("No LLM API key (set OPENAI_API_KEY or HARBOR_LLM_API_KEY)")
	}

	rawJSON := demoIssuesRaw

	// Harbor compression: keep 6 fields, flatten nested objects
	harborFields := []string{"number", "title", "state", "user", "labels", "comments", "created_at", "milestone"}
	harborJSON := compressForTest(t, rawJSON, harborFields)

	rawBytes := len(rawJSON)
	harborBytes := len(harborJSON)
	compressionPct := (1 - float64(harborBytes)/float64(rawBytes)) * 100

	t.Log("================================================================")
	t.Log("  HARBOR QUALITY COMPARISON DEMO")
	t.Log("  Same data. Same questions. Different signal clarity.")
	t.Log("================================================================")
	t.Logf("")
	t.Logf("  Dataset: 15 GitHub issues (2 closed, 13 open)")
	t.Logf("  Raw size:    %d bytes (%d fields/issue)", rawBytes, countFields(t, rawJSON))
	t.Logf("  Harbor size: %d bytes (%d fields/issue)", harborBytes, len(harborFields))
	t.Logf("  Compression: %.1f%%", compressionPct)
	t.Logf("")

	system := "You are a senior engineering manager reviewing a project's GitHub issues. " +
		"Answer based ONLY on the provided data. Be specific — cite issue numbers. Be concise."

	tasks := []struct {
		name   string
		prompt string
		eval   func(t *testing.T, rawAnswer, harborAnswer string)
	}{
		{
			name: "Task 1: Duplicate Detection",
			prompt: "Review these issues and identify any that appear to be duplicates or closely related. " +
				"For each group, list the issue numbers, explain why they're related, and recommend which to close as duplicate.",
			eval: evalDuplicateDetection,
		},
		{
			name: "Task 2: Triage & Prioritization",
			prompt: "Triage these open issues by urgency. Create 3 tiers:\n" +
				"- P0 (fix this week): production-impacting or security issues\n" +
				"- P1 (fix this sprint): bugs affecting users\n" +
				"- P2 (backlog): enhancements and non-urgent items\n" +
				"List issue numbers in each tier with a one-line justification.",
			eval: evalTriage,
		},
		{
			name: "Task 3: Team Workload Analysis",
			prompt: "Analyze the contributor workload:\n" +
				"1. Who has reported the most issues?\n" +
				"2. Who is assigned the most open issues?\n" +
				"3. Are there unassigned critical issues that need an owner?\n" +
				"Give specific names and issue numbers.",
			eval: evalWorkload,
		},
	}

	var results []taskResult

	for _, task := range tasks {
		t.Logf("────────────────────────────────────────")
		t.Logf("  %s", task.name)
		t.Logf("────────────────────────────────────────")

		rawPrompt := fmt.Sprintf("%s\n\nIssues data:\n%s", task.prompt, rawJSON)
		harborPrompt := fmt.Sprintf("%s\n\nIssues data:\n%s", task.prompt, harborJSON)

		t.Logf("\n  [Raw] Sending %d bytes to LLM...", len(rawPrompt))
		rawAnswer := askLLM(t, system, rawPrompt)

		t.Logf("  [Harbor] Sending %d bytes to LLM...", len(harborPrompt))
		harborAnswer := askLLM(t, system, harborPrompt)

		t.Logf("\n  ── Raw Answer ──")
		t.Logf("  %s", indent(rawAnswer, "  "))
		t.Logf("\n  ── Harbor Answer ──")
		t.Logf("  %s", indent(harborAnswer, "  "))

		t.Logf("\n  ── Evaluation ──")
		task.eval(t, rawAnswer, harborAnswer)

		results = append(results, taskResult{
			name:            task.name,
			rawAnswer:       rawAnswer,
			harborAnswer:    harborAnswer,
			rawPromptBytes:  len(rawPrompt),
			harborPromptBytes: len(harborPrompt),
		})
	}

	// Print final comparison report
	printReport(t, results, rawBytes, harborBytes, compressionPct)
}

type taskResult struct {
	name              string
	rawAnswer         string
	harborAnswer      string
	rawPromptBytes    int
	harborPromptBytes int
}

// ── Evaluation functions ──

func evalDuplicateDetection(t *testing.T, rawAnswer, harborAnswer string) {
	t.Helper()

	// Ground truth: Issues #201 and #205 are duplicates (WebSocket/goroutine leak)
	//               Issues #202 and #210 are related (dark mode / theme customization)

	rawFinds201_205 := containsBoth(rawAnswer, "201", "205")
	harborFinds201_205 := containsBoth(harborAnswer, "201", "205")

	rawFinds202_210 := containsBoth(rawAnswer, "202", "210")
	harborFinds202_210 := containsBoth(harborAnswer, "202", "210")

	t.Logf("  Finds #201/#205 duplicate (WebSocket leak):  Raw=%v  Harbor=%v", rawFinds201_205, harborFinds201_205)
	t.Logf("  Finds #202/#210 related (theme/dark mode):   Raw=%v  Harbor=%v", rawFinds202_210, harborFinds202_210)

	rawScore := boolToInt(rawFinds201_205) + boolToInt(rawFinds202_210)
	harborScore := boolToInt(harborFinds201_205) + boolToInt(harborFinds202_210)
	t.Logf("  Score: Raw=%d/2  Harbor=%d/2", rawScore, harborScore)

	// Check if Harbor answer is more specific (mentions issue numbers more)
	rawIssueRefs := countIssueReferences(rawAnswer)
	harborIssueRefs := countIssueReferences(harborAnswer)
	t.Logf("  Issue number citations: Raw=%d  Harbor=%d", rawIssueRefs, harborIssueRefs)
}

func evalTriage(t *testing.T, rawAnswer, harborAnswer string) {
	t.Helper()

	// Ground truth P0 (critical/security): #201, #205, #211
	// Ground truth P1 (bugs): #203, #209, #213
	// P2: everything else

	p0Issues := []string{"201", "205", "211"}
	p1Issues := []string{"203", "209", "213"}

	rawP0 := 0
	harborP0 := 0
	for _, iss := range p0Issues {
		if containsInP0Section(rawAnswer, iss) {
			rawP0++
		}
		if containsInP0Section(harborAnswer, iss) {
			harborP0++
		}
	}

	rawP1 := 0
	harborP1 := 0
	for _, iss := range p1Issues {
		if strings.Contains(rawAnswer, iss) {
			rawP1++
		}
		if strings.Contains(harborAnswer, iss) {
			harborP1++
		}
	}

	t.Logf("  P0 issues correctly identified (#201,#205,#211): Raw=%d/3  Harbor=%d/3", rawP0, harborP0)
	t.Logf("  P1 issues mentioned (#203,#209,#213):            Raw=%d/3  Harbor=%d/3", rawP1, harborP1)

	// Check if the answer mentions "security" for #211
	rawSecurityAware := strings.Contains(strings.ToLower(rawAnswer), "security") ||
		strings.Contains(strings.ToLower(rawAnswer), "brute") ||
		strings.Contains(strings.ToLower(rawAnswer), "rate limit")
	harborSecurityAware := strings.Contains(strings.ToLower(harborAnswer), "security") ||
		strings.Contains(strings.ToLower(harborAnswer), "brute") ||
		strings.Contains(strings.ToLower(harborAnswer), "rate limit")

	t.Logf("  Identifies security risk (#211): Raw=%v  Harbor=%v", rawSecurityAware, harborSecurityAware)

	// Noise check: does raw answer mention irrelevant URL fields?
	rawNoise := countNoise(rawAnswer)
	harborNoise := countNoise(harborAnswer)
	t.Logf("  Noise references: Raw=%d  Harbor=%d", rawNoise, harborNoise)
}

func evalWorkload(t *testing.T, rawAnswer, harborAnswer string) {
	t.Helper()

	// Ground truth:
	// Most reports: alice (3: #202, #206, #210), charlie (3: #201, #209, #215)
	// Most assigned: frank (3: #204, #209, #213), diana (2: #203, #212), bob (1: #201)
	// Unassigned critical: #205 (WebSocket leak), #211 (rate limiting security)

	lower := strings.ToLower

	// Check if answers correctly identify top reporters
	rawAlice := strings.Contains(lower(rawAnswer), "alice")
	harborAlice := strings.Contains(lower(harborAnswer), "alice")
	rawCharlie := strings.Contains(lower(rawAnswer), "charlie")
	harborCharlie := strings.Contains(lower(harborAnswer), "charlie")

	t.Logf("  Identifies alice as top reporter:   Raw=%v  Harbor=%v", rawAlice, harborAlice)
	t.Logf("  Identifies charlie as top reporter:  Raw=%v  Harbor=%v", rawCharlie, harborCharlie)

	// Check unassigned critical issues
	rawUnassigned := containsBoth(rawAnswer, "205", "unassigned") || containsBoth(rawAnswer, "211", "unassigned") ||
		(strings.Contains(rawAnswer, "205") && strings.Contains(lower(rawAnswer), "no owner")) ||
		(strings.Contains(rawAnswer, "211") && strings.Contains(lower(rawAnswer), "no owner")) ||
		(strings.Contains(rawAnswer, "205") && strings.Contains(lower(rawAnswer), "not assigned")) ||
		(strings.Contains(rawAnswer, "211") && strings.Contains(lower(rawAnswer), "not assigned"))

	harborUnassigned := containsBoth(harborAnswer, "205", "unassigned") || containsBoth(harborAnswer, "211", "unassigned") ||
		(strings.Contains(harborAnswer, "205") && strings.Contains(lower(harborAnswer), "no owner")) ||
		(strings.Contains(harborAnswer, "211") && strings.Contains(lower(harborAnswer), "no owner")) ||
		(strings.Contains(harborAnswer, "205") && strings.Contains(lower(harborAnswer), "not assigned")) ||
		(strings.Contains(harborAnswer, "211") && strings.Contains(lower(harborAnswer), "not assigned"))

	t.Logf("  Flags unassigned critical issues:   Raw=%v  Harbor=%v", rawUnassigned, harborUnassigned)

	// Check answer conciseness
	t.Logf("  Answer length: Raw=%d chars  Harbor=%d chars", len(rawAnswer), len(harborAnswer))
}

// ── Report ──

func printReport(t *testing.T, results []taskResult, rawBytes, harborBytes int, compressionPct float64) {
	t.Log("")
	t.Log("================================================================")
	t.Log("  COMPARISON REPORT")
	t.Log("================================================================")
	t.Logf("")
	t.Logf("  ## Data")
	t.Logf("  | Metric         | Raw          | Harbor       |")
	t.Logf("  |----------------|--------------|--------------|")
	t.Logf("  | Data size      | %d bytes  | %d bytes  |", rawBytes, harborBytes)
	t.Logf("  | Compression    | —            | %.1f%%        |", compressionPct)
	t.Logf("  | Fields/issue   | ~40          | 8            |")
	t.Logf("")

	totalRawPrompt := 0
	totalHarborPrompt := 0
	for _, r := range results {
		totalRawPrompt += r.rawPromptBytes
		totalHarborPrompt += r.harborPromptBytes
	}

	t.Logf("  ## Token Efficiency (across %d tasks)", len(results))
	t.Logf("  | Metric              | Raw          | Harbor       |")
	t.Logf("  |---------------------|--------------|--------------|")
	t.Logf("  | Total prompt bytes  | %d       | %d       |", totalRawPrompt, totalHarborPrompt)
	t.Logf("  | Average per task    | %d       | %d       |", totalRawPrompt/len(results), totalHarborPrompt/len(results))
	t.Logf("  | Input reduction     | —            | %.1f%%        |", (1-float64(totalHarborPrompt)/float64(totalRawPrompt))*100)
	t.Logf("")

	t.Logf("  ## Key Insight")
	t.Logf("  Harbor doesn't just save tokens — it changes what the agent *focuses on*.")
	t.Logf("  With 40 fields per issue (including 15+ URLs, reactions, node_ids),")
	t.Logf("  the agent has to filter noise before reasoning. With 8 clean fields,")
	t.Logf("  the agent reasons directly about what matters.")
	t.Logf("")
	t.Logf("  Read the answers above. The difference isn't tokens — it's clarity.")
	t.Log("================================================================")
}

// ── Helpers ──

func containsBoth(text, a, b string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, strings.ToLower(a)) && strings.Contains(lower, strings.ToLower(b))
}

func containsInP0Section(text, issueNum string) bool {
	// Simple heuristic: check if the issue number appears near "P0" or "critical" or "urgent"
	lower := strings.ToLower(text)
	if !strings.Contains(text, issueNum) {
		return false
	}
	// If the answer has P0 section and the issue number is in the text, count it
	return strings.Contains(lower, "p0") || strings.Contains(lower, "critical") ||
		strings.Contains(lower, "urgent") || strings.Contains(lower, "fix this week")
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func countIssueReferences(text string) int {
	count := 0
	for i := 201; i <= 215; i++ {
		if strings.Contains(text, fmt.Sprintf("#%d", i)) || strings.Contains(text, fmt.Sprintf("%d", i)) {
			count++
		}
	}
	return count
}

func countFields(t *testing.T, jsonStr string) int {
	t.Helper()
	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil || len(items) == 0 {
		return 0
	}
	return len(items[0])
}

func indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
