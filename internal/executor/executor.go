// Package executor abstracts how connector requests are fulfilled.
//
// LocalExecutor runs connectors as local subprocesses (current behaviour).
// RemoteExecutor (future) will call the Harbor Cloud API.
package executor

import (
	"context"

	"github.com/oseaitic/harbor/internal/protocol"
)

// Executor fulfils a connector request and returns a protocol response.
// Implementations handle auth retrieval and transport internally.
type Executor interface {
	Execute(ctx context.Context, req protocol.Request) (*protocol.Response, error)
}
