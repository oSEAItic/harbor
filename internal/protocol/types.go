package protocol

import (
	"encoding/json"
	"time"
)

// Request is the canonical request format for all connector invocations.
type Request struct {
	Connector string            `json:"connector"`
	Resource  string            `json:"resource"`
	Params    map[string]string `json:"params,omitempty"`
	Auth      string            `json:"auth,omitempty"`
	Raw       bool              `json:"raw,omitempty"`
}

// Response is the canonical response envelope returned by every connector.
type Response struct {
	Data     json.RawMessage  `json:"data"`
	Meta     Meta             `json:"meta"`
	Raw      json.RawMessage  `json:"raw,omitempty"`
	Errors   []ErrorDetail    `json:"errors"`
	Overview *ContextOverview `json:"overview,omitempty"`
	Summary  string           `json:"summary,omitempty"`
}

// ContextOverview provides a structural summary of the data payload.
type ContextOverview struct {
	TotalItems int      `json:"total_items"`
	Fields     []string `json:"fields"`
}

// Meta contains provenance and versioning information.
type Meta struct {
	Source           string     `json:"source"`
	ConnectorVersion string    `json:"connector_version"`
	Schema           string    `json:"schema"`
	ToolSchemaVersion string   `json:"tool_schema_version,omitempty"`
	FetchedAt        time.Time `json:"fetched_at"`
	RequestID        string    `json:"request_id"`
	Pagination       *PageInfo `json:"pagination,omitempty"`
	MemoryID         string    `json:"memory_id,omitempty"`
	FromMemory       bool      `json:"from_memory,omitempty"`
}

// PageInfo enables cursor-based pagination for large result sets.
type PageInfo struct {
	Cursor  string `json:"cursor,omitempty"`
	HasMore bool   `json:"has_more"`
	Total   int    `json:"total,omitempty"`
}

// ErrorDetail represents a structured error in the response.
type ErrorDetail struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Detail  string    `json:"detail,omitempty"`
}

// ErrorCode is an enumerated error type.
type ErrorCode string

const (
	ErrRateLimitExceeded    ErrorCode = "rate_limit_exceeded"
	ErrAuthRequired         ErrorCode = "auth_required"
	ErrSchemaValidation     ErrorCode = "schema_validation_error"
	ErrExecution            ErrorCode = "execution_error"
	ErrConnectorNotFound    ErrorCode = "connector_not_found"
	ErrResourceNotFound     ErrorCode = "resource_not_found"
	ErrInvalidParams        ErrorCode = "invalid_params"
)

// ToolSchema describes a connector resource in OpenAI function-calling format.
type ToolSchema struct {
	Type     string         `json:"type"`
	Function ToolFunction   `json:"function"`
}

// ToolFunction is the function definition inside a ToolSchema.
type ToolFunction struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Parameters      json.RawMessage `json:"parameters"`
	SummaryFields   []string        `json:"summary_fields,omitempty"`
	SummaryTemplate string          `json:"summary_template,omitempty"`
}
