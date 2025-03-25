package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GetGitRemoteURL returns the URL of the git remote named 'origin'
// from the current working directory
func GetGitRemoteURL() (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get git remote URL: %w, stderr: %s", err, stderr.String())
	}

	url := strings.TrimSpace(stdout.String())
	if url == "" {
		return "", fmt.Errorf("no remote URL found for origin")
	}

	// Normalize SSH URLs to HTTPS format for API compatibility
	// Example: git@github.com:owner/repo.git -> https://github.com/owner/repo
	if strings.HasPrefix(url, "git@") {
		// Convert git@github.com:owner/repo.git to https://github.com/owner/repo
		url = strings.TrimSuffix(url, ".git")
		parts := strings.Split(url, ":")
		if len(parts) == 2 {
			domain := strings.TrimPrefix(parts[0], "git@")
			url = "https://" + domain + "/" + parts[1]
		}
	}

	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")

	return url, nil
}
