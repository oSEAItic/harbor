package worklog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oseaitic/harbor/internal/harborhome"
	_ "modernc.org/sqlite"
)

const schemaVersion = 5

var commitSHAPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func NewStore() (*Store, error) {
	return NewStoreAt(harborhome.Path("worklog.db"))
}

func NewStoreAt(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating worklog directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening worklog database: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, now: func() time.Time { return time.Now().UTC() }}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS features (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			title TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT '',
			size TEXT NOT NULL DEFAULT '',
			budget_seconds INTEGER NOT NULL DEFAULT 0,
			target_date TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_features_project_status ON features(project, status)`,
		`CREATE TABLE IF NOT EXISTS feature_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			session_id TEXT NOT NULL DEFAULT '',
			commit_sha TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_feature_events_feature_time ON feature_events(feature_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS feature_sessions (
			feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
			harbor_session_id TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT '',
			model_name TEXT NOT NULL DEFAULT '',
			external_session_id TEXT NOT NULL DEFAULT '',
			repo_path TEXT NOT NULL DEFAULT '',
			branch TEXT NOT NULL DEFAULT '',
			bound_at TEXT NOT NULL,
			UNIQUE(feature_id, harbor_session_id, source, external_session_id)
		)`,
		`CREATE TABLE IF NOT EXISTS scope_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
			decision TEXT NOT NULL,
			text TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS checkpoint_summaries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
			repo_path TEXT NOT NULL,
			base_sha TEXT NOT NULL,
			head_sha TEXT NOT NULL,
			outcome TEXT NOT NULL,
			decisions_json TEXT NOT NULL DEFAULT '[]',
			verification_json TEXT NOT NULL DEFAULT '[]',
			remaining_json TEXT NOT NULL DEFAULT '[]',
			session_id TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			model_name TEXT NOT NULL DEFAULT '',
			schema_version INTEGER NOT NULL DEFAULT 1,
			generated_at TEXT NOT NULL,
			UNIQUE(feature_id, repo_path, base_sha, head_sha)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_checkpoint_summaries_feature_head ON checkpoint_summaries(feature_id, head_sha)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrating worklog database: %w", err)
		}
	}
	var currentVersion sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_version`).Scan(&currentVersion); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}
	if currentVersion.Valid && currentVersion.Int64 > schemaVersion {
		return fmt.Errorf("worklog schema version %d is newer than supported version %d", currentVersion.Int64, schemaVersion)
	}
	if err := s.ensureColumn(ctx, "feature_sessions", "model_name", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "feature_events", "commit_sha", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "features", "target_date", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if currentVersion.Valid && currentVersion.Int64 == schemaVersion {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM schema_version`); err != nil {
		return fmt.Errorf("resetting schema version: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO schema_version(version) VALUES (?)`, schemaVersion); err != nil {
		return fmt.Errorf("writing schema version: %w", err)
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table, column, definition string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return fmt.Errorf("reading %s columns: %w", table, err)
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return fmt.Errorf("scanning %s columns: %w", table, err)
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("reading %s columns: %w", table, err)
	}
	rows.Close()
	if found {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition); err != nil {
		return fmt.Errorf("adding %s.%s: %w", table, column, err)
	}
	return nil
}

func (s *Store) CreateFeature(ctx context.Context, project, title, kind, size string, budget time.Duration, targetDate string) (Feature, error) {
	project = strings.TrimSpace(project)
	title = strings.TrimSpace(title)
	if project == "" || title == "" {
		return Feature{}, errors.New("project and title are required")
	}
	if budget < 0 {
		return Feature{}, errors.New("budget cannot be negative")
	}
	targetDate, err := normalizeTargetDate(targetDate)
	if err != nil {
		return Feature{}, err
	}
	now := s.now()
	f := Feature{
		ID: "feat_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12], Project: project, Title: title,
		Kind: strings.ToLower(strings.TrimSpace(kind)), Size: strings.ToUpper(strings.TrimSpace(size)), BudgetSeconds: int64(budget.Seconds()), TargetDate: targetDate,
		Status: StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Feature{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO features(id, project, title, kind, size, budget_seconds, target_date, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.Project, f.Title, f.Kind, f.Size, f.BudgetSeconds, f.TargetDate, f.Status, formatTime(now), formatTime(now)); err != nil {
		return Feature{}, fmt.Errorf("creating feature: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO feature_events(feature_id, kind, created_at) VALUES (?, ?, ?)`, f.ID, EventStarted, formatTime(now)); err != nil {
		return Feature{}, fmt.Errorf("recording feature start: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Feature{}, err
	}
	return f, nil
}

func normalizeTargetDate(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil || parsed.Format("2006-01-02") != value {
		return "", fmt.Errorf("target date must use YYYY-MM-DD")
	}
	return value, nil
}

func (s *Store) SetTargetDate(ctx context.Context, featureID, targetDate string) (Feature, error) {
	targetDate, err := normalizeTargetDate(targetDate)
	if err != nil {
		return Feature{}, err
	}
	now := s.now()
	result, err := s.db.ExecContext(ctx, `UPDATE features SET target_date = ?, updated_at = ? WHERE id = ?`, targetDate, formatTime(now), strings.TrimSpace(featureID))
	if err != nil {
		return Feature{}, fmt.Errorf("updating feature target date: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return Feature{}, err
	}
	if changed == 0 {
		return Feature{}, fmt.Errorf("feature %q not found", featureID)
	}
	return s.GetFeature(ctx, featureID)
}

func (s *Store) AddEvent(ctx context.Context, featureID, kind, note, sessionID string) (Feature, error) {
	return s.AddEventWithCommit(ctx, featureID, kind, note, sessionID, "")
}

func (s *Store) addEventAt(ctx context.Context, featureID, kind, note, sessionID string, at time.Time) (Feature, error) {
	return s.addEventAtWithCommit(ctx, featureID, kind, note, sessionID, "", at)
}

func (s *Store) AddEventWithCommit(ctx context.Context, featureID, kind, note, sessionID, commitSHA string) (Feature, error) {
	return s.addEventAtWithCommit(ctx, featureID, kind, note, sessionID, commitSHA, s.now())
}

func (s *Store) addEventAtWithCommit(ctx context.Context, featureID, kind, note, sessionID, commitSHA string, at time.Time) (Feature, error) {
	commitSHA = strings.ToLower(strings.TrimSpace(commitSHA))
	if commitSHA != "" && !commitSHAPattern.MatchString(commitSHA) {
		return Feature{}, errors.New("commit SHA must be 7 to 64 hexadecimal characters")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Feature{}, err
	}
	defer tx.Rollback()
	f, err := getFeature(ctx, tx, featureID)
	if err != nil {
		return Feature{}, err
	}
	next, err := nextStatus(f.Status, kind)
	if err != nil {
		return Feature{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO feature_events(feature_id, kind, note, session_id, commit_sha, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		f.ID, kind, strings.TrimSpace(note), strings.TrimSpace(sessionID), commitSHA, formatTime(at)); err != nil {
		return Feature{}, fmt.Errorf("recording feature event: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE features SET status = ?, updated_at = ? WHERE id = ?`, next, formatTime(at), f.ID); err != nil {
		return Feature{}, fmt.Errorf("updating feature: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Feature{}, err
	}
	f.Status, f.UpdatedAt = next, at.UTC()
	return f, nil
}

func nextStatus(current, event string) (string, error) {
	switch event {
	case EventCheckpoint, EventScope:
		if current == StatusShipped {
			return "", errors.New("shipped feature must be reopened before adding events")
		}
		return current, nil
	case EventBlocked:
		if current != StatusActive {
			return "", fmt.Errorf("cannot block feature in %s state", current)
		}
		return StatusBlocked, nil
	case EventResumed:
		if current != StatusBlocked {
			return "", fmt.Errorf("cannot resume feature in %s state", current)
		}
		return StatusActive, nil
	case EventVerified:
		if current != StatusActive {
			return "", fmt.Errorf("cannot verify feature in %s state", current)
		}
		return StatusVerified, nil
	case EventShipped:
		if current != StatusVerified {
			return "", errors.New("feature must be verified before shipping")
		}
		return StatusShipped, nil
	case EventReopened:
		if current != StatusVerified && current != StatusShipped {
			return "", fmt.Errorf("cannot reopen feature in %s state", current)
		}
		return StatusActive, nil
	default:
		return "", fmt.Errorf("unknown feature event %q", event)
	}
}

func (s *Store) BindSession(ctx context.Context, binding SessionBinding) error {
	binding.FeatureID = strings.TrimSpace(binding.FeatureID)
	binding.HarborSessionID = strings.TrimSpace(binding.HarborSessionID)
	binding.Source = strings.TrimSpace(binding.Source)
	binding.ModelName = strings.TrimSpace(binding.ModelName)
	binding.ExternalSessionID = strings.TrimSpace(binding.ExternalSessionID)
	binding.RepoPath = strings.TrimSpace(binding.RepoPath)
	binding.Branch = strings.TrimSpace(binding.Branch)
	if binding.FeatureID == "" || binding.HarborSessionID == "" {
		return errors.New("feature ID and Harbor session ID are required")
	}
	if _, err := s.GetFeature(ctx, binding.FeatureID); err != nil {
		return err
	}
	if binding.BoundAt.IsZero() {
		binding.BoundAt = s.now()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO feature_sessions(feature_id, harbor_session_id, source, model_name, external_session_id, repo_path, branch, bound_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(feature_id, harbor_session_id, source, external_session_id) DO UPDATE SET
			model_name = CASE WHEN excluded.model_name <> '' THEN excluded.model_name ELSE feature_sessions.model_name END,
			repo_path = CASE WHEN excluded.repo_path <> '' THEN excluded.repo_path ELSE feature_sessions.repo_path END,
			branch = CASE WHEN excluded.branch <> '' THEN excluded.branch ELSE feature_sessions.branch END,
			bound_at = excluded.bound_at`,
		binding.FeatureID, binding.HarborSessionID, binding.Source, binding.ModelName, binding.ExternalSessionID, binding.RepoPath, binding.Branch, formatTime(binding.BoundAt)); err != nil {
		return fmt.Errorf("binding session: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE features SET updated_at = ? WHERE id = ?`, formatTime(binding.BoundAt), binding.FeatureID); err != nil {
		return fmt.Errorf("updating feature session activity: %w", err)
	}
	return tx.Commit()
}

func (s *Store) AddScope(ctx context.Context, featureID, decision, text string) error {
	decision, text = strings.ToLower(strings.TrimSpace(decision)), strings.TrimSpace(text)
	if decision != "include" && decision != "swap" && decision != "defer" && decision != "reject" {
		return errors.New("scope decision must be include, swap, defer, or reject")
	}
	if text == "" {
		return errors.New("scope description is required")
	}
	f, err := s.GetFeature(ctx, featureID)
	if err != nil {
		return err
	}
	if f.Status == StatusShipped {
		return errors.New("shipped feature must be reopened before changing scope")
	}
	if f.Status == StatusVerified && (decision == "include" || decision == "swap") {
		return errors.New("verified feature must be reopened before expanding scope")
	}
	now := s.now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO scope_items(feature_id, decision, text, created_at) VALUES (?, ?, ?, ?)`, featureID, decision, text, formatTime(now)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO feature_events(feature_id, kind, note, created_at) VALUES (?, ?, ?, ?)`, featureID, EventScope, decision+": "+text, formatTime(now)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE features SET updated_at = ? WHERE id = ?`, formatTime(now), featureID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UpsertCheckpointSummary(ctx context.Context, summary CheckpointSummary) (CheckpointSummary, error) {
	summary.FeatureID = strings.TrimSpace(summary.FeatureID)
	summary.RepoPath = filepath.Clean(strings.TrimSpace(summary.RepoPath))
	summary.BaseSHA = strings.ToLower(strings.TrimSpace(summary.BaseSHA))
	summary.HeadSHA = strings.ToLower(strings.TrimSpace(summary.HeadSHA))
	summary.Outcome = strings.TrimSpace(summary.Outcome)
	summary.SessionID = strings.TrimSpace(summary.SessionID)
	summary.Source = strings.TrimSpace(summary.Source)
	summary.ModelName = strings.TrimSpace(summary.ModelName)
	summary.Decisions = cleanStrings(summary.Decisions)
	summary.Verification = cleanStrings(summary.Verification)
	summary.Remaining = cleanStrings(summary.Remaining)
	if summary.FeatureID == "" || summary.RepoPath == "." || summary.Outcome == "" {
		return CheckpointSummary{}, errors.New("feature ID, repository path, and outcome are required")
	}
	if !commitSHAPattern.MatchString(summary.BaseSHA) || !commitSHAPattern.MatchString(summary.HeadSHA) {
		return CheckpointSummary{}, errors.New("base and head commits must be 7-64 character hexadecimal Git SHAs")
	}
	if summary.BaseSHA == summary.HeadSHA {
		return CheckpointSummary{}, errors.New("base and head commits must be different")
	}
	if summary.SchemaVersion == 0 {
		summary.SchemaVersion = 1
	}
	if summary.SchemaVersion != 1 {
		return CheckpointSummary{}, fmt.Errorf("unsupported checkpoint summary schema version %d", summary.SchemaVersion)
	}
	if _, err := s.GetFeature(ctx, summary.FeatureID); err != nil {
		return CheckpointSummary{}, err
	}
	decisions, _ := json.Marshal(summary.Decisions)
	verification, _ := json.Marshal(summary.Verification)
	remaining, _ := json.Marshal(summary.Remaining)
	summary.GeneratedAt = s.now()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CheckpointSummary{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO checkpoint_summaries(
		feature_id, repo_path, base_sha, head_sha, outcome, decisions_json, verification_json, remaining_json,
		session_id, source, model_name, schema_version, generated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(feature_id, repo_path, base_sha, head_sha) DO UPDATE SET
		outcome = excluded.outcome,
		decisions_json = excluded.decisions_json,
		verification_json = excluded.verification_json,
		remaining_json = excluded.remaining_json,
		session_id = excluded.session_id,
		source = excluded.source,
		model_name = excluded.model_name,
		schema_version = excluded.schema_version,
		generated_at = excluded.generated_at`,
		summary.FeatureID, summary.RepoPath, summary.BaseSHA, summary.HeadSHA, summary.Outcome,
		string(decisions), string(verification), string(remaining), summary.SessionID, summary.Source,
		summary.ModelName, summary.SchemaVersion, formatTime(summary.GeneratedAt))
	if err != nil {
		return CheckpointSummary{}, fmt.Errorf("saving checkpoint summary: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE features SET updated_at = ? WHERE id = ?`, formatTime(summary.GeneratedAt), summary.FeatureID); err != nil {
		return CheckpointSummary{}, fmt.Errorf("updating feature checkpoint activity: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return CheckpointSummary{}, err
	}
	return s.checkpointSummary(ctx, summary.FeatureID, summary.RepoPath, summary.BaseSHA, summary.HeadSHA)
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s *Store) GetFeature(ctx context.Context, id string) (Feature, error) {
	return getFeature(ctx, s.db, id)
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getFeature(ctx context.Context, q queryRower, id string) (Feature, error) {
	var f Feature
	var created, updated string
	err := q.QueryRowContext(ctx, `SELECT id, project, title, kind, size, budget_seconds, target_date, status, created_at, updated_at FROM features WHERE id = ?`, strings.TrimSpace(id)).Scan(
		&f.ID, &f.Project, &f.Title, &f.Kind, &f.Size, &f.BudgetSeconds, &f.TargetDate, &f.Status, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Feature{}, fmt.Errorf("feature %q not found", id)
	}
	if err != nil {
		return Feature{}, fmt.Errorf("reading feature: %w", err)
	}
	f.CreatedAt, err = parseTime(created)
	if err != nil {
		return Feature{}, err
	}
	f.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return Feature{}, err
	}
	return f, nil
}

func (s *Store) ListFeatures(ctx context.Context, project, status string) ([]Feature, error) {
	query := `SELECT id, project, title, kind, size, budget_seconds, target_date, status, created_at, updated_at FROM features WHERE 1=1`
	var args []any
	if strings.TrimSpace(project) != "" {
		query += ` AND project = ?`
		args = append(args, strings.TrimSpace(project))
	}
	if strings.TrimSpace(status) != "" {
		query += ` AND status = ?`
		args = append(args, strings.ToLower(strings.TrimSpace(status)))
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Feature
	for rows.Next() {
		var f Feature
		var created, updated string
		if err := rows.Scan(&f.ID, &f.Project, &f.Title, &f.Kind, &f.Size, &f.BudgetSeconds, &f.TargetDate, &f.Status, &created, &updated); err != nil {
			return nil, err
		}
		f.CreatedAt, _ = parseTime(created)
		f.UpdatedAt, _ = parseTime(updated)
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) Detail(ctx context.Context, id string) (Detail, error) {
	f, err := s.GetFeature(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	d := Detail{
		Feature:             f,
		Events:              make([]Event, 0),
		Sessions:            make([]SessionBinding, 0),
		Scope:               make([]ScopeItem, 0),
		CheckpointSummaries: make([]CheckpointSummary, 0),
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, feature_id, kind, note, session_id, commit_sha, created_at FROM feature_events WHERE feature_id = ? ORDER BY created_at, id`, id)
	if err != nil {
		return Detail{}, err
	}
	for rows.Next() {
		var e Event
		var at string
		if err := rows.Scan(&e.ID, &e.FeatureID, &e.Kind, &e.Note, &e.SessionID, &e.CommitSHA, &at); err != nil {
			rows.Close()
			return Detail{}, err
		}
		e.CreatedAt, _ = parseTime(at)
		d.Events = append(d.Events, e)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Detail{}, err
	}
	rows.Close()
	rows, err = s.db.QueryContext(ctx, `SELECT feature_id, harbor_session_id, source, model_name, external_session_id, repo_path, branch, bound_at FROM feature_sessions WHERE feature_id = ? ORDER BY bound_at`, id)
	if err != nil {
		return Detail{}, err
	}
	for rows.Next() {
		var b SessionBinding
		var at string
		if err := rows.Scan(&b.FeatureID, &b.HarborSessionID, &b.Source, &b.ModelName, &b.ExternalSessionID, &b.RepoPath, &b.Branch, &at); err != nil {
			rows.Close()
			return Detail{}, err
		}
		b.BoundAt, _ = parseTime(at)
		d.Sessions = append(d.Sessions, b)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Detail{}, err
	}
	rows.Close()
	rows, err = s.db.QueryContext(ctx, `SELECT id, feature_id, decision, text, created_at FROM scope_items WHERE feature_id = ? ORDER BY created_at, id`, id)
	if err != nil {
		return Detail{}, err
	}
	for rows.Next() {
		var item ScopeItem
		var at string
		if err := rows.Scan(&item.ID, &item.FeatureID, &item.Decision, &item.Text, &at); err != nil {
			rows.Close()
			return Detail{}, err
		}
		item.CreatedAt, _ = parseTime(at)
		d.Scope = append(d.Scope, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Detail{}, err
	}
	rows.Close()
	rows, err = s.db.QueryContext(ctx, `SELECT id, feature_id, repo_path, base_sha, head_sha, outcome,
		decisions_json, verification_json, remaining_json, session_id, source, model_name, schema_version, generated_at
		FROM checkpoint_summaries WHERE feature_id = ? ORDER BY generated_at, id`, id)
	if err != nil {
		return Detail{}, err
	}
	defer rows.Close()
	for rows.Next() {
		summary, err := scanCheckpointSummary(rows)
		if err != nil {
			return Detail{}, err
		}
		d.CheckpointSummaries = append(d.CheckpointSummaries, summary)
	}
	return d, rows.Err()
}

type rowScanner interface {
	Scan(...any) error
}

func scanCheckpointSummary(row rowScanner) (CheckpointSummary, error) {
	var summary CheckpointSummary
	var decisions, verification, remaining, generatedAt string
	if err := row.Scan(
		&summary.ID, &summary.FeatureID, &summary.RepoPath, &summary.BaseSHA, &summary.HeadSHA, &summary.Outcome,
		&decisions, &verification, &remaining, &summary.SessionID, &summary.Source, &summary.ModelName,
		&summary.SchemaVersion, &generatedAt,
	); err != nil {
		return CheckpointSummary{}, err
	}
	if err := json.Unmarshal([]byte(decisions), &summary.Decisions); err != nil {
		return CheckpointSummary{}, fmt.Errorf("decoding checkpoint decisions: %w", err)
	}
	if err := json.Unmarshal([]byte(verification), &summary.Verification); err != nil {
		return CheckpointSummary{}, fmt.Errorf("decoding checkpoint verification: %w", err)
	}
	if err := json.Unmarshal([]byte(remaining), &summary.Remaining); err != nil {
		return CheckpointSummary{}, fmt.Errorf("decoding checkpoint remaining work: %w", err)
	}
	summary.GeneratedAt, _ = parseTime(generatedAt)
	return summary, nil
}

func (s *Store) checkpointSummary(ctx context.Context, featureID, repoPath, baseSHA, headSHA string) (CheckpointSummary, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, feature_id, repo_path, base_sha, head_sha, outcome,
		decisions_json, verification_json, remaining_json, session_id, source, model_name, schema_version, generated_at
		FROM checkpoint_summaries WHERE feature_id = ? AND repo_path = ? AND base_sha = ? AND head_sha = ?`,
		featureID, repoPath, baseSHA, headSHA)
	return scanCheckpointSummary(row)
}

func (s *Store) ResolveFeatureContext(ctx context.Context, sessionID, repoPath, branch, project string) (FeatureContext, error) {
	sessionID = strings.TrimSpace(sessionID)
	repoPath = filepath.Clean(strings.TrimSpace(repoPath))
	branch = strings.TrimSpace(branch)
	project = strings.TrimSpace(project)
	if sessionID != "" {
		var featureID string
		err := s.db.QueryRowContext(ctx, `SELECT feature_id FROM feature_sessions
			WHERE harbor_session_id = ? OR external_session_id = ? ORDER BY bound_at DESC LIMIT 1`, sessionID, sessionID).Scan(&featureID)
		if err == nil {
			detail, detailErr := s.Detail(ctx, featureID)
			return FeatureContext{Match: "session", Detail: &detail}, detailErr
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return FeatureContext{}, err
		}
	}
	if repoPath != "." {
		query := `SELECT feature_id FROM feature_sessions WHERE repo_path = ?`
		args := []any{repoPath}
		if branch != "" {
			query += ` AND branch = ?`
			args = append(args, branch)
		}
		query += ` ORDER BY bound_at DESC LIMIT 1`
		var featureID string
		err := s.db.QueryRowContext(ctx, query, args...).Scan(&featureID)
		if err == nil {
			detail, detailErr := s.Detail(ctx, featureID)
			return FeatureContext{Match: "repository", Detail: &detail}, detailErr
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return FeatureContext{}, err
		}
	}
	if project != "" {
		features, err := s.ListFeatures(ctx, project, StatusActive)
		if err != nil {
			return FeatureContext{}, err
		}
		if len(features) == 1 {
			detail, detailErr := s.Detail(ctx, features[0].ID)
			return FeatureContext{Match: "project", Detail: &detail}, detailErr
		}
		if len(features) > 1 {
			return FeatureContext{Match: "ambiguous"}, nil
		}
	}
	return FeatureContext{Match: "none"}, nil
}

func (s *Store) BuildReport(ctx context.Context, since, now time.Time) (Report, error) {
	if now.IsZero() {
		now = s.now()
	}
	features, err := s.ListFeatures(ctx, "", "")
	if err != nil {
		return Report{}, err
	}
	r := Report{Since: since, GeneratedAt: now}
	for _, f := range features {
		if !since.IsZero() && f.UpdatedAt.Before(since) {
			continue
		}
		d, err := s.Detail(ctx, f.ID)
		if err != nil {
			return Report{}, err
		}
		stats := calculateStats(d, now)
		r.Features = append(r.Features, stats)
		r.TotalSessions += stats.SessionCount
		r.TotalScopeAdded += stats.ScopeAdded
		switch f.Status {
		case StatusActive:
			r.ActiveCount++
		case StatusBlocked:
			r.BlockedCount++
		case StatusVerified:
			r.VerifiedCount++
		case StatusShipped:
			r.ShippedCount++
		}
	}
	return r, nil
}

func calculateStats(d Detail, now time.Time) FeatureStats {
	s := FeatureStats{Feature: d.Feature, SessionCount: len(d.Sessions)}
	end := now
	var blockedAt, verifiedAt time.Time
	for _, e := range d.Events {
		switch e.Kind {
		case EventBlocked:
			blockedAt = e.CreatedAt
		case EventResumed, EventVerified, EventShipped:
			if !blockedAt.IsZero() {
				s.BlockedSeconds += int64(e.CreatedAt.Sub(blockedAt).Seconds())
				blockedAt = time.Time{}
			}
		}
		if e.Kind == EventVerified {
			verifiedAt = e.CreatedAt
		}
		if e.Kind == EventShipped {
			end = e.CreatedAt
			if !verifiedAt.IsZero() {
				s.VerificationLagSeconds = int64(e.CreatedAt.Sub(verifiedAt).Seconds())
			}
		}
		if e.Kind == EventReopened {
			end = now
			verifiedAt = time.Time{}
			s.VerificationLagSeconds = 0
		}
	}
	if !blockedAt.IsZero() {
		s.BlockedSeconds += int64(now.Sub(blockedAt).Seconds())
	}
	s.CycleSeconds = int64(end.Sub(d.Feature.CreatedAt).Seconds())
	for _, item := range d.Scope {
		if item.Decision == "include" {
			s.ScopeAdded++
		}
		if item.Decision == "defer" {
			s.ScopeDeferred++
		}
	}
	return s
}

func (s *Store) Estimate(ctx context.Context, project, kind, size string) (Estimate, error) {
	query := `SELECT id FROM features WHERE status = ?`
	args := []any{StatusShipped}
	for _, filter := range []struct{ value, column string }{{project, "project"}, {strings.ToLower(kind), "kind"}, {strings.ToUpper(size), "size"}} {
		if strings.TrimSpace(filter.value) != "" {
			query += ` AND ` + filter.column + ` = ?`
			args = append(args, strings.TrimSpace(filter.value))
		}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Estimate{}, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return Estimate{}, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	var durations []time.Duration
	for _, id := range ids {
		d, err := s.Detail(ctx, id)
		if err != nil {
			return Estimate{}, err
		}
		durations = append(durations, time.Duration(calculateStats(d, d.Feature.UpdatedAt).CycleSeconds)*time.Second)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	e := Estimate{Project: project, Kind: kind, Size: strings.ToUpper(size), Samples: len(durations)}
	if len(durations) > 0 {
		e.P50CycleSeconds = int64(percentile(durations, 0.50).Seconds())
		e.P80CycleSeconds = int64(percentile(durations, 0.80).Seconds())
	}
	return e, nil
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1)*p + 0.5)
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func formatTime(t time.Time) string         { return t.UTC().Format(time.RFC3339Nano) }
func parseTime(v string) (time.Time, error) { return time.Parse(time.RFC3339Nano, v) }
