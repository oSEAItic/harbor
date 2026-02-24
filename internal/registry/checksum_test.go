package registry

import (
	"strings"
	"testing"
)

func TestVerifyAndPinChecksumPinsFirstInstall(t *testing.T) {
	t.Setenv("HARBOR_HOME", t.TempDir())

	data := []byte("first-bundle")
	got, err := verifyAndPinChecksum("demo", data, "")
	if err != nil {
		t.Fatalf("verifyAndPinChecksum returned error: %v", err)
	}
	if len(got) != 64 {
		t.Fatalf("expected 64-char checksum, got %q", got)
	}

	pinned, err := readChecksumPin(checksumPinPath("demo"))
	if err != nil {
		t.Fatalf("readChecksumPin returned error: %v", err)
	}
	if got != pinned {
		t.Fatalf("expected pinned checksum %q, got %q", got, pinned)
	}
}

func TestVerifyAndPinChecksumRejectsPinnedMismatch(t *testing.T) {
	t.Setenv("HARBOR_HOME", t.TempDir())

	if _, err := verifyAndPinChecksum("demo", []byte("first-bundle"), ""); err != nil {
		t.Fatalf("first pin failed: %v", err)
	}

	_, err := verifyAndPinChecksum("demo", []byte("second-bundle"), "")
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "pinned") {
		t.Fatalf("expected pinned mismatch error, got: %v", err)
	}
}

func TestVerifyAndPinChecksumAllowsExplicitRepin(t *testing.T) {
	t.Setenv("HARBOR_HOME", t.TempDir())

	if _, err := verifyAndPinChecksum("demo", []byte("first-bundle"), ""); err != nil {
		t.Fatalf("first pin failed: %v", err)
	}

	second := []byte("second-bundle")
	expected := sha256Hex(second)
	got, err := verifyAndPinChecksum("demo", second, expected)
	if err != nil {
		t.Fatalf("explicit checksum install failed: %v", err)
	}
	if got != expected {
		t.Fatalf("expected checksum %q, got %q", expected, got)
	}

	pinned, err := readChecksumPin(checksumPinPath("demo"))
	if err != nil {
		t.Fatalf("readChecksumPin returned error: %v", err)
	}
	if pinned != expected {
		t.Fatalf("expected repinned checksum %q, got %q", expected, pinned)
	}
}

func TestVerifyAndPinChecksumRejectsInvalidExpected(t *testing.T) {
	t.Setenv("HARBOR_HOME", t.TempDir())

	_, err := verifyAndPinChecksum("demo", []byte("bundle"), "not-a-sha")
	if err == nil {
		t.Fatal("expected invalid checksum error")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected invalid checksum error, got: %v", err)
	}
}
