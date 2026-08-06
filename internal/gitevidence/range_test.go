package gitevidence

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveValidatesRepositoryRange(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Harbor Test")
	runGit(t, repo, "config", "user.email", "harbor@example.com")
	writeAndCommit(t, repo, "one.txt", "one", "one")
	base := gitValue(t, repo, "rev-parse", "HEAD")
	writeAndCommit(t, repo, "two.txt", "two", "two")
	head := gitValue(t, repo, "rev-parse", "HEAD")

	resolved, err := Resolve(repo, base[:12], head[:12])
	if err != nil {
		t.Fatal(err)
	}
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.BaseSHA != base || resolved.HeadSHA != head || resolved.RepoPath != filepath.Clean(canonicalRepo) {
		t.Fatalf("unexpected resolved range: %+v", resolved)
	}
	if _, err := Resolve(repo, head, base); err == nil {
		t.Fatal("reverse range should be rejected")
	}
	if _, err := Resolve(repo, base, base); err == nil {
		t.Fatal("empty range should be rejected")
	}
}

func writeAndCommit(t *testing.T, repo, name, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", name)
	runGit(t, repo, "commit", "-m", message)
}

func gitValue(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out[:len(out)-1])
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
