package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// NormalizeRepoURL converts various Git remote URL formats to a standard HTTPS URL
// Output format: https://github.com/owner/repo
func NormalizeRepoURL(url string) (string, error) {
	if url == "" {
		return "", errors.New("empty URL provided")
	}

	// Handle SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(url, "git@") {
		url = strings.TrimSuffix(url, ".git")
		parts := strings.Split(url, ":")
		if len(parts) == 2 {
			domain := strings.TrimPrefix(parts[0], "git@")
			url = "https://" + domain + "/" + parts[1]
		}
	}

	// Handle git:// protocol: git://github.com/owner/repo.git
	if strings.HasPrefix(url, "git://") {
		url = strings.TrimSuffix(url, ".git")
		url = "https://" + strings.TrimPrefix(url, "git://")
	}

	// Handle http:// - convert to https://
	if strings.HasPrefix(url, "http://") {
		url = "https://" + strings.TrimPrefix(url, "http://")
	}

	// Handle https:// - already in the right format, just need to trim .git
	if strings.HasPrefix(url, "https://") {
		url = strings.TrimSuffix(url, ".git")
	}

	// Handle URLs with trailing slashes
	url = strings.TrimSuffix(url, "/")

	// Verify the URL has a valid format
	parts := strings.Split(url, "/")
	if !strings.HasPrefix(url, "https://") || len(parts) < 3 {
		return "", fmt.Errorf("invalid repository URL format: %s", url)
	}

	return url, nil
}

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

	return NormalizeRepoURL(url)
}
