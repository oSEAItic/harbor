package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallFromLocalRejectsPathTraversalName(t *testing.T) {
	t.Setenv("HARBOR_HOME", t.TempDir())

	bundle := filepath.Join(t.TempDir(), "bundle.js")
	if err := os.WriteFile(bundle, []byte("#!/usr/bin/env node\n"), 0o755); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	err := InstallFromLocal("../escaped", bundle, InstallOptions{})
	if err == nil {
		t.Fatal("expected InstallFromLocal to reject invalid connector name")
	}
	if !strings.Contains(err.Error(), "invalid connector name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUninstallRejectsPathTraversalName(t *testing.T) {
	t.Setenv("HARBOR_HOME", t.TempDir())

	err := Uninstall("../escaped")
	if err == nil {
		t.Fatal("expected Uninstall to reject invalid connector name")
	}
	if !strings.Contains(err.Error(), "invalid connector name") {
		t.Fatalf("unexpected error: %v", err)
	}
}
