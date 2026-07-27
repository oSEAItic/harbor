// Package farm is the Harbor-owned client for Farm cloud state and agent usage.
package farm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/harborhome"
)

const maxQueuedEvents = 5000

type Crop struct {
	Name        string `json:"name"`
	Cost        int    `json:"cost"`
	Reward      int    `json:"reward"`
	XP          int    `json:"xp"`
	GrowSeconds int    `json:"grow_seconds"`
}

type Plot struct {
	PlotIndex        int        `json:"plot_index"`
	CropType         *string    `json:"crop_type"`
	PlantedAt        *time.Time `json:"planted_at"`
	ReadyAt          *time.Time `json:"ready_at"`
	IsReady          bool       `json:"is_ready"`
	RemainingSeconds int        `json:"remaining_seconds"`
	ForageCount      int        `json:"forage_count,omitempty"`
	CanForage        bool       `json:"can_forage,omitempty"`
}

type AgentSession struct {
	ExternalSessionID string `json:"external_session_id"`
	Source            string `json:"source"`
	Model             string `json:"model,omitempty"`
	State             string `json:"state"`
	UsageQuality      string `json:"usage_quality"`
}

type SessionReceipt struct {
	Version      int    `json:"version"`
	Source       string `json:"source"`
	ModelFamily  string `json:"model_family,omitempty"`
	Duration     string `json:"duration_bucket"`
	InputTokens  string `json:"input_token_bucket"`
	OutputTokens string `json:"output_token_bucket"`
	ToolCount    int    `json:"tool_count"`
	EventCount   int    `json:"event_count"`
	Outcome      string `json:"outcome"`
	Privacy      string `json:"privacy"`
}

type CropGenome struct {
	Version     int    `json:"version"`
	Fingerprint string `json:"fingerprint"`
	Hue         int    `json:"hue"`
	LeafShape   string `json:"leaf_shape"`
	FruitShape  string `json:"fruit_shape"`
	Marking     string `json:"marking"`
	Trait       string `json:"trait"`
}

type SessionCrop struct {
	ID          string         `json:"id"`
	DisplayName string         `json:"display_name"`
	Species     string         `json:"species"`
	Rarity      string         `json:"rarity"`
	Stage       string         `json:"stage"`
	Progress    int            `json:"progress"`
	Genome      CropGenome     `json:"genome"`
	Receipt     SessionReceipt `json:"receipt"`
	RevealedAt  *time.Time     `json:"revealed_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Neighbor struct {
	FarmCode     string `json:"farm_code"`
	DisplayName  string `json:"display_name"`
	Level        int    `json:"level"`
	ReadyPlots   int    `json:"ready_plots"`
	SessionCrops int    `json:"session_crops"`
}

type FarmSocial struct {
	FarmCode  string     `json:"farm_code"`
	Neighbors []Neighbor `json:"neighbors"`
}

type NeighborFarm struct {
	FarmCode     string        `json:"farm_code"`
	DisplayName  string        `json:"display_name"`
	Level        int           `json:"level"`
	Plots        []Plot        `json:"plots"`
	SessionCrops []SessionCrop `json:"session_crops"`
	ServerTime   time.Time     `json:"server_time"`
}

type ForageResult struct {
	FarmCode           string `json:"farm_code"`
	PlotIndex          int    `json:"plot_index"`
	CropType           string `json:"crop_type"`
	Reward             int    `json:"reward"`
	RemainingClippings int    `json:"remaining_clippings"`
}

type Bootstrap struct {
	Account struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
	} `json:"account"`
	Profile struct {
		Level int   `json:"level"`
		XP    int64 `json:"xp"`
		Coins int64 `json:"coins"`
	} `json:"profile"`
	Plots          []Plot          `json:"plots"`
	Crops          map[string]Crop `json:"crops"`
	TodayUsage     Usage           `json:"today_usage"`
	ActiveSessions []AgentSession  `json:"active_sessions"`
	SessionCrops   []SessionCrop   `json:"session_crops"`
	Social         FarmSocial      `json:"social"`
	ServerTime     time.Time       `json:"server_time"`
}

type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type EventMetadata struct {
	RuntimeID     string `json:"runtime_id,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	ClientVersion string `json:"client_version,omitempty"`
	DurationMS    int64  `json:"duration_ms,omitempty"`
}

// Event deliberately has no prompt, output, tool arguments, or file fields.
type Event struct {
	EventID           string        `json:"event_id"`
	ExternalSessionID string        `json:"external_session_id"`
	Source            string        `json:"source"`
	Surface           string        `json:"surface"`
	Model             string        `json:"model,omitempty"`
	EventType         string        `json:"event_type"`
	State             string        `json:"state,omitempty"`
	InputTokensDelta  int           `json:"input_tokens_delta"`
	OutputTokensDelta int           `json:"output_tokens_delta"`
	UsageQuality      string        `json:"usage_quality"`
	OccurredAt        time.Time     `json:"occurred_at"`
	Metadata          EventMetadata `json:"metadata"`
}

type ingestResult struct {
	Accepted    int `json:"accepted"`
	Duplicates  int `json:"duplicates"`
	CoinsEarned int `json:"coins_earned"`
}

type state struct {
	AccountHash    string     `json:"account_hash"`
	InstallationID string     `json:"installation_id"`
	DeviceID       string     `json:"device_id,omitempty"`
	LastBootstrap  *Bootstrap `json:"last_bootstrap,omitempty"`
}

type Client struct {
	Config     *cloudauth.Config
	Version    string
	HTTPClient *http.Client
	statePath  string
	queueDir   string
}

func NewClient(cfg *cloudauth.Config, version string) *Client {
	return &Client{
		Config: cfg, Version: version,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		statePath:  harborhome.Path("farm.json"),
		queueDir:   harborhome.Path("farm-queue"),
	}
}

func (c *Client) Bootstrap(ctx context.Context) (*Bootstrap, error) {
	st, err := c.loadState()
	if err != nil {
		return nil, err
	}
	if err := c.ensureDevice(ctx, &st); err != nil {
		if st.LastBootstrap != nil {
			return st.LastBootstrap, fmt.Errorf("farm is offline: %w", err)
		}
		return nil, err
	}
	_ = c.flush(ctx, &st)
	var bootstrap Bootstrap
	if err := c.request(ctx, http.MethodGet, "/api/farm/bootstrap", nil, &bootstrap); err != nil {
		if st.LastBootstrap != nil {
			return st.LastBootstrap, fmt.Errorf("farm is offline: %w", err)
		}
		return nil, err
	}
	st.LastBootstrap = &bootstrap
	if err := c.saveState(st); err != nil {
		return nil, err
	}
	return &bootstrap, nil
}

func (c *Client) Plant(ctx context.Context, plotIndex int, cropType, idempotencyKey string) error {
	payload := map[string]any{"crop_type": cropType, "idempotency_key": idempotencyKey}
	return c.request(ctx, http.MethodPost, fmt.Sprintf("/api/farm/plots/%d/plant", plotIndex), payload, nil)
}

func (c *Client) Harvest(ctx context.Context, plotIndex int) error {
	return c.request(ctx, http.MethodPost, fmt.Sprintf("/api/farm/plots/%d/harvest", plotIndex), map[string]any{}, nil)
}

func (c *Client) ConnectNeighbor(ctx context.Context, farmCode string) error {
	return c.request(ctx, http.MethodPost, "/api/farm/neighbors/connect", map[string]string{"farm_code": farmCode}, nil)
}

func (c *Client) VisitNeighbor(ctx context.Context, farmCode string) (*NeighborFarm, error) {
	var neighbor NeighborFarm
	err := c.request(ctx, http.MethodGet, "/api/farm/neighbors/"+farmCode, nil, &neighbor)
	return &neighbor, err
}

func (c *Client) ForageNeighbor(ctx context.Context, farmCode string, plotIndex int) (*ForageResult, error) {
	var result ForageResult
	err := c.request(ctx, http.MethodPost, fmt.Sprintf("/api/farm/neighbors/%s/plots/%d/forage", farmCode, plotIndex), map[string]any{}, &result)
	return &result, err
}

// Record queues before sending so a temporary network failure does not lose usage.
func (c *Client) Record(ctx context.Context, event Event) error {
	st, err := c.loadState()
	if err != nil {
		return err
	}
	if err := c.queueEvent(event); err != nil {
		return err
	}
	if err := c.ensureDevice(ctx, &st); err != nil {
		return err
	}
	return c.flush(ctx, &st)
}

func (c *Client) ensureDevice(ctx context.Context, st *state) error {
	if st.DeviceID != "" {
		return nil
	}
	hostname, _ := os.Hostname()
	payload := map[string]string{
		"installation_id": st.InstallationID,
		"name":            hostname,
		"platform":        runtime.GOOS,
		"app_version":     c.Version,
	}
	var response struct {
		DeviceID string `json:"device_id"`
	}
	if err := c.request(ctx, http.MethodPost, "/api/farm/devices/register", payload, &response); err != nil {
		return err
	}
	st.DeviceID = response.DeviceID
	return c.saveState(*st)
}

func (c *Client) flush(ctx context.Context, st *state) error {
	for {
		paths, events, err := c.queuedBatch(100)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			return nil
		}
		payload := map[string]any{"device_id": st.DeviceID, "events": events}
		var result ingestResult
		if err := c.request(ctx, http.MethodPost, "/api/farm/events/batch", payload, &result); err != nil {
			return err
		}
		for _, path := range paths {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
}

func (c *Client) queueEvent(event Event) error {
	if err := os.MkdirAll(c.queueDir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(event.EventID))
	path := filepath.Join(c.queueDir, hex.EncodeToString(digest[:])+".json")
	temporary := path + ".tmp-" + uuid.NewString()
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	entries, err := os.ReadDir(c.queueDir)
	if err != nil || len(entries) <= maxQueuedEvents {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		left, _ := entries[i].Info()
		right, _ := entries[j].Info()
		return left.ModTime().Before(right.ModTime())
	})
	for _, entry := range entries[:len(entries)-maxQueuedEvents] {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			_ = os.Remove(filepath.Join(c.queueDir, entry.Name()))
		}
	}
	return nil
}

func (c *Client) queuedBatch(limit int) ([]string, []Event, error) {
	entries, err := os.ReadDir(c.queueDir)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	paths := make([]string, 0, min(limit, len(entries)))
	events := make([]Event, 0, min(limit, len(entries)))
	for _, entry := range entries {
		if len(events) >= limit {
			break
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(c.queueDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		var event Event
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, nil, fmt.Errorf("parsing queued farm event: %w", err)
		}
		paths = append(paths, path)
		events = append(events, event)
	}
	return paths, events, nil
}

func (c *Client) request(ctx context.Context, method, path string, payload, response any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Config.Endpoint+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.Config.APIKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("contacting Harbor Cloud: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<10))
		var apiError struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &apiError)
		if apiError.Error != "" {
			return fmt.Errorf("Harbor Cloud: %s", apiError.Error)
		}
		return fmt.Errorf("Harbor Cloud returned %d", resp.StatusCode)
	}
	if response == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return fmt.Errorf("decoding Harbor Cloud response: %w", err)
	}
	return nil
}

func (c *Client) loadState() (state, error) {
	accountHash := hashAccount(c.Config.APIKey)
	st := state{AccountHash: accountHash, InstallationID: uuid.NewString()}
	data, err := os.ReadFile(c.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return st, c.saveState(st)
		}
		return state{}, fmt.Errorf("reading farm state: %w", err)
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return state{}, fmt.Errorf("parsing farm state: %w", err)
	}
	if st.AccountHash != accountHash {
		return state{AccountHash: accountHash, InstallationID: uuid.NewString()}, nil
	}
	if st.InstallationID == "" {
		st.InstallationID = uuid.NewString()
	}
	return st, nil
}

func (c *Client) saveState(st state) error {
	if err := os.MkdirAll(filepath.Dir(c.statePath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	temporary := c.statePath + ".tmp-" + uuid.NewString()
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, c.statePath)
}

func hashAccount(apiKey string) string {
	digest := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(digest[:])
}
