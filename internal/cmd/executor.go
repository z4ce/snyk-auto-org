package cmd

import (
	"fmt"
	"os"
	"os/exec"
)

// SnykExecutor executes Snyk CLI commands
type SnykExecutor struct {
	// The organization ID to use for Snyk commands
	OrgID string
}

// NewSnykExecutor creates a new Snyk executor
func NewSnykExecutor(orgID string) *SnykExecutor {
	return &SnykExecutor{
		OrgID: orgID,
	}
}

// Execute runs a Snyk command with the configured organization
func (e *SnykExecutor) Execute(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no arguments provided")
	}

	// Create the command to execute
	cmd := exec.Command("snyk", args...)

	// Copy the current environment
	env := os.Environ()

	// Add the SNYK_CFG_ORG environment variable only if OrgID is not empty
	if e.OrgID != "" {
		env = append(env, fmt.Sprintf("SNYK_CFG_ORG=%s", e.OrgID))
	}
	cmd.Env = env

	// Connect standard I/O
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Execute the command
	return cmd.Run()
}
