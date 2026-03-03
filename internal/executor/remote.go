package executor

import (
	"context"

	"github.com/oseaitic/harbor/internal/protocol"
)

// RemoteExecutor is selected by Resolve when the user is logged in to Harbor Cloud.
// Execution always runs locally: if the connector binary is not installed, it is
// pulled from the user's cloud registry (lazy-install) before running.
//
// This keeps compute on the caller's machine (infinite scale, no server bottleneck)
// while still requiring an authenticated cloud account for connector access.
// The cloud gateway (/run) remains available for MCP/SDK web clients that have
// no local environment.
type RemoteExecutor struct {
	endpoint string // reserved: cloud endpoint for registry + future cloud-push
	apiKey   string // reserved: API key for cloud registry access
}

// NewRemoteExecutor creates a RemoteExecutor bound to the user's cloud account.
func NewRemoteExecutor(endpoint, apiKey string) *RemoteExecutor {
	return &RemoteExecutor{endpoint: endpoint, apiKey: apiKey}
}

// Execute runs the connector locally via LocalExecutor.
// If the connector binary is missing, LocalExecutor automatically pulls it
// from the cloud registry before retrying.
func (e *RemoteExecutor) Execute(ctx context.Context, req protocol.Request) (*protocol.Response, error) {
	return NewLocalExecutor().Execute(ctx, req)
}
