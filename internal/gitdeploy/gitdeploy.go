package gitdeploy

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Pull runs git pull in the repository directory.
func Pull(ctx context.Context, repoPath, branch string) (string, error) {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return "", fmt.Errorf("repo path required")
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	out, err := exec.CommandContext(ctx, "git", "-C", repoPath, "pull", "origin", branch).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git pull: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// InitRepo creates a bare-ish working tree path for deploy hooks.
func InitRepo(ctx context.Context, repoPath string) error {
	if err := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--git-dir").Run(); err == nil {
		return nil
	}
	return exec.CommandContext(ctx, "git", "-C", repoPath, "init").Run()
}
