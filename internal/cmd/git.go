package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const gitSuffix = ".git"

// NormalizeRepoURL normalizes a repo URL into a browsable URL.
//
//nolint:gocyclo // too many ifs, can be handled later
func NormalizeRepoURL(url string) (string, error) {
	url = strings.ToLower(url)
	// 'git+https://github.com/snyk/runtime-entities-service.git'
	if strings.HasPrefix(url, "git+https://") && strings.HasSuffix(url, gitSuffix) {
		return url[4:], nil
	}
	// 'github.com/snyk/runtime-entities-service'
	if strings.HasPrefix(url, "github.com/") && !strings.HasSuffix(url, gitSuffix) {
		return "https://" + url + gitSuffix, nil
	}
	// https://github.com/snyk/runtime-entities-service
	if strings.HasPrefix(url, "https://github.com/") && !strings.HasSuffix(url, gitSuffix) {
		return url + gitSuffix, nil
	}
	// 'git@github.com:snyk/runtime-entities-service.git'
	if strings.HasPrefix(url, "git@github.com:") && strings.HasSuffix(url, gitSuffix) {
		repoParts := strings.Split(url[15:], "/")
		if len(repoParts) > 1 {
			return "https://github.com/" + repoParts[0] + "/" + strings.Join(repoParts[1:], "/"), nil
		}
	}
	// 'git@bitbucket.org/user/repo.git'
	if strings.HasPrefix(url, "git@bitbucket.org/") && strings.HasSuffix(url, gitSuffix) {
		return "https://" + url[4:], nil
	}
	// 'ssh://git@bitbucket.org/user/repo.git'
	if strings.HasPrefix(url, "ssh://git@bitbucket.org/") && strings.HasSuffix(url, gitSuffix) {
		return "https://" + url[10:], nil
	}
	// 'https://git@bitbucket.org/user/repo.git'
	if strings.HasPrefix(url, "https://") && strings.Contains(url, "@bitbucket") && strings.HasSuffix(url, gitSuffix) {
		return "https://" + url[strings.Index(url, "@")+1:], nil
	}
	// 'git+ssh://git@github.com/snyk/test-service.git'
	if strings.HasPrefix(url, "git+ssh://git@") && strings.HasSuffix(url, gitSuffix) {
		return "https://" + url[strings.Index(url, "@")+1:], nil
	}
	// 'https://dev.azure.com/user/project/_git/repository
	if strings.HasPrefix(url, "https://dev.azure.com/") && strings.Contains(url, "_git") && !strings.HasSuffix(url, gitSuffix) {
		return url + gitSuffix, nil
	}
	// 'github.private-domain.com/user/repo.git
	if strings.HasPrefix(url, "github.") && strings.HasSuffix(strings.Split(url, "/")[0], ".com") && strings.HasSuffix(url, gitSuffix) {
		return "https://" + url, nil
	}
	// 'https://github.com/snyk/runtime-entities-service.git'
	if strings.HasPrefix(url, "https://") && strings.HasSuffix(url, gitSuffix) {
		return url, nil
	}

	return url, errors.New("URL format was not handled in NormalizeRepoURL")
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

	return NormalizeRepoURL(url)
}
