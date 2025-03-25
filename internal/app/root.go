package app

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/z4ce/snyk-auto-org/internal/api"
	"github.com/z4ce/snyk-auto-org/internal/cache"
	cmdpkg "github.com/z4ce/snyk-auto-org/internal/cmd"
	"github.com/z4ce/snyk-auto-org/internal/config"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "snyk-auto-org [snyk command]",
	Short: "Run Snyk CLI commands with automatic organization selection",
	Long: `Snyk Auto Org is a wrapper around the Snyk CLI that automatically sets 
the SNYK_CFG_ORG environment variable based on available organizations
from your Snyk account.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(cmd, args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Add flags
	rootCmd.Flags().Bool("reset-cache", false, "Reset the organization cache")
	rootCmd.Flags().String("cache-ttl", "24h", "Set the time-to-live for cached data")
	rootCmd.Flags().String("org", "", "Explicitly specify which organization to use by name or ID")
	rootCmd.Flags().Bool("list-orgs", false, "Display available organizations and exit")
	rootCmd.Flags().Bool("verbose", false, "Show additional information during execution")
	rootCmd.Flags().String("git-url", "", "Specify a Git URL to automatically find the right organization")
	rootCmd.Flags().Bool("auto-detect-git", true, "Automatically detect Git remote URL for organization selection")
}

func run(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override config with command line flags
	if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
		cfg.Verbose = true
	}

	// Check for cache-ttl flag
	if cacheTTLStr, _ := cmd.Flags().GetString("cache-ttl"); cacheTTLStr != "" {
		cacheTTL, err := time.ParseDuration(cacheTTLStr)
		if err != nil {
			return fmt.Errorf("invalid cache TTL: %w", err)
		}
		cfg.CacheTTL = cacheTTL
	}

	// Create the cache
	db, err := cache.NewSQLiteCache()
	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}
	defer db.Close()

	// Check if the user requested a cache reset
	if resetCache, _ := cmd.Flags().GetBool("reset-cache"); resetCache {
		if err := db.ResetCache(); err != nil {
			return fmt.Errorf("failed to reset cache: %w", err)
		}
		if cfg.Verbose {
			fmt.Println("Cache has been reset")
		}
	}

	// If the user explicitly specified an organization, use that
	if orgOption, _ := cmd.Flags().GetString("org"); orgOption != "" {
		// Check if the org exists and get its ID
		organizations, err := getOrganizations(db, cfg)
		if err != nil {
			return fmt.Errorf("failed to get organizations: %w", err)
		}

		var orgID string
		found := false
		for _, org := range organizations {
			if org.ID == orgOption || org.Name == orgOption || org.Slug == orgOption {
				orgID = org.ID
				found = true
				if cfg.Verbose {
					fmt.Printf("Using specified Snyk organization: %s (%s)\n", org.Name, org.ID)
				}
				break
			}
		}

		if !found {
			return fmt.Errorf("organization not found: %s", orgOption)
		}

		// Use the specified organization
		executor := cmdpkg.NewSnykExecutor(orgID)
		return executor.Execute(args)
	}

	// Check if the user requested to list organizations
	if listOrgs, _ := cmd.Flags().GetBool("list-orgs"); listOrgs {
		organizations, err := getOrganizations(db, cfg)
		if err != nil {
			return fmt.Errorf("failed to get organizations: %w", err)
		}

		fmt.Println("Available Snyk organizations:")
		for _, org := range organizations {
			fmt.Printf("- %s (%s)\n", org.Name, org.ID)
		}
		return nil
	}

	// Create Snyk client
	client, err := api.NewSnykClient()
	if err != nil {
		return fmt.Errorf("failed to create Snyk client: %w", err)
	}

	// Check if Git URL detection is enabled/provided
	gitURL, _ := cmd.Flags().GetString("git-url")
	autoDetectGit, _ := cmd.Flags().GetBool("auto-detect-git")

	// Try to get organization by git URL
	if gitURL != "" || autoDetectGit {
		// If explicit URL provided, use it, otherwise try to detect
		if gitURL == "" && autoDetectGit {
			// Try to detect git remote URL
			detectedURL, err := cmdpkg.GetGitRemoteURL()
			if err != nil {
				if cfg.Verbose {
					fmt.Printf("Could not detect Git remote URL: %v\n", err)
				}
				// Continue without setting org since we couldn't detect Git URL
				if cfg.Verbose {
					fmt.Println("Running Snyk command without organization")
				}
				executor := cmdpkg.NewSnykExecutor("")
				return executor.Execute(args)
			} else {
				gitURL = detectedURL
				if cfg.Verbose {
					fmt.Printf("Detected Git remote URL: %s\n", gitURL)
				}
			}
		}

		// If we have a Git URL (whether provided or detected), use it to find organization
		if gitURL != "" {
			if cfg.Verbose {
				fmt.Printf("Searching for organization with target URL: %s\n", gitURL)
			}

			// Find organization by target URL
			orgTarget, err := client.FindOrgWithTargetURL(gitURL)
			if err != nil {
				if cfg.Verbose {
					fmt.Printf("Could not find organization for Git URL: %v\n", err)
					fmt.Println("Running Snyk command without organization")
				}

				// Just execute the command without setting an organization
				executor := cmdpkg.NewSnykExecutor("")
				return executor.Execute(args)
			} else {
				if cfg.Verbose {
					fmt.Printf("Found organization %s for target %s\n", orgTarget.OrgName, orgTarget.TargetName)
				}

				// Create the executor and run the command with the found organization
				executor := cmdpkg.NewSnykExecutor(orgTarget.OrgID)
				return executor.Execute(args)
			}
		}
	}

	// If we reach here, we have no organization to use
	// (no git URL was found or specified, and no --org flag was used)

	// Check if there's a default org in the config
	if cfg.DefaultOrg != "" {
		organizations, err := getOrganizations(db, cfg)
		if err != nil {
			return fmt.Errorf("failed to get organizations: %w", err)
		}

		// Try to find the default org
		for _, org := range organizations {
			if org.ID == cfg.DefaultOrg || org.Name == cfg.DefaultOrg || org.Slug == cfg.DefaultOrg {
				if cfg.Verbose {
					fmt.Printf("Using default organization from config: %s (%s)\n", org.Name, org.ID)
				}
				executor := cmdpkg.NewSnykExecutor(org.ID)
				return executor.Execute(args)
			}
		}
	}

	// If no arguments were provided, just show help
	if len(args) == 0 {
		return cmd.Help()
	}

	// Run the command without setting an organization
	if cfg.Verbose {
		fmt.Println("Running Snyk command without organization")
	}
	executor := cmdpkg.NewSnykExecutor("")
	return executor.Execute(args)
}

// getOrganizations retrieves organizations from the cache or the Snyk API
func getOrganizations(db *cache.SQLiteCache, cfg *config.Config) ([]api.Organization, error) {
	// Check if the cache is expired
	expired, err := db.IsExpired(cfg.CacheTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to check cache expiration: %w", err)
	}

	// If the cache is valid, use it
	if !expired {
		orgs, err := db.GetOrganizations()
		if err != nil {
			return nil, fmt.Errorf("failed to get organizations from cache: %w", err)
		}
		if len(orgs) > 0 {
			return orgs, nil
		}
	}

	// Cache is expired or empty, fetch organizations from the API
	client, err := api.NewSnykClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Snyk client: %w", err)
	}

	orgs, err := client.GetOrganizations()
	if err != nil {
		return nil, fmt.Errorf("failed to get organizations from API: %w", err)
	}

	// Store the organizations in the cache
	if err := db.StoreOrganizations(orgs); err != nil {
		return nil, fmt.Errorf("failed to store organizations in cache: %w", err)
	}

	return orgs, nil
}
