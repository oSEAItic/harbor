package worklog

import "time"

const (
	StatusActive   = "active"
	StatusBlocked  = "blocked"
	StatusVerified = "verified"
	StatusShipped  = "shipped"
)

const (
	EventStarted    = "started"
	EventCheckpoint = "checkpoint"
	EventBlocked    = "blocked"
	EventResumed    = "resumed"
	EventVerified   = "verified"
	EventShipped    = "shipped"
	EventReopened   = "reopened"
	EventScope      = "scope"
)

type Feature struct {
	ID            string    `json:"id"`
	Project       string    `json:"project"`
	Title         string    `json:"title"`
	Kind          string    `json:"type,omitempty"`
	Size          string    `json:"size,omitempty"`
	BudgetSeconds int64     `json:"budget_seconds,omitempty"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Event struct {
	ID        int64     `json:"id"`
	FeatureID string    `json:"feature_id"`
	Kind      string    `json:"kind"`
	Note      string    `json:"note,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type SessionBinding struct {
	FeatureID         string    `json:"feature_id"`
	HarborSessionID   string    `json:"harbor_session_id"`
	Source            string    `json:"source,omitempty"`
	ModelName         string    `json:"model_name,omitempty"`
	ExternalSessionID string    `json:"external_session_id,omitempty"`
	RepoPath          string    `json:"repo_path,omitempty"`
	Branch            string    `json:"branch,omitempty"`
	BoundAt           time.Time `json:"bound_at"`
}

type ScopeItem struct {
	ID        int64     `json:"id"`
	FeatureID string    `json:"feature_id"`
	Decision  string    `json:"decision"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type Detail struct {
	Feature  Feature          `json:"feature"`
	Events   []Event          `json:"events"`
	Sessions []SessionBinding `json:"sessions"`
	Scope    []ScopeItem      `json:"scope"`
}

type FeatureStats struct {
	Feature                Feature `json:"feature"`
	CycleSeconds           int64   `json:"cycle_seconds"`
	BlockedSeconds         int64   `json:"blocked_seconds"`
	VerificationLagSeconds int64   `json:"verification_lag_seconds"`
	SessionCount           int     `json:"session_count"`
	ScopeAdded             int     `json:"scope_added"`
	ScopeDeferred          int     `json:"scope_deferred"`
}

type Report struct {
	Since           time.Time      `json:"since"`
	GeneratedAt     time.Time      `json:"generated_at"`
	Features        []FeatureStats `json:"features"`
	ActiveCount     int            `json:"active_count"`
	BlockedCount    int            `json:"blocked_count"`
	VerifiedCount   int            `json:"verified_count"`
	ShippedCount    int            `json:"shipped_count"`
	TotalSessions   int            `json:"total_sessions"`
	TotalScopeAdded int            `json:"total_scope_added"`
}

type Estimate struct {
	Project         string `json:"project,omitempty"`
	Kind            string `json:"type,omitempty"`
	Size            string `json:"size,omitempty"`
	Samples         int    `json:"samples"`
	P50CycleSeconds int64  `json:"p50_cycle_seconds"`
	P80CycleSeconds int64  `json:"p80_cycle_seconds"`
}
