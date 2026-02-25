package cli

import (
	"reflect"
	"testing"
)

func TestExtractCredentialFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantCreds   map[string]string
		wantRemain  []string
	}{
		{
			name:       "no credentials",
			args:       []string{"npx", "@modelcontextprotocol/server-github"},
			wantCreds:  map[string]string{},
			wantRemain: []string{"npx", "@modelcontextprotocol/server-github"},
		},
		{
			name:       "single credential",
			args:       []string{"--credential", "GITHUB_TOKEN=github-pat", "npx", "@modelcontextprotocol/server-github"},
			wantCreds:  map[string]string{"GITHUB_TOKEN": "github-pat"},
			wantRemain: []string{"npx", "@modelcontextprotocol/server-github"},
		},
		{
			name: "multiple credentials",
			args: []string{
				"--credential", "GITHUB_TOKEN=github-pat",
				"--credential", "SLACK_TOKEN=slack-bot",
				"npx", "my-server",
			},
			wantCreds: map[string]string{
				"GITHUB_TOKEN": "github-pat",
				"SLACK_TOKEN":  "slack-bot",
			},
			wantRemain: []string{"npx", "my-server"},
		},
		{
			name:       "credential in the middle",
			args:       []string{"npx", "--credential", "API_KEY=my-key", "my-server"},
			wantCreds:  map[string]string{"API_KEY": "my-key"},
			wantRemain: []string{"npx", "my-server"},
		},
		{
			name:       "credential without value is skipped",
			args:       []string{"--credential", "NOEQUALS", "npx", "my-server"},
			wantCreds:  map[string]string{},
			wantRemain: []string{"npx", "my-server"},
		},
		{
			name:       "credential at end with no value is kept as-is",
			args:       []string{"npx", "--credential"},
			wantCreds:  map[string]string{},
			wantRemain: []string{"npx", "--credential"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCreds, gotRemain := extractCredentialFlags(tt.args)
			if !reflect.DeepEqual(gotCreds, tt.wantCreds) {
				t.Errorf("credentials = %v, want %v", gotCreds, tt.wantCreds)
			}
			if !reflect.DeepEqual(gotRemain, tt.wantRemain) {
				t.Errorf("remaining = %v, want %v", gotRemain, tt.wantRemain)
			}
		})
	}
}
