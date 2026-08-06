package gitevidence

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var commitPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)

type Range struct {
	RepoPath string
	BaseSHA  string
	HeadSHA  string
}

func Resolve(repoPath, baseSHA, headSHA string) (Range, error) {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return Range{}, errors.New("repository path is required")
	}
	if !commitPattern.MatchString(strings.TrimSpace(baseSHA)) {
		return Range{}, errors.New("base commit must be a 7-64 character hexadecimal Git SHA")
	}
	if !commitPattern.MatchString(strings.TrimSpace(headSHA)) {
		return Range{}, errors.New("head commit must be a 7-64 character hexadecimal Git SHA")
	}

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return Range{}, fmt.Errorf("resolving repository path: %w", err)
	}
	root, err := gitOutput(absPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return Range{}, fmt.Errorf("resolving Git repository: %w", err)
	}
	base, err := gitOutput(root, "rev-parse", "--verify", strings.TrimSpace(baseSHA)+"^{commit}")
	if err != nil {
		return Range{}, fmt.Errorf("resolving base commit: %w", err)
	}
	head, err := gitOutput(root, "rev-parse", "--verify", strings.TrimSpace(headSHA)+"^{commit}")
	if err != nil {
		return Range{}, fmt.Errorf("resolving head commit: %w", err)
	}
	if base == head {
		return Range{}, errors.New("base and head commits must be different")
	}
	if err := gitRun(root, "merge-base", "--is-ancestor", base, head); err != nil {
		return Range{}, errors.New("base commit must be an ancestor of head commit")
	}
	return Range{RepoPath: filepath.Clean(root), BaseSHA: strings.ToLower(base), HeadSHA: strings.ToLower(head)}, nil
}

func gitOutput(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return "", errors.New(message)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitRun(repoPath string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	return cmd.Run()
}
