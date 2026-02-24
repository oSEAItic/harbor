package connector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/oseaitic/harbor/internal/protocol"
)

// ContractCase is one executable connector contract test.
type ContractCase struct {
	Name     string            `json:"name,omitempty"`
	Resource string            `json:"resource"`
	Params   map[string]string `json:"params,omitempty"`
	Raw      bool              `json:"raw,omitempty"`
}

// ContractSuite describes expected runnable examples for a connector bundle.
type ContractSuite struct {
	Cases []ContractCase `json:"cases"`
}

// LoadContractSuite parses a JSON contract suite from disk.
func LoadContractSuite(path string) (*ContractSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading suite %q: %w", path, err)
	}

	var suite ContractSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("parsing suite %q: %w", path, err)
	}
	if len(suite.Cases) == 0 {
		return nil, fmt.Errorf("suite %q has no cases", path)
	}
	return &suite, nil
}

// RunContractSuite executes all contract cases and validates protocol output.
func RunContractSuite(binPath string, suite *ContractSuite) error {
	for i, tc := range suite.Cases {
		if err := RunContractCase(binPath, tc); err != nil {
			caseName := tc.Name
			if caseName == "" {
				caseName = fmt.Sprintf("#%d", i+1)
			}
			return fmt.Errorf("contract case %s failed: %w", caseName, err)
		}
	}
	return nil
}

// RunContractCase executes one case and validates its response envelope.
func RunContractCase(binPath string, tc ContractCase) error {
	if tc.Resource == "" {
		return fmt.Errorf("resource is required")
	}

	params, err := json.Marshal(tc.Params)
	if err != nil {
		return fmt.Errorf("marshaling params: %w", err)
	}

	args := []string{
		binPath,
		"--resource", tc.Resource,
		"--params", string(params),
	}
	if tc.Raw {
		args = append(args, "--raw")
	}

	cmd := exec.Command("node", args...)
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running connector: %w (stderr: %s)", err, stderr.String())
	}

	var resp protocol.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}

	if err := protocol.ValidateResponse(&resp); err != nil {
		return fmt.Errorf("protocol validation failed: %w", err)
	}

	return nil
}
