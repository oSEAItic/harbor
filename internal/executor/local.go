package executor

import (
	"context"

	"github.com/oseaitic/harbor/internal/auth"
	"github.com/oseaitic/harbor/internal/connector"
	"github.com/oseaitic/harbor/internal/protocol"
)

// LocalExecutor runs connectors as local subprocesses.
// Auth is retrieved from the OS keychain and injected via the HARBOR_AUTH
// environment variable.
type LocalExecutor struct{}

// NewLocalExecutor returns a LocalExecutor.
func NewLocalExecutor() *LocalExecutor {
	return &LocalExecutor{}
}

// Execute retrieves auth from the keychain and runs the connector binary.
func (e *LocalExecutor) Execute(_ context.Context, req protocol.Request) (*protocol.Response, error) {
	token, _ := auth.Retrieve(req.Connector)
	req.Auth = token
	return connector.Execute(req)
}
