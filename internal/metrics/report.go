package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/oseaitic/harbor/internal/proxy"
)

// Query filters metrics before aggregation.
type Query struct {
	Since time.Duration
	Tool  string
	Now   time.Time
}

// ToolSummary aggregates metrics for one tool.
type ToolSummary struct {
	ToolName               string  `json:"tool_name"`
	Calls                  int     `json:"calls"`
	MemoryHits             int     `json:"memory_hits"`
	SchemaAppliedCalls     int     `json:"schema_applied_calls"`
	DriftDetectedCalls     int     `json:"drift_detected_calls"`
	AvgCompressionRatio    float64 `json:"avg_compression_ratio"`
	ApproxTokensSavedTotal int     `json:"approx_tokens_saved_total"`
}

// Summary aggregates overall proxy compression metrics.
type Summary struct {
	Path                   string        `json:"path"`
	Since                  time.Duration `json:"since"`
	TotalCalls             int           `json:"total_calls"`
	WindowStart            time.Time     `json:"window_start,omitempty"`
	WindowEnd              time.Time     `json:"window_end,omitempty"`
	MemoryHitRate          float64       `json:"memory_hit_rate"`
	SchemaAppliedRate      float64       `json:"schema_applied_rate"`
	DriftRate              float64       `json:"drift_rate"`
	AvgCompressionRatio    float64       `json:"avg_compression_ratio"`
	ApproxTokensSavedTotal int           `json:"approx_tokens_saved_total"`
	Tools                  []ToolSummary `json:"tools"`
}

// Load reads JSONL proxy compression metrics from disk.
func Load(path string) ([]proxy.CompressionMetric, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening metrics %q: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var out []proxy.CompressionMetric
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var m proxy.CompressionMetric
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		out = append(out, m)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading metrics %q: %w", path, err)
	}

	return out, nil
}

// BuildSummary aggregates metrics with optional filters.
func BuildSummary(path string, events []proxy.CompressionMetric, q Query) Summary {
	now := q.Now
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.Add(-q.Since)

	var (
		totalCalls              int
		memoryHits              int
		schemaAppliedCalls      int
		driftDetectedCalls      int
		compressionRatioSum     float64
		compressionRatioSamples int
		approxTokensSavedTotal  int
		windowStart, windowEnd  time.Time
		toolMap                 = map[string]*ToolSummary{}
	)

	for _, e := range events {
		if q.Since > 0 && !e.Timestamp.IsZero() && e.Timestamp.Before(cutoff) {
			continue
		}
		if q.Tool != "" && e.ToolName != q.Tool {
			continue
		}

		totalCalls++
		if windowStart.IsZero() || (!e.Timestamp.IsZero() && e.Timestamp.Before(windowStart)) {
			windowStart = e.Timestamp
		}
		if windowEnd.IsZero() || e.Timestamp.After(windowEnd) {
			windowEnd = e.Timestamp
		}

		ts := toolMap[e.ToolName]
		if ts == nil {
			ts = &ToolSummary{ToolName: e.ToolName}
			toolMap[e.ToolName] = ts
		}
		ts.Calls++

		if e.FromMemory {
			memoryHits++
			ts.MemoryHits++
		}
		if e.SchemaApplied {
			schemaAppliedCalls++
			ts.SchemaAppliedCalls++
		}
		if e.DriftDetected {
			driftDetectedCalls++
			ts.DriftDetectedCalls++
		}

		if e.SchemaApplied && e.RawBytes > 0 {
			ratio := e.CompressionRatio
			if ratio <= 0 {
				ratio = float64(e.CompactBytes) / float64(e.RawBytes)
			}
			compressionRatioSum += ratio
			compressionRatioSamples++
			ts.AvgCompressionRatio += ratio

			saved := (e.RawBytes - e.CompactBytes) / 4
			if saved > 0 {
				approxTokensSavedTotal += saved
				ts.ApproxTokensSavedTotal += saved
			}
		}
	}

	tools := make([]ToolSummary, 0, len(toolMap))
	for _, t := range toolMap {
		if t.SchemaAppliedCalls > 0 {
			t.AvgCompressionRatio = t.AvgCompressionRatio / float64(t.SchemaAppliedCalls)
		}
		tools = append(tools, *t)
	}
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Calls == tools[j].Calls {
			return tools[i].ToolName < tools[j].ToolName
		}
		return tools[i].Calls > tools[j].Calls
	})

	s := Summary{
		Path:                   path,
		Since:                  q.Since,
		TotalCalls:             totalCalls,
		WindowStart:            windowStart,
		WindowEnd:              windowEnd,
		ApproxTokensSavedTotal: approxTokensSavedTotal,
		Tools:                  tools,
	}

	if totalCalls > 0 {
		s.MemoryHitRate = float64(memoryHits) / float64(totalCalls)
		s.SchemaAppliedRate = float64(schemaAppliedCalls) / float64(totalCalls)
		s.DriftRate = float64(driftDetectedCalls) / float64(totalCalls)
	}
	if compressionRatioSamples > 0 {
		s.AvgCompressionRatio = compressionRatioSum / float64(compressionRatioSamples)
	}

	return s
}
