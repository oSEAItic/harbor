package executor

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/oseaitic/harbor/internal/auth"
	"github.com/oseaitic/harbor/internal/cloudauth"
	"github.com/oseaitic/harbor/internal/connector"
	"github.com/oseaitic/harbor/internal/protocol"
	"github.com/oseaitic/harbor/internal/registry"
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
// If the connector is not installed and the user is logged in to Harbor Cloud,
// it automatically installs the connector from the cloud registry before retrying.
func (e *LocalExecutor) Execute(_ context.Context, req protocol.Request) (*protocol.Response, error) {
	token, _ := auth.Retrieve(req.Connector)
	req.Auth = token

	resp, err := connector.Execute(req)
	if err != nil && isNotInstalledError(err) {
		if cfg, cloudErr := cloudauth.Load(); cloudErr == nil {
			fmt.Fprintf(os.Stderr, "  connector %q not installed, pulling from cloud...\n", req.Connector)
			if pullErr := registry.InstallFromCloud(req.Connector, cfg); pullErr == nil {
				fmt.Fprintf(os.Stderr, "  installed %q from cloud.\n", req.Connector)
				// Re-retrieve auth in case it is now available after install
				token, _ = auth.Retrieve(req.Connector)
				req.Auth = token
				resp, err = connector.Execute(req)
			}
		}
	}

	return resp, err
}

// isNotInstalledError reports whether err indicates the connector binary is missing.
func isNotInstalledError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not installed")
}
