package connector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/oseaitic/harbor/internal/protocol"
)

// Execute runs a connector as an exec plugin and returns the parsed response.
// The connector binary receives resource and params as CLI args.
// Auth is injected via the HARBOR_AUTH environment variable (never CLI args).
func Execute(req protocol.Request) (*protocol.Response, error) {
	binPath := ConnectorPath(req.Connector)

	// Check connector exists
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("connector %q not installed (expected at %s)", req.Connector, binPath)
	}

	// Build params JSON
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return nil, fmt.Errorf("marshaling params: %w", err)
	}

	// Build command
	args := []string{
		"--resource", req.Resource,
		"--params", string(paramsJSON),
	}
	if req.Raw {
		args = append(args, "--raw")
	}

	cmd := exec.Command(binPath, args...)

	// Inject auth via env var — never pass secrets as CLI args
	cmd.Env = append(os.Environ(), fmt.Sprintf("HARBOR_AUTH=%s", req.Auth))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run with timeout
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("connector %q exited with error: %w\nstderr: %s", req.Connector, err, stderr.String())
		}
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("connector %q timed out after 30s", req.Connector)
	}

	// Parse response
	var resp protocol.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("connector %q returned invalid JSON: %w\nraw output: %s", req.Connector, err, stdout.String())
	}

	return &resp, nil
}
