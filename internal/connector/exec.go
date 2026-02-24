package connector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oseaitic/harbor/internal/harborhome"
	"github.com/oseaitic/harbor/internal/protocol"
)

// Execute runs a connector as an exec plugin and returns the parsed response.
// The connector binary receives resource and params as CLI args.
// Auth is injected via the HARBOR_AUTH environment variable (never CLI args).
func Execute(req protocol.Request) (*protocol.Response, error) {
	binPath, err := SafeConnectorPath(req.Connector)
	if err != nil {
		return nil, err
	}

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
	runtimeDir, err := connectorRuntimeDir(req.Connector)
	if err != nil {
		return nil, fmt.Errorf("preparing connector runtime dir: %w", err)
	}
	cmd.Dir = runtimeDir

	// Run connectors with a minimal environment to reduce accidental secret leakage.
	env, err := connectorEnv(req.Auth)
	if err != nil {
		return nil, fmt.Errorf("preparing connector environment: %w", err)
	}
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(nil)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run with timeout
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case runErr := <-done:
		trimmedStdout := bytes.TrimSpace(stdout.Bytes())
		trimmedStderr := strings.TrimSpace(stderr.String())

		var (
			resp      protocol.Response
			parseErr  error
			validated bool
		)
		if len(trimmedStdout) > 0 {
			parseErr = json.Unmarshal(trimmedStdout, &resp)
			if parseErr == nil {
				if err := protocol.ValidateResponse(&resp); err == nil {
					validated = true
				} else {
					parseErr = err
				}
			}
		}

		if runErr != nil {
			// If connector returned a valid protocol response, surface that
			// response instead of a transport error so agents can reason on
			// structured error fields.
			if validated {
				if len(resp.Errors) == 0 {
					resp.Errors = []protocol.ErrorDetail{
						{
							Code:    protocol.ErrExecution,
							Message: "connector process exited with non-zero status",
							Detail:  firstNonEmpty(trimmedStderr, runErr.Error()),
						},
					}
				} else if trimmedStderr != "" {
					resp.Errors = append(resp.Errors, protocol.ErrorDetail{
						Code:    protocol.ErrExecution,
						Message: "connector stderr",
						Detail:  trimmedStderr,
					})
				}
				return &resp, nil
			}

			return nil, fmt.Errorf(
				"connector %q exited with error: %w\nstderr: %s\nstdout: %s",
				req.Connector,
				runErr,
				trimmedStderr,
				clip(string(trimmedStdout), 1200),
			)
		}

		// Successful process execution must return valid Harbor protocol JSON.
		if parseErr != nil {
			return nil, fmt.Errorf(
				"connector %q returned invalid protocol response: %v\nstdout: %s",
				req.Connector,
				parseErr,
				clip(string(trimmedStdout), 1200),
			)
		}
		if !validated {
			return nil, fmt.Errorf(
				"connector %q returned invalid protocol response\nstdout: %s",
				req.Connector,
				clip(string(trimmedStdout), 1200),
			)
		}
		return &resp, nil
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("connector %q timed out after 30s", req.Connector)
	}
}

func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func connectorRuntimeDir(name string) (string, error) {
	dir := harborhome.Path("runtime", "workdir", name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func connectorEnv(auth string) ([]string, error) {
	runtimeHome := harborhome.Path("runtime", "home")
	runtimeTmp := harborhome.Path("runtime", "tmp")
	if err := os.MkdirAll(runtimeHome, 0o700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(runtimeTmp, 0o700); err != nil {
		return nil, err
	}

	env := []string{
		"HARBOR_AUTH=" + auth,
		"HOME=" + runtimeHome,
		"TMPDIR=" + runtimeTmp,
	}

	if path := os.Getenv("PATH"); strings.TrimSpace(path) != "" {
		env = append(env, "PATH="+path)
	}

	// Allow network + TLS behavior required by many connectors while blocking
	// unrelated environment leakage.
	passThrough := []string{
		"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY",
		"http_proxy", "https_proxy", "no_proxy",
		"SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS",
		"LANG", "LC_ALL", "TZ",
	}
	for _, key := range passThrough {
		if val := os.Getenv(key); val != "" {
			env = append(env, key+"="+val)
		}
	}

	// Preserve Harbor-scoped controls (except auth, which we set explicitly).
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "HARBOR_") || strings.HasPrefix(kv, "HARBOR_AUTH=") {
			continue
		}
		env = append(env, kv)
	}

	sort.Strings(env)
	return env, nil
}
