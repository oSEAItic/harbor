package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oseaitic/harbor/internal/connector"
	harborctx "github.com/oseaitic/harbor/internal/context"
	"github.com/oseaitic/harbor/internal/executor"
	"github.com/oseaitic/harbor/internal/govern"
	"github.com/oseaitic/harbor/internal/memory"
	"github.com/oseaitic/harbor/internal/protocol"
)

// Result holds the output of a pipeline execution.
type Result struct {
	Response            *protocol.Response
	MemoryID            string
	FromMem             bool
	GovernFieldsRemoved int // number of field types removed by govern filter
}

// ErrorMode controls whether pipeline errors are hard-fail or soft-degraded.
type ErrorMode string

const (
	// ErrorModeSoft degrades to the normalized/raw response when compile validation fails.
	ErrorModeSoft ErrorMode = "soft"
	// ErrorModeHard returns an error immediately when compile validation fails.
	ErrorModeHard ErrorMode = "hard"
)

// Options controls pipeline behaviour.
type Options struct {
	NoMemory      bool
	Refresh       bool
	Compile       harborctx.CompileOptions
	Errors        ErrorMode
	MaxVisibility string // govern Layer 3: field visibility ceiling (empty = no filtering)
}

// Execute runs the full fetch→compile→memory pipeline for a connector resource.
// It checks memory first (unless NoMemory/Refresh), executes the connector via
// the provided Executor, compiles the response, and saves the result to memory.
func Execute(exec executor.Executor, connectorName, resource string, params map[string]string, summaryFields []string, opts Options) (*Result, error) {
	// Memory check: try to serve from memory before fetching
	if !opts.NoMemory && !opts.Refresh {
		store, err := memory.NewStore()
		if err == nil {
			entry := store.Latest(connectorName, resource, params)
			if store.IsFresh(entry) {
				obj, err := store.Get(entry.ID)
				if err == nil {
					resp := responseFromMemory(obj)
					return &Result{
						Response: resp,
						MemoryID: obj.ID,
						FromMem:  true,
					}, nil
				}
			}
		}
	}

	req := protocol.Request{
		Connector: connectorName,
		Resource:  resource,
		Params:    params,
	}

	resp, err := exec.Execute(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("executing connector: %w", err)
	}

	// Fetch summary fields and field visibility from connector schema
	schema := resp.Meta.Schema
	var fieldVisibility map[string]string
	if len(summaryFields) == 0 && opts.Compile.Mode != "full" {
		if tf, err := connector.GetResourceSchema(connectorName, resource); err == nil {
			summaryFields = tf.SummaryFields
			fieldVisibility = tf.FieldVisibility
		}
	}

	// Govern Layer 3: filter fields by visibility before compile
	var governResult govern.FilterResult
	if opts.MaxVisibility != "" && len(fieldVisibility) > 0 {
		var items []map[string]interface{}
		if err := json.Unmarshal(resp.Data, &items); err == nil && len(items) > 0 {
			items, governResult = govern.FilterItems(items, fieldVisibility, opts.MaxVisibility)
			if filtered, err := json.Marshal(items); err == nil {
				resp.Data = filtered
			}
		}
	}

	// Compile the response
	compiled := harborctx.Compile(resp, summaryFields, schema, opts.Compile)
	if err := protocol.ValidateResponse(compiled); err != nil {
		if opts.errorMode() == ErrorModeHard {
			return nil, fmt.Errorf("compiled response validation failed: %w", err)
		}

		// Soft degrade to the normalized response to keep the tool call usable.
		fallback := *resp
		fallback.Errors = append(fallback.Errors, protocol.ErrorDetail{
			Code:    protocol.ErrSchemaValidation,
			Message: fmt.Sprintf("compile validation failed, served normalized data: %v", err),
		})
		compiled = &fallback
	}

	// Save to memory
	var memID string
	if !opts.NoMemory {
		store, err := memory.NewStore()
		if err == nil {
			memObj := BuildMemoryObject(connectorName, resource, params, resp, compiled, schema)
			if id, err := store.Save(memObj); err == nil {
				memID = id
				compiled.Meta.MemoryID = id
				compiled.Meta.FromMemory = false
			}
		}
	}

	return &Result{
		Response:            compiled,
		MemoryID:            memID,
		FromMem:             false,
		GovernFieldsRemoved: governResult.RemovedCount,
	}, nil
}

func (o Options) errorMode() ErrorMode {
	if o.Errors == "" {
		return ErrorModeSoft
	}
	return o.Errors
}

// BuildMemoryObject constructs a 4-layer memory object from fetch results.
func BuildMemoryObject(connectorName, resource string, params map[string]string, rawResp, compiledResp *protocol.Response, schema string) *memory.Object {
	return &memory.Object{
		Connector:  connectorName,
		Resource:   resource,
		Schema:     schema,
		Params:     params,
		TTLSeconds: int(memory.DefaultTTL(connectorName, resource).Seconds()),
		Layers: memory.Layers{
			Raw:        rawResp.Raw,
			Normalized: rawResp.Data,
			Compact:    compiledResp.Data,
			Summary:    compiledResp.Summary,
		},
		Meta: memory.ObjectMeta{
			Source:           rawResp.Meta.Source,
			ConnectorVersion: rawResp.Meta.ConnectorVersion,
			RequestID:        rawResp.Meta.RequestID,
		},
	}
}

// responseFromMemory builds a protocol.Response from a stored memory object,
// serving the compact layer by default.
func responseFromMemory(obj *memory.Object) *protocol.Response {
	return &protocol.Response{
		Data: obj.Layers.Compact,
		Meta: protocol.Meta{
			Source:           obj.Meta.Source,
			ConnectorVersion: obj.Meta.ConnectorVersion,
			Schema:           obj.Schema,
			FetchedAt:        obj.CreatedAt,
			RequestID:        obj.Meta.RequestID,
			MemoryID:         obj.ID,
			FromMemory:       true,
		},
		Summary: obj.Layers.Summary,
		Errors:  []protocol.ErrorDetail{},
	}
}

// ParseConnectorResource splits "connector.resource" into its parts.
func ParseConnectorResource(arg string) (connectorName, resource string, err error) {
	parts := strings.SplitN(arg, ".", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("resource must be in format <connector>.<resource>, got %q", arg)
	}
	if err := connector.ValidateConnectorName(parts[0]); err != nil {
		return "", "", err
	}
	return parts[0], parts[1], nil
}
